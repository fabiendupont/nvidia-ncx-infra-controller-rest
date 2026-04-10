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

package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler() (*BlueprintHandler, BlueprintStoreInterface) {
	store := NewBlueprintStore()
	return NewBlueprintHandler(store), store
}

func seedBlueprint(t *testing.T, store BlueprintStoreInterface, name string) *Blueprint {
	t.Helper()
	b := &Blueprint{
		Name:    name,
		Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc"},
		},
		Visibility: VisibilityPublic,
	}
	require.NoError(t, store.Create(b))
	return b
}

func TestHandleCreateBlueprint_WithPricing(t *testing.T) {
	h, _ := newTestHandler()
	e := echo.New()
	body := `{
		"name": "gpu-slice",
		"version": "1.0.0",
		"resources": {"vpc": {"type": "nico/vpc"}},
		"pricing": {"rate": 10.0, "unit": "hour", "currency": "USD"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/blueprints", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleCreateBlueprint(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var b Blueprint
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "gpu-slice", b.Name)
	require.NotNil(t, b.Pricing)
	assert.Equal(t, 10.0, b.Pricing.Rate)
	assert.Equal(t, "hour", b.Pricing.Unit)
	assert.Equal(t, VisibilityPublic, b.Visibility)
}

func TestHandleCreateBlueprint_InvalidPricingUnit(t *testing.T) {
	h, _ := newTestHandler()
	e := echo.New()
	body := `{
		"name": "bad-pricing",
		"version": "1.0.0",
		"resources": {"vpc": {"type": "nico/vpc"}},
		"pricing": {"rate": 5.0, "unit": "weekly"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/blueprints", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleCreateBlueprint(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateBlueprint_TenantDefaultsToOrganization(t *testing.T) {
	h, _ := newTestHandler()
	e := echo.New()
	tenantID := uuid.New()
	body := `{
		"name": "tenant-bp",
		"version": "1.0.0",
		"tenant_id": "` + tenantID.String() + `",
		"resources": {"base": {"type": "blueprint/some-parent"}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/blueprints", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleCreateBlueprint(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var b Blueprint
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, VisibilityOrganization, b.Visibility)
}

func TestHandleListBlueprints_TenantFilter(t *testing.T) {
	h, store := newTestHandler()

	// Create a public provider blueprint
	pub := seedBlueprint(t, store, "public-bp")

	// Create a tenant-owned blueprint
	tenantID := uuid.New()
	tenantBP := &Blueprint{
		Name:       "tenant-bp",
		Version:    "1.0.0",
		TenantID:   &tenantID,
		Visibility: VisibilityOrganization,
		Resources:  map[string]BlueprintResource{"base": {Type: "blueprint/" + pub.ID}},
	}
	require.NoError(t, store.Create(tenantBP))

	// Create another tenant's blueprint (should not appear)
	otherTenant := uuid.New()
	otherBP := &Blueprint{
		Name:       "other-bp",
		Version:    "1.0.0",
		TenantID:   &otherTenant,
		Visibility: VisibilityOrganization,
		Resources:  map[string]BlueprintResource{"base": {Type: "blueprint/" + pub.ID}},
	}
	require.NoError(t, store.Create(otherBP))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/blueprints?tenant_id="+tenantID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.handleListBlueprints(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var list []*Blueprint
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	// Should see the public blueprint + the tenant's own, but not the other tenant's
	assert.Len(t, list, 2)

	var names []string
	for _, bp := range list {
		names = append(names, bp.Name)
	}
	assert.Contains(t, names, "public-bp")
	assert.Contains(t, names, "tenant-bp")
	assert.NotContains(t, names, "other-bp")
}

func TestHandleEstimateCost_WithPricing(t *testing.T) {
	h, store := newTestHandler()

	b := &Blueprint{
		Name:    "priced-bp",
		Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc"},
		},
		Pricing: &PricingSpec{Rate: 10.0, Unit: "hour", Currency: "USD"},
	}
	require.NoError(t, store.Create(b))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/blueprints/"+b.ID+"/estimate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(b.ID)

	err := h.handleEstimateCost(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var est CostEstimate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &est))
	assert.Equal(t, 10.0, est.EstimatedRate)
	assert.Equal(t, "hour", est.Unit)
	assert.Len(t, est.Breakdown, 1)
}

func TestHandleEstimateCost_ComposedBlueprint(t *testing.T) {
	h, store := newTestHandler()

	// Create two atomic blueprints with pricing
	gpu := &Blueprint{
		Name: "gpu", Version: "1.0.0",
		Resources: map[string]BlueprintResource{"i": {Type: "nico/instance"}},
		Pricing:   &PricingSpec{Rate: 10.0, Unit: "hour", Currency: "USD"},
	}
	storage := &Blueprint{
		Name: "storage", Version: "1.0.0",
		Resources: map[string]BlueprintResource{"a": {Type: "nico/allocation"}},
		Pricing:   &PricingSpec{Rate: 3.0, Unit: "hour", Currency: "USD"},
	}
	require.NoError(t, store.Create(gpu))
	require.NoError(t, store.Create(storage))

	// Create a composed blueprint without explicit pricing
	composed := &Blueprint{
		Name: "workstation", Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"gpu":     {Type: "blueprint/" + gpu.ID},
			"storage": {Type: "blueprint/" + storage.ID, DependsOn: []string{"gpu"}},
		},
	}
	require.NoError(t, store.Create(composed))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/blueprints/"+composed.ID+"/estimate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(composed.ID)

	err := h.handleEstimateCost(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var est CostEstimate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &est))
	assert.Equal(t, 13.0, est.EstimatedRate)
	assert.Len(t, est.Breakdown, 2)
}

func TestHandleResolvedBlueprint_NonVariant(t *testing.T) {
	h, store := newTestHandler()
	b := seedBlueprint(t, store, "standalone")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/blueprints/"+b.ID+"/resolved", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(b.ID)

	err := h.handleResolvedBlueprint(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resolved Blueprint
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resolved))
	assert.Equal(t, b.ID, resolved.ID)
}

func TestHandleResolvedBlueprint_VariantHidesLocked(t *testing.T) {
	h, store := newTestHandler()

	parent := &Blueprint{
		Name:    "parent",
		Version: "1.0.0",
		Parameters: map[string]BlueprintParameter{
			"region": {Name: "region", Type: "string", Required: true},
			"nsg":    {Name: "nsg", Type: "string", Required: true},
		},
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc"},
		},
	}
	require.NoError(t, store.Create(parent))

	variant := &Blueprint{
		Name:    "variant",
		Version: "1.0.0",
		BasedOn: parent.ID,
		Parameters: map[string]BlueprintParameter{
			"nsg": {Name: "nsg", Default: "sg-corp", Locked: boolPtr(true)},
		},
	}
	require.NoError(t, store.Create(variant))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/blueprints/"+variant.ID+"/resolved", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(variant.ID)

	err := h.handleResolvedBlueprint(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resolved Blueprint
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resolved))
	// Locked param should be filtered out
	assert.NotContains(t, resolved.Parameters, "nsg")
	// Unlocked param should remain
	assert.Contains(t, resolved.Parameters, "region")
	// Resources from parent
	assert.Contains(t, resolved.Resources, "vpc")
}

func TestHandleEstimateCost_NotFound(t *testing.T) {
	h, _ := newTestHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/blueprints/nonexistent/estimate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := h.handleEstimateCost(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSeedBlueprints_LoadsData(t *testing.T) {
	store := NewBlueprintStore()
	p := &CatalogProvider{blueprintStore: store}
	p.LoadSeedData()

	all := store.GetAll()
	assert.Len(t, all, 4) // 3 atomic + 1 composed

	var names []string
	for _, b := range all {
		names = append(names, b.Name)
	}
	assert.Contains(t, names, "GPU 4xA100 Slice")
	assert.Contains(t, names, "Storage 200GB")
	assert.Contains(t, names, "PyTorch Stack")
	assert.Contains(t, names, "AI Standard Workstation")
}

func TestSeedBlueprints_SkipsIfNotEmpty(t *testing.T) {
	store := NewBlueprintStore()
	existing := &Blueprint{Name: "existing", Version: "1.0.0"}
	require.NoError(t, store.Create(existing))

	p := &CatalogProvider{blueprintStore: store}
	p.LoadSeedData()

	all := store.GetAll()
	assert.Len(t, all, 1)
	assert.Equal(t, "existing", all[0].Name)
}
