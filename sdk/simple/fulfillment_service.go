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

// FulfillmentService represents a provisioned service in the simple SDK.
type FulfillmentService struct {
	ID            string            `json:"id"`
	OrderID       string            `json:"order_id"`
	BlueprintID   string            `json:"blueprint_id"`
	BlueprintName string            `json:"blueprint_name"`
	TenantID      string            `json:"tenant_id"`
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	Resources     map[string]string `json:"resources,omitempty"`
	Created       time.Time         `json:"created"`
	Updated       time.Time         `json:"updated"`
}

// FulfillmentServiceUpdateRequest is a request to update a service.
type FulfillmentServiceUpdateRequest struct {
	Name      *string           `json:"name,omitempty"`
	Resources map[string]string `json:"resources,omitempty"`
}

// FulfillmentServiceFilter encapsulates filter parameters for listing services.
type FulfillmentServiceFilter struct {
	TenantID *string
}

// FulfillmentServiceManager manages fulfillment service operations.
type FulfillmentServiceManager struct {
	client *Client
}

// NewFulfillmentServiceManager creates a new FulfillmentServiceManager.
func NewFulfillmentServiceManager(client *Client) FulfillmentServiceManager {
	return FulfillmentServiceManager{client: client}
}

func fulfillmentServiceFromStandard(api standard.FulfillmentService) FulfillmentService {
	s := FulfillmentService{
		Resources: api.Resources,
	}
	if api.Id != nil {
		s.ID = *api.Id
	}
	if api.OrderId != nil {
		s.OrderID = *api.OrderId
	}
	if api.BlueprintId != nil {
		s.BlueprintID = *api.BlueprintId
	}
	if api.BlueprintName != nil {
		s.BlueprintName = *api.BlueprintName
	}
	if api.TenantId != nil {
		s.TenantID = *api.TenantId
	}
	if api.Name != nil {
		s.Name = *api.Name
	}
	if api.Status != nil {
		s.Status = *api.Status
	}
	if api.Created != nil {
		s.Created = *api.Created
	}
	if api.Updated != nil {
		s.Updated = *api.Updated
	}
	return s
}

// GetServices returns all services with optional filtering and pagination.
func (sm FulfillmentServiceManager) GetServices(ctx context.Context, filter *FulfillmentServiceFilter, paginationFilter *OffsetPaginationFilter) ([]FulfillmentService, *OffsetPaginationResponse, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	req := sm.client.apiClient.FulfillmentAPI.ListServices(ctx, sm.client.apiMetadata.Organization)
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

	services := make([]FulfillmentService, 0, len(listResp.GetItems()))
	for _, api := range listResp.GetItems() {
		services = append(services, fulfillmentServiceFromStandard(api))
	}

	pagination := &OffsetPaginationResponse{
		Total:  int(listResp.GetTotal()),
		Offset: int(listResp.GetOffset()),
		Limit:  int(listResp.GetLimit()),
	}
	return services, pagination, nil
}

// GetService returns a service by ID.
func (sm FulfillmentServiceManager) GetService(ctx context.Context, id string) (*FulfillmentService, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	apiSvc, resp, err := sm.client.apiClient.FulfillmentAPI.GetService(ctx, sm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	svc := fulfillmentServiceFromStandard(*apiSvc)
	return &svc, nil
}

// UpdateService updates an existing service.
func (sm FulfillmentServiceManager) UpdateService(ctx context.Context, id string, request FulfillmentServiceUpdateRequest) (*FulfillmentService, *ApiError) {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	apiReq := standard.UpdateServiceRequest{}
	if request.Name != nil {
		apiReq.Name = request.Name
	}
	apiReq.Resources = request.Resources

	apiSvc, resp, err := sm.client.apiClient.FulfillmentAPI.UpdateService(ctx, sm.client.apiMetadata.Organization, id).
		UpdateServiceRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	svc := fulfillmentServiceFromStandard(*apiSvc)
	return &svc, nil
}

// DeleteService requests teardown of a service.
func (sm FulfillmentServiceManager) DeleteService(ctx context.Context, id string) *ApiError {
	ctx = WithLogger(ctx, sm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, sm.client.Config.Token)

	resp, err := sm.client.apiClient.FulfillmentAPI.DeleteService(ctx, sm.client.apiMetadata.Organization, id).Execute()
	return HandleResponseError(resp, err)
}
