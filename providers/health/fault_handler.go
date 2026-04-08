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
	"strconv"
	"time"

	echo "github.com/labstack/echo/v4"

	"github.com/google/uuid"
)

// FaultHandler groups the operator-facing fault event HTTP handlers and the
// stores they depend on. The stores are injected at construction time so that
// the handler does not need to reach into the HealthProvider struct.
type FaultHandler struct {
	faultStore          *FaultEventStore
	classificationStore *ClassificationStore
}

// NewFaultHandler creates a FaultHandler with the given stores.
func NewFaultHandler(
	faultStore *FaultEventStore,
	classificationStore *ClassificationStore,
) *FaultHandler {
	return &FaultHandler{
		faultStore:          faultStore,
		classificationStore: classificationStore,
	}
}

// handleIngestFault handles POST /health/events/ingest.
// It parses a FaultIngestionRequest, creates a FaultEvent with state=open,
// assigns a UUID, and returns 201. If machine_id is provided the handler
// looks up site_id, tenant_id, and instance_id from the store.
func (h *FaultHandler) handleIngestFault(c echo.Context) error {
	var req FaultIngestionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Failed to parse request body",
		})
	}

	if req.Source == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "validation_error",
			"message": "Source is required",
		})
	}
	if req.Severity == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "validation_error",
			"message": "Severity is required",
		})
	}
	if req.Component == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "validation_error",
			"message": "Component is required",
		})
	}
	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "validation_error",
			"message": "Message is required",
		})
	}

	now := time.Now().UTC()
	detectedAt := now
	if req.DetectedAt != nil {
		detectedAt = *req.DetectedAt
	}

	event := &FaultEvent{
		Source:         req.Source,
		Severity:       req.Severity,
		Component:      req.Component,
		Classification: req.Classification,
		Message:        req.Message,
		MachineID:      req.MachineID,
		State:          FaultStateOpen,
		DetectedAt:     detectedAt,
		Metadata:       req.Metadata,
	}

	// If machine_id is provided, resolve site_id / tenant_id / instance_id.
	if req.MachineID != nil {
		resolved, err := h.faultStore.ResolveMachineContext(*req.MachineID)
		if err == nil {
			event.SiteID = resolved.SiteID
			event.TenantID = resolved.TenantID
			event.InstanceID = resolved.InstanceID
		}
	}

	if err := h.faultStore.Create(event); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error":   "internal_error",
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, event)
}

// handleListFaultEvents handles GET /health/events.
// Supports query filters: severity, component, state, site_id, machine_id.
// Supports pagination: limit, offset.
// Supports sorting: sort, order.
func (h *FaultHandler) handleListFaultEvents(c echo.Context) error {
	filter := buildFaultFilter(c)

	events := h.faultStore.GetAll(filter)

	// Apply sorting.
	sortField := c.QueryParam("sort")
	sortOrder := c.QueryParam("order")
	if sortField != "" {
		sortFaultEvents(events, sortField, sortOrder)
	}

	// Apply pagination.
	if v := c.QueryParam("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Invalid offset parameter",
			})
		}
		if n > len(events) {
			n = len(events)
		}
		events = events[n:]
	}
	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Invalid limit parameter",
			})
		}
		if n < len(events) {
			events = events[:n]
		}
	}

	return c.JSON(http.StatusOK, events)
}

// handleGetFaultEvent handles GET /health/events/:id.
// Returns a single fault event or 404 if not found.
func (h *FaultHandler) handleGetFaultEvent(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid fault event ID format",
		})
	}

	event, err := h.faultStore.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{
			"error":   "not_found",
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, event)
}

// handleUpdateFaultEvent handles PATCH /health/events/:id.
// Supports updating state (acknowledge, suppress) and adding metadata.
func (h *FaultHandler) handleUpdateFaultEvent(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid fault event ID format",
		})
	}

	event, err := h.faultStore.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{
			"error":   "not_found",
			"message": err.Error(),
		})
	}

	var req faultEventUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Failed to parse request body",
		})
	}

	now := time.Now().UTC()

	if req.State != nil {
		switch *req.State {
		case FaultStateAcknowledged:
			event.AcknowledgedAt = &now
		case FaultStateSuppressed:
			if req.SuppressedUntil != nil {
				event.SuppressedUntil = req.SuppressedUntil
			}
		case FaultStateResolved:
			event.ResolvedAt = &now
		}
		event.State = *req.State
	}

	if req.Metadata != nil {
		if event.Metadata == nil {
			event.Metadata = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			event.Metadata[k] = v
		}
	}

	updated, err := h.faultStore.Update(event)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error":   "internal_error",
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, updated)
}

// handleGetFaultSummary handles GET /health/events/summary.
// Returns aggregated counts by severity, component, state, and site.
func (h *FaultHandler) handleGetFaultSummary(c echo.Context) error {
	summary := h.faultStore.GetSummary()
	return c.JSON(http.StatusOK, summary)
}

// handleTriggerRemediation handles POST /health/events/:id/remediate.
// Updates the fault event state to remediating and returns 202 Accepted.
func (h *FaultHandler) handleTriggerRemediation(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "bad_request",
			"message": "Invalid fault event ID format",
		})
	}

	event, err := h.faultStore.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{
			"error":   "not_found",
			"message": err.Error(),
		})
	}

	event.State = FaultStateRemediating
	event.RemediationAttempts++

	updated, err := h.faultStore.Update(event)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error":   "internal_error",
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusAccepted, updated)
}

// faultEventUpdateRequest is the JSON body for PATCH /health/events/:id.
type faultEventUpdateRequest struct {
	State           *string                `json:"state,omitempty"`
	SuppressedUntil *time.Time             `json:"suppressed_until,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// buildFaultFilter constructs a FaultEventFilter from query parameters.
func buildFaultFilter(c echo.Context) FaultEventFilter {
	var filter FaultEventFilter

	if v := c.QueryParam("severity"); v != "" {
		filter.Severity = append(filter.Severity, v)
	}
	if v := c.QueryParam("component"); v != "" {
		filter.Component = append(filter.Component, v)
	}
	if v := c.QueryParam("state"); v != "" {
		filter.State = append(filter.State, v)
	}
	if v := c.QueryParam("site_id"); v != "" {
		filter.SiteID = &v
	}
	if v := c.QueryParam("machine_id"); v != "" {
		filter.MachineID = &v
	}

	return filter
}

// sortFaultEvents sorts fault events in place by the given field and order.
func sortFaultEvents(events []*FaultEvent, field, order string) {
	less := func(i, j int) bool {
		switch field {
		case "detected_at":
			return events[i].DetectedAt.Before(events[j].DetectedAt)
		case "created_at":
			return events[i].CreatedAt.Before(events[j].CreatedAt)
		case "updated_at":
			return events[i].UpdatedAt.Before(events[j].UpdatedAt)
		case "severity":
			return events[i].Severity < events[j].Severity
		case "component":
			return events[i].Component < events[j].Component
		case "state":
			return events[i].State < events[j].State
		default:
			return events[i].DetectedAt.Before(events[j].DetectedAt)
		}
	}

	for i := 1; i < len(events); i++ {
		for j := i; j > 0; j-- {
			shouldSwap := less(j, j-1)
			if order == "desc" {
				shouldSwap = !shouldSwap
			}
			if shouldSwap {
				events[j], events[j-1] = events[j-1], events[j]
			} else {
				break
			}
		}
	}
}
