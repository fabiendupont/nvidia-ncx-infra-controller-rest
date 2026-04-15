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

package fulfillment

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus represents the lifecycle state of a service order.
type OrderStatus string

const (
	OrderStatusPending      OrderStatus = "Pending"
	OrderStatusProvisioning OrderStatus = "Provisioning"
	OrderStatusReady        OrderStatus = "Ready"
	OrderStatusFailed       OrderStatus = "Failed"
	OrderStatusCancelled    OrderStatus = "Cancelled"
)

// Order represents a request to provision a service from a catalog blueprint.
type Order struct {
	ID            uuid.UUID              `json:"id"`
	BlueprintID   uuid.UUID              `json:"blueprint_id"`
	BlueprintName string                 `json:"blueprint_name"`
	TenantID      uuid.UUID              `json:"tenant_id"`
	Parameters    map[string]interface{} `json:"parameters"`
	Status        OrderStatus            `json:"status"`
	StatusMessage string                 `json:"status_message,omitempty"`
	WorkflowID    string                 `json:"workflow_id,omitempty"`
	ServiceID     *uuid.UUID             `json:"service_id,omitempty"`
	Created       time.Time              `json:"created"`
	Updated       time.Time              `json:"updated"`
}

// ServiceStatus represents the lifecycle state of a provisioned service.
type ServiceStatus string

const (
	ServiceStatusProvisioning ServiceStatus = "Provisioning"
	ServiceStatusActive       ServiceStatus = "Active"
	ServiceStatusUpdating     ServiceStatus = "Updating"
	ServiceStatusTerminating  ServiceStatus = "Terminating"
	ServiceStatusTerminated   ServiceStatus = "Terminated"
)

// Service represents a provisioned tenant environment with its associated resources.
type Service struct {
	ID            uuid.UUID         `json:"id"`
	OrderID       uuid.UUID         `json:"order_id"`
	BlueprintID   uuid.UUID         `json:"blueprint_id"`
	BlueprintName string            `json:"blueprint_name"`
	TenantID      uuid.UUID         `json:"tenant_id"`
	Name          string            `json:"name"`
	Status        ServiceStatus     `json:"status"`
	Resources     map[string]string `json:"resources,omitempty"`
	Created       time.Time         `json:"created"`
	Updated       time.Time         `json:"updated"`
}
