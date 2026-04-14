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

// Order represents a catalog order in the simple SDK.
type Order struct {
	ID            string                 `json:"id"`
	BlueprintID   string                 `json:"blueprint_id"`
	BlueprintName string                 `json:"blueprint_name"`
	TenantID      string                 `json:"tenant_id"`
	Parameters    map[string]interface{} `json:"parameters"`
	Status        string                 `json:"status"`
	StatusMessage string                 `json:"status_message,omitempty"`
	WorkflowID    string                 `json:"workflow_id,omitempty"`
	ServiceID     *string                `json:"service_id,omitempty"`
	Created       time.Time              `json:"created"`
	Updated       time.Time              `json:"updated"`
}

// OrderCreateRequest is a request to create an order.
type OrderCreateRequest struct {
	BlueprintID   string                 `json:"blueprint_id"`
	BlueprintName string                 `json:"blueprint_name,omitempty"`
	TenantID      string                 `json:"tenant_id"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
}

// OrderFilter encapsulates filter parameters for listing orders.
type OrderFilter struct {
	TenantID *string
	Status   *string
}

// OrderManager manages order operations.
type OrderManager struct {
	client *Client
}

// NewOrderManager creates a new OrderManager.
func NewOrderManager(client *Client) OrderManager {
	return OrderManager{client: client}
}

func orderFromStandard(api standard.Order) Order {
	o := Order{
		Parameters: api.Parameters,
		ServiceID:  api.ServiceId,
	}
	if api.Id != nil {
		o.ID = *api.Id
	}
	if api.BlueprintId != nil {
		o.BlueprintID = *api.BlueprintId
	}
	if api.BlueprintName != nil {
		o.BlueprintName = *api.BlueprintName
	}
	if api.TenantId != nil {
		o.TenantID = *api.TenantId
	}
	if api.Status != nil {
		o.Status = *api.Status
	}
	if api.StatusMessage != nil {
		o.StatusMessage = *api.StatusMessage
	}
	if api.WorkflowId != nil {
		o.WorkflowID = *api.WorkflowId
	}
	if api.Created != nil {
		o.Created = *api.Created
	}
	if api.Updated != nil {
		o.Updated = *api.Updated
	}
	return o
}

// Create creates a new order.
func (om OrderManager) Create(ctx context.Context, request OrderCreateRequest) (*Order, *ApiError) {
	ctx = WithLogger(ctx, om.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, om.client.Config.Token)

	apiReq := *standard.NewCreateOrderRequest(request.BlueprintID, request.TenantID)
	if request.BlueprintName != "" {
		apiReq.BlueprintName = &request.BlueprintName
	}
	apiReq.Parameters = request.Parameters

	apiOrder, resp, err := om.client.apiClient.FulfillmentAPI.CreateOrder(ctx, om.client.apiMetadata.Organization).
		CreateOrderRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	o := orderFromStandard(*apiOrder)
	return &o, nil
}

// GetOrders returns all orders with optional filtering and pagination.
func (om OrderManager) GetOrders(ctx context.Context, filter *OrderFilter, paginationFilter *OffsetPaginationFilter) ([]Order, *OffsetPaginationResponse, *ApiError) {
	ctx = WithLogger(ctx, om.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, om.client.Config.Token)

	req := om.client.apiClient.FulfillmentAPI.ListOrders(ctx, om.client.apiMetadata.Organization)
	if filter != nil {
		if filter.TenantID != nil {
			req = req.TenantId(*filter.TenantID)
		}
		if filter.Status != nil {
			req = req.Status(*filter.Status)
		}
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

	orders := make([]Order, 0, len(listResp.GetItems()))
	for _, api := range listResp.GetItems() {
		orders = append(orders, orderFromStandard(api))
	}

	pagination := &OffsetPaginationResponse{
		Total:  int(listResp.GetTotal()),
		Offset: int(listResp.GetOffset()),
		Limit:  int(listResp.GetLimit()),
	}
	return orders, pagination, nil
}

// GetOrder returns an order by ID.
func (om OrderManager) GetOrder(ctx context.Context, id string) (*Order, *ApiError) {
	ctx = WithLogger(ctx, om.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, om.client.Config.Token)

	apiOrder, resp, err := om.client.apiClient.FulfillmentAPI.GetOrder(ctx, om.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	o := orderFromStandard(*apiOrder)
	return &o, nil
}

// CancelOrder cancels an order by ID.
func (om OrderManager) CancelOrder(ctx context.Context, id string) *ApiError {
	ctx = WithLogger(ctx, om.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, om.client.Config.Token)

	resp, err := om.client.apiClient.FulfillmentAPI.CancelOrder(ctx, om.client.apiMetadata.Organization, id).Execute()
	return HandleResponseError(resp, err)
}
