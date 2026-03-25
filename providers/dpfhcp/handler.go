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

package dpfhcp

import (
	"net/http"
	"time"

	echo "github.com/labstack/echo/v4"
)

// handleProvision handles POST /sites/:siteId/dpf-hcp.
func handleProvision(store *ProvisioningStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		siteID := c.Param("siteId")
		if siteID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "Site ID is required",
			})
		}

		var req DPFHCPRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "bad_request",
				"message": "Failed to parse request body",
			})
		}

		if req.DPUClusterRef.Name == "" || req.DPUClusterRef.Namespace == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "dpuClusterRef name and namespace are required",
			})
		}
		if req.BaseDomain == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "baseDomain is required",
			})
		}
		if req.OCPReleaseImage == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "ocpReleaseImage is required",
			})
		}
		if req.SSHKeySecretRef == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "sshKeySecretRef is required",
			})
		}
		if req.PullSecretRef == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "pullSecretRef is required",
			})
		}

		now := time.Now().UTC()
		record := &ProvisioningRecord{
			SiteID:  siteID,
			Config:  req,
			Status:  StatusPending,
			Created: now,
			Updated: now,
		}

		if err := store.Create(record); err != nil {
			return c.JSON(http.StatusConflict, echo.Map{
				"error":   "conflict",
				"message": err.Error(),
			})
		}

		return c.JSON(http.StatusCreated, record)
	}
}

// handleGetStatus handles GET /sites/:siteId/dpf-hcp.
func handleGetStatus(store *ProvisioningStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		siteID := c.Param("siteId")
		if siteID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "Site ID is required",
			})
		}

		record, err := store.GetBySiteID(siteID)
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error":   "not_found",
				"message": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, record)
	}
}

// handleDelete handles DELETE /sites/:siteId/dpf-hcp.
func handleDelete(store *ProvisioningStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		siteID := c.Param("siteId")
		if siteID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error":   "validation_error",
				"message": "Site ID is required",
			})
		}

		record, err := store.GetBySiteID(siteID)
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error":   "not_found",
				"message": err.Error(),
			})
		}

		record.Status = StatusDeleting
		record.Updated = time.Now().UTC()

		if err := store.Update(record); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error":   "internal_error",
				"message": err.Error(),
			})
		}

		return c.NoContent(http.StatusAccepted)
	}
}
