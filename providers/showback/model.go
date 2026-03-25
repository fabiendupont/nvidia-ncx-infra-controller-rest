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
	"time"

	"github.com/google/uuid"
)

// UsageRecord tracks a single metered usage period for a resource.
type UsageRecord struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	ServiceID  uuid.UUID  `json:"service_id"`
	ResourceID uuid.UUID  `json:"resource_id"`
	MetricName string     `json:"metric_name"` // "gpu-hours", "storage-gb-hours"
	Value      float64    `json:"value"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
}

// UsageSummary aggregates usage metrics for a tenant over a period.
type UsageSummary struct {
	TenantID uuid.UUID          `json:"tenant_id"`
	Period   string             `json:"period"` // "current-month", "last-30d"
	Metrics  map[string]float64 `json:"metrics"`
}

// QuotaInfo reports quota limits and current usage for a tenant.
type QuotaInfo struct {
	TenantID uuid.UUID            `json:"tenant_id"`
	Quotas   map[string]QuotaLimit `json:"quotas"`
}

// QuotaLimit describes a single quota with its limit and current usage.
type QuotaLimit struct {
	Limit   float64 `json:"limit"`
	Current float64 `json:"current"`
	Unit    string  `json:"unit"`
}
