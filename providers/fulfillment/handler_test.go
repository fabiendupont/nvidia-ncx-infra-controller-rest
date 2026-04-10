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

package fulfillment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- OrderHandler tests ---

func TestOrderHandler_Create(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	body := `{
		"template_id": "` + uuid.New().String() + `",
		"template_name": "gpu-cluster",
		"tenant_id": "` + uuid.New().String() + `",
		"parameters": {"nodes": 4}
	}`

	req := httptest.NewRequest(http.MethodPost, "/catalog/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var order Order
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &order))
	assert.Equal(t, OrderStatusPending, order.Status)
	assert.Equal(t, "gpu-cluster", order.TemplateName)
}

func TestOrderHandler_Get_Found(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	order := newTestOrder()
	require.NoError(t, store.Create(order))

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders/"+order.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(order.ID.String())

	err := handler.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var got Order
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, order.ID, got.ID)
}

func TestOrderHandler_Get_NotFound(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	err := handler.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrderHandler_Cancel_Found(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	order := newTestOrder()
	require.NoError(t, store.Create(order))

	req := httptest.NewRequest(http.MethodDelete, "/catalog/orders/"+order.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(order.ID.String())

	err := handler.Cancel(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestOrderHandler_Cancel_NotFound(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/catalog/orders/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	err := handler.Cancel(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrderHandler_List_All(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	require.NoError(t, store.Create(newTestOrder()))
	require.NoError(t, store.Create(newTestOrder()))

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp provider.ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
}

func TestOrderHandler_List_ByTenant(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	tenantA := uuid.New()
	tenantB := uuid.New()

	o1 := newTestOrder()
	o1.TenantID = tenantA
	o2 := newTestOrder()
	o2.TenantID = tenantB

	require.NoError(t, store.Create(o1))
	require.NoError(t, store.Create(o2))

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders?tenant_id="+tenantA.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp provider.ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
}

func TestOrderHandler_List_ByStatus(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	o1 := newTestOrder()
	o1.Status = OrderStatusPending
	o2 := newTestOrder()
	o2.Status = OrderStatusReady

	require.NoError(t, store.Create(o1))
	require.NoError(t, store.Create(o2))

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders?status=Ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp provider.ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
}

func TestOrderHandler_List_Empty(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp provider.ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Total)
}

func TestOrderHandler_List_Pagination(t *testing.T) {
	e := echo.New()
	store := NewOrderStore()
	handler := NewOrderHandler(store)

	for i := 0; i < 5; i++ {
		require.NoError(t, store.Create(newTestOrder()))
	}

	req := httptest.NewRequest(http.MethodGet, "/catalog/orders?offset=2&limit=2", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp provider.ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 5, resp.Total)
	assert.Equal(t, 2, resp.Offset)
	assert.Equal(t, 2, resp.Limit)
	items := resp.Items.([]interface{})
	assert.Len(t, items, 2)
}

// --- ServiceHandler tests ---

func TestServiceHandler_List(t *testing.T) {
	e := echo.New()
	store := NewServiceStore()
	handler := NewServiceHandler(store)

	tenantID := uuid.New()
	s1 := newTestService(tenantID)
	s2 := newTestService(uuid.New())
	require.NoError(t, store.Create(s1))
	require.NoError(t, store.Create(s2))

	// List all.
	req := httptest.NewRequest(http.MethodGet, "/services", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var services []*Service
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &services))
	assert.Len(t, services, 2)
}

func TestServiceHandler_Get_Found(t *testing.T) {
	e := echo.New()
	store := NewServiceStore()
	handler := NewServiceHandler(store)

	svc := newTestService(uuid.New())
	require.NoError(t, store.Create(svc))

	req := httptest.NewRequest(http.MethodGet, "/services/"+svc.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(svc.ID.String())

	err := handler.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var got Service
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, svc.ID, got.ID)
}

func TestServiceHandler_Get_NotFound(t *testing.T) {
	e := echo.New()
	store := NewServiceStore()
	handler := NewServiceHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/services/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	err := handler.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServiceHandler_Delete_Found(t *testing.T) {
	e := echo.New()
	store := NewServiceStore()
	handler := NewServiceHandler(store)

	svc := &Service{
		ID:       uuid.New(),
		OrderID:  uuid.New(),
		TenantID: uuid.New(),
		Name:     "svc-test",
		Status:   ServiceStatusActive,
		Created:  time.Now().UTC(),
		Updated:  time.Now().UTC(),
	}
	require.NoError(t, store.Create(svc))

	req := httptest.NewRequest(http.MethodDelete, "/services/"+svc.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(svc.ID.String())

	err := handler.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestServiceHandler_Delete_NotFound(t *testing.T) {
	e := echo.New()
	store := NewServiceStore()
	handler := NewServiceHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/services/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	err := handler.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
