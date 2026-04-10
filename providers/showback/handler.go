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
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid service id"})
	}
	summary := p.store.GetUsageByService(id)
	return c.JSON(http.StatusOK, summary)
}

// handleGetSelfUsage returns total usage for the current tenant.
// GET /self/usage
// Extracts tenant ID from auth context. Returns aggregated metrics
// from the usage store for the current billing period.
func (p *ShowbackProvider) handleGetSelfUsage(c echo.Context) error {
	// Extract tenant ID from auth context (set by NICo auth middleware).
	tenantIDStr := c.Get("tenant_id")
	if tenantIDStr == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized", "message": "tenant_id not found in auth context"})
	}

	str, ok := tenantIDStr.(string)
	if !ok || str == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized", "message": "tenant_id not found in auth context"})
	}

	tenantID, err := uuid.Parse(str)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid tenant_id"})
	}

	summary := p.store.GetUsageByTenant(tenantID)
	summary.Period = "current-month"
	return c.JSON(http.StatusOK, summary)
}

// handleGetSelfQuotas returns quota limits and current usage for the
// current tenant. Quota limits are derived from the tenant's allocation
// constraints. Current values are computed from the usage store.
func (p *ShowbackProvider) handleGetSelfQuotas(c echo.Context) error {
	tenantIDStr := c.Get("tenant_id")
	if tenantIDStr == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized", "message": "tenant_id not found in auth context"})
	}

	str, ok := tenantIDStr.(string)
	if !ok || str == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized", "message": "tenant_id not found in auth context"})
	}

	tenantID, err := uuid.Parse(str)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid tenant_id"})
	}

	// Get current usage from store
	usage := p.store.GetUsageByTenant(tenantID)

	// Build quota info from usage metrics.
	// Quota limits would come from the tenant's allocation constraints
	// in production (via compute service interface).
	quotas := make(map[string]QuotaLimit)
	for metric, current := range usage.Metrics {
		unit := "hours"
		if metric == "storage-gb-hours" {
			unit = "gb-hours"
		}
		quotas[metric] = QuotaLimit{
			Current: current,
			Unit:    unit,
			// Limit would be populated from allocation constraints
		}
	}

	info := QuotaInfo{
		TenantID: tenantID,
		Quotas:   quotas,
	}
	return c.JSON(http.StatusOK, info)
}

// handleGetSelfUsageCosts returns usage with cost breakdown for the current tenant.
// Costs are derived from usage metrics multiplied by the rate table.
func (p *ShowbackProvider) handleGetSelfUsageCosts(c echo.Context) error {
	tenantIDStr := c.Get("tenant_id")
	if tenantIDStr == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized", "message": "tenant_id not found in auth context"})
	}

	str, ok := tenantIDStr.(string)
	if !ok || str == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized", "message": "tenant_id not found in auth context"})
	}

	tenantID, err := uuid.Parse(str)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid_id", "message": "invalid tenant_id"})
	}

	usage := p.store.GetUsageByTenant(tenantID)

	costs := make(map[string]CostDetail)
	var totalCost float64
	currency := "USD"

	for metric, quantity := range usage.Metrics {
		rate := 0.0
		if entry, ok := p.rates[metric]; ok {
			rate = entry.Rate
			currency = entry.Currency
		}
		cost := quantity * rate
		totalCost += cost
		costs[metric] = CostDetail{
			Quantity: quantity,
			Rate:     rate,
			Unit:     metric,
			Cost:     cost,
		}
	}

	return c.JSON(http.StatusOK, UsageCostSummary{
		TenantID:  tenantID,
		Period:    "current-month",
		Metrics:   usage.Metrics,
		Costs:     costs,
		TotalCost: totalCost,
		Currency:  currency,
	})
}
