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
		rates: map[string]RateEntry{
			"gpu-hours":        {Rate: 10.00, Currency: "USD"},
			"storage-gb-hours": {Rate: 0.015, Currency: "USD"},
		},
	}
}

func TestHandleGetServiceUsage_Found(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()
	resourceID := uuid.New()
	serviceID := uuid.New()

	// Use the concrete in-memory store to access internal fields for test setup.
	memStore := p.store.(*UsageStore)
	memStore.StartMetering(tenantID, resourceID, "gpu-hours")
	// Set ServiceID on the record.
	for _, rec := range memStore.records {
		rec.ServiceID = serviceID
	}
	err := memStore.StopMetering(resourceID)
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

func TestHandleGetSelfUsage_EmptyStore(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/usage", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", tenantID.String())

	err := p.handleGetSelfUsage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)
	assert.Equal(t, "current-month", summary.Period)
	assert.Empty(t, summary.Metrics)
}

func TestHandleGetSelfUsage_WithData(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()
	resourceID := uuid.New()
	p.store.StartMetering(tenantID, resourceID, "gpu-hours")
	p.store.StopMetering(resourceID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/usage", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", tenantID.String())

	err := p.handleGetSelfUsage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)
	assert.Contains(t, summary.Metrics, "gpu-hours")
}

func TestHandleGetSelfQuotas_EmptyStore(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/quotas", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", tenantID.String())

	err := p.handleGetSelfQuotas(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var info QuotaInfo
	err = json.Unmarshal(rec.Body.Bytes(), &info)
	require.NoError(t, err)
	assert.Empty(t, info.Quotas)
}

func TestHandleGetSelfQuotas_WithData(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()
	resourceID := uuid.New()
	p.store.StartMetering(tenantID, resourceID, "gpu-hours")
	p.store.StopMetering(resourceID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/quotas", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", tenantID.String())

	err := p.handleGetSelfQuotas(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var info QuotaInfo
	err = json.Unmarshal(rec.Body.Bytes(), &info)
	require.NoError(t, err)
	require.Contains(t, info.Quotas, "gpu-hours")
	assert.Equal(t, "hours", info.Quotas["gpu-hours"].Unit)
}

func TestHandleGetSelfUsageCosts_WithData(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()
	resourceID := uuid.New()
	p.store.StartMetering(tenantID, resourceID, "gpu-hours")
	p.store.StopMetering(resourceID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/usage/costs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", tenantID.String())

	err := p.handleGetSelfUsageCosts(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageCostSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)
	assert.Equal(t, "current-month", summary.Period)
	assert.Equal(t, "USD", summary.Currency)
	require.Contains(t, summary.Costs, "gpu-hours")
	assert.Equal(t, 10.0, summary.Costs["gpu-hours"].Rate)
	assert.Greater(t, summary.Costs["gpu-hours"].Quantity, 0.0)
	assert.Greater(t, summary.TotalCost, 0.0)
}

func TestHandleGetSelfUsageCosts_Empty(t *testing.T) {
	p := newTestProvider()
	tenantID := uuid.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/self/usage/costs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", tenantID.String())

	err := p.handleGetSelfUsageCosts(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary UsageCostSummary
	err = json.Unmarshal(rec.Body.Bytes(), &summary)
	require.NoError(t, err)
	assert.Equal(t, 0.0, summary.TotalCost)
	assert.Empty(t, summary.Costs)
}
