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

package showback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	echo "github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider() *ShowbackProvider {
	return &ShowbackProvider{
		store: NewUsageStore(),
	}
}

func TestHandleGetServiceUsage_Found(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()
	resourceID := uuid.New()
	serviceID := uuid.New()

	p.store.StartMetering(tenantID, resourceID, "gpu-hours")
	// Set ServiceID on the record.
	for _, rec := range p.store.records {
		rec.ServiceID = serviceID
	}
	err := p.store.StopMetering(resourceID)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/services/"+serviceID.String()+"/usage", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(serviceID.String())

	err = p.handleGetServiceUsage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)
	assert.Equal(t, tenantID, summary.TenantID)
	assert.Contains(t, summary.Metrics, "gpu-hours")
}

func TestHandleGetServiceUsage_NotFound(t *testing.T) {
	p := newTestProvider()
	serviceID := uuid.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/services/"+serviceID.String()+"/usage", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(serviceID.String())

	err := p.handleGetServiceUsage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)
	assert.Empty(t, summary.Metrics)
}

func TestHandleGetServiceUsage_InvalidID(t *testing.T) {
	p := newTestProvider()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/services/bad-id/usage", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("bad-id")

	err := p.handleGetServiceUsage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetSelfUsage_ReturnsMockData(t *testing.T) {
	p := newTestProvider()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/usage", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := p.handleGetSelfUsage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)

	assert.Equal(t, "current-month", summary.Period)
	assert.Equal(t, 120.5, summary.Metrics["gpu-hours"])
	assert.Equal(t, 2048.0, summary.Metrics["storage-gb-hours"])
}

func TestHandleGetSelfQuotas_ReturnsMockData(t *testing.T) {
	p := newTestProvider()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/quotas", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := p.handleGetSelfQuotas(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var info QuotaInfo
	err = json.Unmarshal(rec.Body.Bytes(), &info)
	require.NoError(t, err)

	require.Contains(t, info.Quotas, "gpu-hours")
	assert.Equal(t, 1000.0, info.Quotas["gpu-hours"].Limit)
	assert.Equal(t, 120.5, info.Quotas["gpu-hours"].Current)
	assert.Equal(t, "hours", info.Quotas["gpu-hours"].Unit)

	require.Contains(t, info.Quotas, "storage-gb-hours")
	assert.Equal(t, 10000.0, info.Quotas["storage-gb-hours"].Limit)
	assert.Equal(t, 2048.0, info.Quotas["storage-gb-hours"].Current)
	assert.Equal(t, "gb-hours", info.Quotas["storage-gb-hours"].Unit)
}
