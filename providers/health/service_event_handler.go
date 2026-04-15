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
	"net/http"
	"time"

	echo "github.com/labstack/echo/v4"

	"github.com/google/uuid"
)

// ServiceEventHandler groups the tenant-facing service event HTTP handlers
// and the stores they depend on. Responses deliberately omit machine IDs,
// rack locations, classifications, and remediation details.
type ServiceEventHandler struct {
	serviceEventStore *ServiceEventStore
	faultServiceStore *FaultServiceEventStore
}

// NewServiceEventHandler creates a ServiceEventHandler with the given stores.
func NewServiceEventHandler(
	serviceEventStore *ServiceEventStore,
	faultServiceStore *FaultServiceEventStore,
) *ServiceEventHandler {
	return &ServiceEventHandler{
		serviceEventStore: serviceEventStore,
		faultServiceStore: faultServiceStore,
	}
}

// serviceEventResponse is the tenant-safe view of a ServiceEvent.
// It contains only summary, impact, state, timing, and billing fields.
// No machine IDs, rack locations, classifications, or remediation details.
type serviceEventResponse struct {
	ID                    string     `json:"id"`
	Summary               string     `json:"summary"`
	Impact                string     `json:"impact"`
	State                 string     `json:"state"`
	StartedAt             time.Time  `json:"started_at"`
	EstimatedResolutionAt *time.Time `json:"estimated_resolution_at,omitempty"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
	DowntimeExcluded      bool       `json:"downtime_excluded"`
}

// toServiceEventResponse converts a ServiceEvent to a tenant-safe response
// that excludes all infrastructure details.
func toServiceEventResponse(e *ServiceEvent) *serviceEventResponse {
	return &serviceEventResponse{
		ID:                    e.ID,
		Summary:               e.Summary,
		Impact:                e.Impact,
		State:                 e.State,
		StartedAt:             e.StartedAt,
		EstimatedResolutionAt: e.EstimatedResolutionAt,
		ResolvedAt:            e.ResolvedAt,
		DowntimeExcluded:      e.DowntimeExcluded,
	}
}

// handleListServiceEvents handles GET /tenant/:tenantId/service-events.
// Lists service events for a tenant, optionally filtered by state.
func (h *ServiceEventHandler) handleListServiceEvents(c echo.Context) error {
	tenantID := c.Param("tenantId")
	if _, err := uuid.Parse(tenantID); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid tenant ID format",
		})
	}

	events := h.serviceEventStore.GetByTenantID(tenantID)

	// Apply optional state filter.
	state := c.QueryParam("state")
	if state != "" {
		events = filterServiceEventsByState(events, state)
	}

	resp := make([]*serviceEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, toServiceEventResponse(e))
	}

	return c.JSON(http.StatusOK, resp)
}

// handleGetActiveServiceEvents handles GET /tenant/:tenantId/service-events/active.
// Returns only active service events for the tenant.
func (h *ServiceEventHandler) handleGetActiveServiceEvents(c echo.Context) error {
	tenantID := c.Param("tenantId")
	if _, err := uuid.Parse(tenantID); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid tenant ID format",
		})
	}

	events := h.serviceEventStore.GetByTenantID(tenantID)
	events = filterServiceEventsByState(events, "active")

	resp := make([]*serviceEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, toServiceEventResponse(e))
	}

	return c.JSON(http.StatusOK, resp)
}

// handleGetServiceEvent handles GET /tenant/:tenantId/service-events/:id.
// Returns a single service event or 404 if not found.
func (h *ServiceEventHandler) handleGetServiceEvent(c echo.Context) error {
	tenantID := c.Param("tenantId")
	if _, err := uuid.Parse(tenantID); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid tenant ID format",
		})
	}

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid service event ID format",
		})
	}

	event, err := h.serviceEventStore.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{
			"error":   "not_found",
			"message": err.Error(),
		})
	}

	// Verify the event belongs to the requested tenant.
	if event.TenantID != tenantID {
		return c.JSON(http.StatusNotFound, echo.Map{
			"error":   "not_found",
			"message": "service event not found",
		})
	}

	return c.JSON(http.StatusOK, toServiceEventResponse(event))
}

// filterServiceEventsByState returns only events matching the given state.
func filterServiceEventsByState(events []*ServiceEvent, state string) []*ServiceEvent {
	filtered := make([]*ServiceEvent, 0, len(events))
	for _, e := range events {
		if e.State == state {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
