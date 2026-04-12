/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	echo "github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFaultHandler() *FaultHandler {
	faultStore := NewFaultEventStore(nil)
	classStore := NewClassificationStore(loadDefaultClassifications())
	return NewFaultHandler(faultStore, classStore)
}

func TestHandleIngestFault_Success(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	body := `{"source":"dcgm","severity":"critical","component":"gpu","message":"GPU ECC error"}`
	req := httptest.NewRequest(http.MethodPost, "/health/events/ingest", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleIngestFault(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var event FaultEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &event))
	assert.Equal(t, "dcgm", event.Source)
	assert.Equal(t, FaultStateOpen, event.State)
	assert.NotEmpty(t, event.ID)
}

func TestHandleIngestFault_MissingSource(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	body := `{"severity":"critical","component":"gpu","message":"error"}`
	req := httptest.NewRequest(http.MethodPost, "/health/events/ingest", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleIngestFault(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleIngestFault_MissingSeverity(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	body := `{"source":"dcgm","component":"gpu","message":"error"}`
	req := httptest.NewRequest(http.MethodPost, "/health/events/ingest", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleIngestFault(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleIngestFault_MissingComponent(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	body := `{"source":"dcgm","severity":"critical","message":"error"}`
	req := httptest.NewRequest(http.MethodPost, "/health/events/ingest", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleIngestFault(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleIngestFault_MissingMessage(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	body := `{"source":"dcgm","severity":"critical","component":"gpu"}`
	req := httptest.NewRequest(http.MethodPost, "/health/events/ingest", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleIngestFault(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleListFaultEvents_Empty(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/health/events", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleListFaultEvents(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleListFaultEvents_WithFilter(t *testing.T) {
	h := newTestFaultHandler()

	// Seed data
	require.NoError(t, h.faultStore.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "gpu fault", SiteID: "site-1",
	}))
	require.NoError(t, h.faultStore.Create(&FaultEvent{
		Source: "nhc", Severity: SeverityWarning, Component: ComponentNetwork,
		Message: "link down", SiteID: "site-1",
	}))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health/events?severity=critical", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleListFaultEvents(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var events []FaultEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &events))
	assert.Len(t, events, 1)
	assert.Equal(t, SeverityCritical, events[0].Severity)
}

func TestHandleGetFaultEvent_Success(t *testing.T) {
	h := newTestFaultHandler()

	event := &FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "fault", SiteID: "site-1",
	}
	require.NoError(t, h.faultStore.Create(event))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health/events/"+event.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(event.ID)

	err := h.handleGetFaultEvent(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleGetFaultEvent_InvalidID(t *testing.T) {
	h := newTestFaultHandler()
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/health/events/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")

	err := h.handleGetFaultEvent(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetFaultSummary(t *testing.T) {
	h := newTestFaultHandler()

	require.NoError(t, h.faultStore.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "gpu", SiteID: "site-1",
	}))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health/events/summary", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleGetFaultSummary(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary FaultSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
	assert.Equal(t, 1, summary.BySeverity[SeverityCritical])
}

func TestHandleUpdateFaultEvent_Acknowledge(t *testing.T) {
	h := newTestFaultHandler()

	event := &FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "fault", SiteID: "site-1",
	}
	require.NoError(t, h.faultStore.Create(event))

	e := echo.New()
	body := `{"state":"acknowledged"}`
	req := httptest.NewRequest(http.MethodPatch, "/health/events/"+event.ID, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(event.ID)

	err := h.handleUpdateFaultEvent(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var updated FaultEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, FaultStateAcknowledged, updated.State)
	assert.NotNil(t, updated.AcknowledgedAt)
}

func TestHandleTriggerRemediation(t *testing.T) {
	h := newTestFaultHandler()

	event := &FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "fault", SiteID: "site-1",
	}
	require.NoError(t, h.faultStore.Create(event))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/health/events/"+event.ID+"/remediate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(event.ID)

	err := h.handleTriggerRemediation(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, rec.Code)

	var updated FaultEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, FaultStateRemediating, updated.State)
	assert.Equal(t, 1, updated.RemediationAttempts)
}
