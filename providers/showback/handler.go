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
	"net/http"

	echo "github.com/labstack/echo/v4"

	"github.com/google/uuid"
)

// handleGetServiceUsage returns usage for a specific service.
// GET /services/:id/usage
func (p *ShowbackProvider) handleGetServiceUsage(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid service id",
		})
	}
	summary := p.store.GetUsageByService(id)
	return c.JSON(http.StatusOK, summary)
}

// handleGetSelfUsage returns total usage for the current tenant.
// GET /self/usage
// Placeholder: returns mock data.
func (p *ShowbackProvider) handleGetSelfUsage(c echo.Context) error {
	summary := UsageSummary{
		TenantID: uuid.Nil,
		Period:   "current-month",
		Metrics: map[string]float64{
			"gpu-hours":        120.5,
			"storage-gb-hours": 2048.0,
		},
	}
	return c.JSON(http.StatusOK, summary)
}

// handleGetSelfQuotas returns quota limits and current usage for the current tenant.
// GET /self/quotas
// Placeholder: returns mock data.
func (p *ShowbackProvider) handleGetSelfQuotas(c echo.Context) error {
	info := QuotaInfo{
		TenantID: uuid.Nil,
		Quotas: map[string]QuotaLimit{
			"gpu-hours": {
				Limit:   1000.0,
				Current: 120.5,
				Unit:    "hours",
			},
			"storage-gb-hours": {
				Limit:   10000.0,
				Current: 2048.0,
				Unit:    "gb-hours",
			},
		},
	}
	return c.JSON(http.StatusOK, info)
}
