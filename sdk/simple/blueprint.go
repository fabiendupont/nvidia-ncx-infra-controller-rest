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
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard"
)

// Blueprint represents a catalog blueprint in the simple SDK.
type Blueprint struct {
	ID          string                                 `json:"id"`
	Name        string                                 `json:"name"`
	Version     string                                 `json:"version"`
	Description string                                 `json:"description"`
	Parameters  map[string]standard.BlueprintParameter `json:"parameters,omitempty"`
	Resources   map[string]standard.BlueprintResource  `json:"resources,omitempty"`
	Labels      map[string]string                      `json:"labels,omitempty"`
	Pricing     *standard.PricingSpec                  `json:"pricing,omitempty"`
	TenantID    *string                                `json:"tenant_id,omitempty"`
	Visibility  string                                 `json:"visibility"`
	BasedOn     string                                 `json:"based_on,omitempty"`
	IsActive    bool                                   `json:"is_active"`
	Created     time.Time                              `json:"created"`
	Updated     time.Time                              `json:"updated"`
}

// BlueprintCreateRequest is a request to create a blueprint.
type BlueprintCreateRequest struct {
	Name        string                                 `json:"name"`
	Version     string                                 `json:"version"`
	Description string                                 `json:"description"`
	Parameters  map[string]standard.BlueprintParameter `json:"parameters,omitempty"`
	Resources   map[string]standard.BlueprintResource  `json:"resources,omitempty"`
	Labels      map[string]string                      `json:"labels,omitempty"`
	Pricing     *standard.PricingSpec                  `json:"pricing,omitempty"`
	Visibility  string                                 `json:"visibility,omitempty"`
	BasedOn     string                                 `json:"based_on,omitempty"`
}

// BlueprintUpdateRequest is a request to update a blueprint.
type BlueprintUpdateRequest struct {
	Name        string                                 `json:"name,omitempty"`
	Version     string                                 `json:"version,omitempty"`
	Description string                                 `json:"description,omitempty"`
	Parameters  map[string]standard.BlueprintParameter `json:"parameters,omitempty"`
	Resources   map[string]standard.BlueprintResource  `json:"resources,omitempty"`
	Labels      map[string]string                      `json:"labels,omitempty"`
	Pricing     *standard.PricingSpec                  `json:"pricing,omitempty"`
	Visibility  string                                 `json:"visibility,omitempty"`
}

// BlueprintFilter encapsulates filter parameters for listing blueprints.
type BlueprintFilter struct {
	TenantID *string
}

// BlueprintValidationResult holds the result of blueprint validation.
type BlueprintValidationResult = standard.ValidateBlueprint200Response

// CostEstimate represents the estimated cost for a blueprint.
type CostEstimate = standard.CostEstimate

// BlueprintManager manages blueprint operations.
type BlueprintManager struct {
	client *Client
}

// NewBlueprintManager creates a new BlueprintManager.
func NewBlueprintManager(client *Client) BlueprintManager {
	return BlueprintManager{client: client}
}

func blueprintFromStandard(api standard.Blueprint) Blueprint {
	b := Blueprint{
		ID:         api.Id,
		Name:       api.Name,
		Version:    api.Version,
		Parameters: api.Parameters,
		Resources:  api.Resources,
		Labels:     api.Labels,
		Pricing:    api.Pricing,
		TenantID:   api.TenantId,
		Visibility: api.Visibility,
		IsActive:   api.IsActive,
	}
	if api.Description != nil {
		b.Description = *api.Description
	}
	if api.BasedOn != nil {
		b.BasedOn = *api.BasedOn
	}
	if api.Created != nil {
		b.Created = *api.Created
	}
	if api.Updated != nil {
		b.Updated = *api.Updated
	}
	return b
}

func toStandardCreateBlueprintRequest(request BlueprintCreateRequest) standard.CreateBlueprintRequest {
	apiReq := *standard.NewCreateBlueprintRequest(request.Name, request.Version)
	if request.Description != "" {
		apiReq.Description = &request.Description
	}
	apiReq.Parameters = request.Parameters
	apiReq.Resources = request.Resources
	apiReq.Labels = request.Labels
	apiReq.Pricing = request.Pricing
	if request.Visibility != "" {
		apiReq.Visibility = &request.Visibility
	}
	if request.BasedOn != "" {
		apiReq.BasedOn = &request.BasedOn
	}
	return apiReq
}

