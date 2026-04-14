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

package simple

import (
	"context"

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard"
)

// UsageSummary aggregates usage metrics for a tenant over a period.
type UsageSummary = standard.UsageSummary

// UsageCostSummary extends UsageSummary with cost breakdown.
type UsageCostSummary = standard.UsageCostSummary

// CostDetail shows the cost for a single metric.
type CostDetail = standard.CostDetail

// QuotaInfo reports quota limits and current usage for a tenant.
type QuotaInfo = standard.QuotaInfo

// QuotaLimit describes a single quota with its limit and current usage.
type QuotaLimit = standard.QuotaLimit

// ShowbackManager manages showback (usage, cost, quota) operations.
type ShowbackManager struct {
	client *Client
}

// NewShowbackManager creates a new ShowbackManager.
func NewShowbackManager(client *Client) ShowbackManager {
	return ShowbackManager{client: client}
}

// GetServiceUsage returns usage metrics for a specific service.
func (sm ShowbackManager) GetServiceUsage(ctx context.Context, serviceID string) (*UsageSummary, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	result, resp, err := sm.client.apiClient.ShowbackAPI.GetServiceUsage(ctx, sm.client.apiMetadata.Organization, serviceID).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetSelfUsage returns total usage for the current tenant.
func (sm ShowbackManager) GetSelfUsage(ctx context.Context) (*UsageSummary, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	result, resp, err := sm.client.apiClient.ShowbackAPI.GetSelfUsage(ctx, sm.client.apiMetadata.Organization).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetSelfUsageCosts returns usage with cost breakdown for the current tenant.
func (sm ShowbackManager) GetSelfUsageCosts(ctx context.Context) (*UsageCostSummary, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	result, resp, err := sm.client.apiClient.ShowbackAPI.GetUsageCosts(ctx, sm.client.apiMetadata.Organization).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetSelfQuotas returns quota limits and current usage for the current tenant.
func (sm ShowbackManager) GetSelfQuotas(ctx context.Context) (*QuotaInfo, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	result, resp, err := sm.client.apiClient.ShowbackAPI.GetSelfQuotas(ctx, sm.client.apiMetadata.Organization).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}
