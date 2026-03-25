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
	"net/http"
	"time"

	echo "github.com/labstack/echo/v4"

	"github.com/google/uuid"
)

// createTemplateRequest is the JSON body for creating a service template.
type createTemplateRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Version     string              `json:"version"`
	Parameters  []TemplateParameter `json:"parameters"`
	Labels      map[string]string   `json:"labels,omitempty"`
}

// updateTemplateRequest is the JSON body for patching a service template.
type updateTemplateRequest struct {
	Name        *string              `json:"name,omitempty"`
	Description *string              `json:"description,omitempty"`
	Version     *string              `json:"version,omitempty"`
	Parameters  *[]TemplateParameter `json:"parameters,omitempty"`
	Labels      *map[string]string   `json:"labels,omitempty"`
	IsActive    *bool                `json:"is_active,omitempty"`
}

// handleCreateTemplate handles POST /catalog/templates.
func handleCreateTemplate(store *TemplateStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req createTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Failed to parse request body",
			})
		}

		if req.Name == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "Name is required",
			})
		}
		if req.Version == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "Version is required",
			})
		}

		now := time.Now().UTC()
		t := &ServiceTemplate{
			ID:          uuid.New(),
			Name:        req.Name,
			Description: req.Description,
			Version:     req.Version,
			Parameters:  req.Parameters,
			Labels:      req.Labels,
			IsActive:    true,
			Created:     now,
			Updated:     now,
		}
		if t.Parameters == nil {
			t.Parameters = []TemplateParameter{}
		}

		if err := store.Create(t); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error":   "internal_error",
				"message": err.Error(),
			})
		}

		return c.JSON(http.StatusCreated, t)
	}
}

// handleListTemplates handles GET /catalog/templates.
func handleListTemplates(store *TemplateStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		all := store.GetAll()
		active := make([]*ServiceTemplate, 0, len(all))
		for _, t := range all {
			if t.IsActive {
				active = append(active, t)
			}
		}
		return c.JSON(http.StatusOK, active)
	}
}

// handleGetTemplate handles GET /catalog/templates/:id.
func handleGetTemplate(store *TemplateStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Invalid template ID format",
			})
		}

		t, err := store.GetByID(id)
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error":   "not_found",
				"message": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, t)
	}
}

// handleUpdateTemplate handles PATCH /catalog/templates/:id.
func handleUpdateTemplate(store *TemplateStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Invalid template ID format",
			})
		}

		t, err := store.GetByID(id)
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error":   "not_found",
				"message": err.Error(),
			})
		}

		var req updateTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Failed to parse request body",
			})
		}

		if req.Name != nil {
			t.Name = *req.Name
		}
		if req.Description != nil {
			t.Description = *req.Description
		}
		if req.Version != nil {
			t.Version = *req.Version
		}
		if req.Parameters != nil {
			t.Parameters = *req.Parameters
		}
		if req.Labels != nil {
			t.Labels = *req.Labels
		}
		if req.IsActive != nil {
			t.IsActive = *req.IsActive
		}
		t.Updated = time.Now().UTC()

		if err := store.Update(t); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error":   "internal_error",
				"message": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, t)
	}
}

// handleDeleteTemplate handles DELETE /catalog/templates/:id.
// This performs a soft delete by setting IsActive to false.
func handleDeleteTemplate(store *TemplateStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Invalid template ID format",
			})
		}

		t, err := store.GetByID(id)
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error":   "not_found",
				"message": err.Error(),
			})
		}

		t.IsActive = false
		t.Updated = time.Now().UTC()

		if err := store.Update(t); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error":   "internal_error",
				"message": err.Error(),
			})
		}

		return c.NoContent(http.StatusNoContent)
	}
}