func toStandardUpdateBlueprintRequest(request BlueprintUpdateRequest) standard.UpdateBlueprintRequest {
	apiReq := standard.UpdateBlueprintRequest{}
	if request.Name != "" {
		apiReq.Name = &request.Name
	}
	if request.Version != "" {
		apiReq.Version = &request.Version
	}
	if request.Description != "" {
		apiReq.Description = &request.Description
	}
	apiReq.Parameters = request.Parameters
	apiReq.Resources = request.Resources
	apiReq.Labels = request.Labels
	apiReq.Pricing = request.Pricing
	if request.Visibility != "" {
		apiReq.Visibility = &request.Visibility
	}
	return apiReq
}

// Create creates a new blueprint.
func (bm BlueprintManager) Create(ctx context.Context, request BlueprintCreateRequest) (*Blueprint, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	apiReq := toStandardCreateBlueprintRequest(request)
	apiBlueprint, resp, err := bm.client.apiClient.CatalogAPI.CreateBlueprint(ctx, bm.client.apiMetadata.Organization).
		CreateBlueprintRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	b := blueprintFromStandard(*apiBlueprint)
	return &b, nil
}

// GetBlueprints returns all blueprints with optional filtering and pagination.
func (bm BlueprintManager) GetBlueprints(ctx context.Context, filter *BlueprintFilter, paginationFilter *OffsetPaginationFilter) ([]Blueprint, *OffsetPaginationResponse, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	req := bm.client.apiClient.CatalogAPI.ListBlueprints(ctx, bm.client.apiMetadata.Organization)
	if filter != nil && filter.TenantID != nil {
		req = req.TenantId(*filter.TenantID)
	}
	if paginationFilter != nil {
		if paginationFilter.Offset != nil {
			req = req.Offset(int32(*paginationFilter.Offset))
		}
		if paginationFilter.Limit != nil {
			req = req.Limit(int32(*paginationFilter.Limit))
		}
	}

	listResp, resp, err := req.Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, nil, apiErr
	}

	blueprints := make([]Blueprint, 0, len(listResp.GetItems()))
	for _, api := range listResp.GetItems() {
		blueprints = append(blueprints, blueprintFromStandard(api))
	}

	pagination := &OffsetPaginationResponse{
		Total:  int(listResp.GetTotal()),
		Offset: int(listResp.GetOffset()),
		Limit:  int(listResp.GetLimit()),
	}
	return blueprints, pagination, nil
}

// GetBlueprint returns a blueprint by ID.
func (bm BlueprintManager) GetBlueprint(ctx context.Context, id string) (*Blueprint, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	apiBlueprint, resp, err := bm.client.apiClient.CatalogAPI.GetBlueprint(ctx, bm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	b := blueprintFromStandard(*apiBlueprint)
	return &b, nil
}

// UpdateBlueprint updates an existing blueprint.
func (bm BlueprintManager) UpdateBlueprint(ctx context.Context, id string, request BlueprintUpdateRequest) (*Blueprint, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	apiReq := toStandardUpdateBlueprintRequest(request)
	apiBlueprint, resp, err := bm.client.apiClient.CatalogAPI.UpdateBlueprint(ctx, bm.client.apiMetadata.Organization, id).
		UpdateBlueprintRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	b := blueprintFromStandard(*apiBlueprint)
	return &b, nil
}

// DeleteBlueprint deletes a blueprint by ID.
func (bm BlueprintManager) DeleteBlueprint(ctx context.Context, id string) *ApiError {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	resp, err := bm.client.apiClient.CatalogAPI.DeleteBlueprint(ctx, bm.client.apiMetadata.Organization, id).Execute()
	return HandleResponseError(resp, err)
}

// ValidateBlueprint validates a blueprint and returns the validation result.
func (bm BlueprintManager) ValidateBlueprint(ctx context.Context, id string) (*BlueprintValidationResult, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	result, resp, err := bm.client.apiClient.CatalogAPI.ValidateBlueprint(ctx, bm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetResolvedBlueprint returns the effective blueprint after variant resolution.
func (bm BlueprintManager) GetResolvedBlueprint(ctx context.Context, id string) (*Blueprint, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	apiBlueprint, resp, err := bm.client.apiClient.CatalogAPI.GetResolvedBlueprint(ctx, bm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	b := blueprintFromStandard(*apiBlueprint)
	return &b, nil
}

// EstimateBlueprintCost returns a cost estimate for a blueprint.
func (bm BlueprintManager) EstimateBlueprintCost(ctx context.Context, id string) (*CostEstimate, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	result, resp, err := bm.client.apiClient.CatalogAPI.EstimateBlueprintCost(ctx, bm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetResourceTypes returns the list of available resource types.
func (bm BlueprintManager) GetResourceTypes(ctx context.Context) ([]string, *ApiError) {
	ctx = WithLogger(ctx, bm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, bm.client.Config.Token)

	result, resp, err := bm.client.apiClient.CatalogAPI.ListResourceTypes(ctx, bm.client.apiMetadata.Organization).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result.GetResourceTypes(), nil
}
