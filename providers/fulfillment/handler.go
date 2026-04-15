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
	"net/http"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	echo "github.com/labstack/echo/v4"

	"github.com/google/uuid"
)

// createOrderRequest is the payload for placing a new order.
type createOrderRequest struct {
	BlueprintID   uuid.UUID              `json:"blueprint_id"`
	BlueprintName string                 `json:"blueprint_name"`
	TenantID      uuid.UUID              `json:"tenant_id"`
	Parameters    map[string]interface{} `json:"parameters"`
}

// updateServiceRequest is the payload for updating a service.
type updateServiceRequest struct {
	Name      *string           `json:"name,omitempty"`
	Resources map[string]string `json:"resources,omitempty"`
}

// BlueprintValidator checks whether a blueprint ID exists.
// Returns the blueprint name if found, or an error.
type BlueprintValidator func(id uuid.UUID) (name string, err error)

// OrderHandler handles order-related HTTP requests.
type OrderHandler struct {
	orders            OrderStoreInterface
	validateBlueprint BlueprintValidator
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(orders OrderStoreInterface) *OrderHandler {
	return &OrderHandler{orders: orders}
}

// WithBlueprintValidator sets the blueprint validation function.
func (h *OrderHandler) WithBlueprintValidator(v BlueprintValidator) *OrderHandler {
	h.validateBlueprint = v
	return h
}

// Create handles POST requests to place a new order.
func (h *OrderHandler) Create(c echo.Context) error {
	var req createOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad_request", "message": "failed to parse request body"})
	}

	if req.BlueprintID == uuid.Nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad_request", "message": "blueprint_id is required"})
	}
	if req.TenantID == uuid.Nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad_request", "message": "tenant_id is required"})
	}

	// Validate that the referenced blueprint exists
	if h.validateBlueprint != nil {
		name, err := h.validateBlueprint(req.BlueprintID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad_request", "message": "blueprint not found"})
		}
		if req.BlueprintName == "" {
			req.BlueprintName = name
		}
	}

	now := time.Now().UTC()
	order := &Order{
		ID:            uuid.New(),
		BlueprintID:   req.BlueprintID,
		BlueprintName: req.BlueprintName,
		TenantID:      req.TenantID,
		Parameters:    req.Parameters,
		Status:        OrderStatusPending,
		Created:       now,
		Updated:       now,
	}

	if err := h.orders.Create(order); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal_error", "message": "an internal error occurred"})
	}

	return c.JSON(http.StatusCreated, order)
}

// callerTenantID extracts the caller's tenant_id from auth context.
// Returns empty string if not set (dev mode).
func callerTenantID(c echo.Context) string {
	if s, ok := c.Get("tenant_id").(string); ok {
		return s
	}
	return ""
}

// Get handles GET requests to retrieve an order by ID.
func (h *OrderHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid order id"})
	}

	order, err := h.orders.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}

	// Tenant isolation: caller can only see their own orders
	if tid := callerTenantID(c); tid != "" && order.TenantID.String() != tid {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": "order not found"})
	}

	return c.JSON(http.StatusOK, order)
}

// List handles GET requests to list orders with optional tenant and status filtering.
func (h *OrderHandler) List(c echo.Context) error {
	tenantParam := c.QueryParam("tenant_id")
	statusParam := c.QueryParam("status")

	var orders []*Order
	if tenantParam != "" {
		tenantID, err := uuid.Parse(tenantParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid tenant_id"})
		}
		orders = h.orders.ListByTenant(tenantID)
	} else {
		orders = h.orders.List()
	}

	if statusParam != "" {
		var filtered []*Order
		for _, o := range orders {
			if string(o.Status) == statusParam {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}

	if orders == nil {
		orders = []*Order{}
	}

	offset, limit := provider.ParsePagination(c)
	total := len(orders)
	start, end := provider.Paginate(total, offset, limit)
	page := orders[start:end]
	if page == nil {
		page = []*Order{}
	}

	return c.JSON(http.StatusOK, provider.ListResponse{
		Items:  page,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// Cancel handles DELETE requests to cancel an order.
func (h *OrderHandler) Cancel(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid order id"})
	}

	order, err := h.orders.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}

	if tid := callerTenantID(c); tid != "" && order.TenantID.String() != tid {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": "order not found"})
	}

	order.Status = OrderStatusCancelled
	order.Updated = time.Now().UTC()
	if err := h.orders.Update(order); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal_error", "message": "an internal error occurred"})
	}

	return c.NoContent(http.StatusNoContent)
}

// ServiceHandler handles service-related HTTP requests.
type ServiceHandler struct {
	services ServiceStoreInterface
}

// NewServiceHandler creates a new ServiceHandler.
func NewServiceHandler(services ServiceStoreInterface) *ServiceHandler {
	return &ServiceHandler{services: services}
}

// List handles GET requests to list active services for a tenant.
func (h *ServiceHandler) List(c echo.Context) error {
	tenantParam := c.QueryParam("tenant_id")
	var services []*Service
	if tenantParam != "" {
		tenantID, err := uuid.Parse(tenantParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid tenant_id"})
		}
		services = h.services.ListByTenant(tenantID)
	} else {
		services = h.services.List()
	}

	if services == nil {
		services = []*Service{}
	}

	offset, limit := provider.ParsePagination(c)
	total := len(services)
	start, end := provider.Paginate(total, offset, limit)
	page := services[start:end]
	if page == nil {
		page = []*Service{}
	}

	return c.JSON(http.StatusOK, provider.ListResponse{
		Items:  page,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// Get handles GET requests to retrieve a service by ID.
func (h *ServiceHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid service id"})
	}

	svc, err := h.services.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}

	if tid := callerTenantID(c); tid != "" && svc.TenantID.String() != tid {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": "service not found"})
	}

	return c.JSON(http.StatusOK, svc)
}

// Update handles PATCH requests to update a service (scale, modify).
func (h *ServiceHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid service id"})
	}

	svc, err := h.services.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}

	if tid := callerTenantID(c); tid != "" && svc.TenantID.String() != tid {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": "service not found"})
	}

	var req updateServiceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad_request", "message": err.Error()})
	}

	if req.Name != nil {
		svc.Name = *req.Name
	}
	if req.Resources != nil {
		for k, v := range req.Resources {
			if svc.Resources == nil {
				svc.Resources = make(map[string]string)
			}
			svc.Resources[k] = v
		}
	}

	svc.Status = ServiceStatusUpdating
	svc.Updated = time.Now().UTC()
	if err := h.services.Update(svc); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal_error", "message": "an internal error occurred"})
	}

	return c.JSON(http.StatusOK, svc)
}

// Delete handles DELETE requests to teardown a service.
func (h *ServiceHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid service id"})
	}

	svc, err := h.services.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}

	if tid := callerTenantID(c); tid != "" && svc.TenantID.String() != tid {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": "service not found"})
	}

	svc.Status = ServiceStatusTerminating
	svc.Updated = time.Now().UTC()
	if err := h.services.Update(svc); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal_error", "message": "an internal error occurred"})
	}

	return c.NoContent(http.StatusNoContent)
}
