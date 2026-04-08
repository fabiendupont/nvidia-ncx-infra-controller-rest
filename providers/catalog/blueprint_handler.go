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

	"github.com/labstack/echo/v4"
)

// BlueprintHandler handles blueprint API requests.
type BlueprintHandler struct {
	store *BlueprintStore
}

// NewBlueprintHandler creates a new handler.
func NewBlueprintHandler(store *BlueprintStore) *BlueprintHandler {
	return &BlueprintHandler{store: store}
}

func (h *BlueprintHandler) handleCreateBlueprint(c echo.Context) error {
	var b Blueprint
	if err := c.Bind(&b); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_request", "message": err.Error()})
	}

	result := ValidateBlueprint(&b)
	if !result.Valid {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "validation_failed", "message": "Blueprint validation failed", "details": result.Errors})
	}

	if err := h.store.Create(&b); err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "conflict", "message": err.Error()})
	}

	return c.JSON(http.StatusCreated, b)
}

func (h *BlueprintHandler) handleListBlueprints(c echo.Context) error {
	blueprints := h.store.GetAll()
	if blueprints == nil {
		blueprints = []*Blueprint{}
	}
	return c.JSON(http.StatusOK, blueprints)
}

func (h *BlueprintHandler) handleGetBlueprint(c echo.Context) error {
	id := c.Param("id")
	b, err := h.store.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}
	return c.JSON(http.StatusOK, b)
}

func (h *BlueprintHandler) handleUpdateBlueprint(c echo.Context) error {
	id := c.Param("id")
	existing, err := h.store.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}

	var update Blueprint
	if err := c.Bind(&update); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_request", "message": err.Error()})
	}

	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Version != "" {
		existing.Version = update.Version
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Parameters != nil {
		existing.Parameters = update.Parameters
	}
	if update.Resources != nil {
		existing.Resources = update.Resources
	}
	if update.Labels != nil {
		existing.Labels = update.Labels
	}

	result := ValidateBlueprint(existing)
	if !result.Valid {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "validation_failed", "message": "Updated blueprint is invalid", "details": result.Errors})
	}

	if err := h.store.Update(existing); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "update_failed", "message": err.Error()})
	}

	return c.JSON(http.StatusOK, existing)
}

func (h *BlueprintHandler) handleDeleteBlueprint(c echo.Context) error {
	id := c.Param("id")
	if err := h.store.Delete(id); err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *BlueprintHandler) handleValidateBlueprint(c echo.Context) error {
	id := c.Param("id")
	b, err := h.store.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not_found", "message": err.Error()})
	}
	result := ValidateBlueprint(b)
	return c.JSON(http.StatusOK, result)
}

func (h *BlueprintHandler) handleListResourceTypes(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"resource_types": AvailableResourceTypes})
}
