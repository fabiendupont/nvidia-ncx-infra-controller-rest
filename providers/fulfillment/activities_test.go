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
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newActivities() (*FulfillmentActivities, *OrderStore, *ServiceStore) {
	os := NewOrderStore()
	ss := NewServiceStore()
	return &FulfillmentActivities{
		orderStore:   os,
		serviceStore: ss,
	}, os, ss
}

func TestValidateOrder_Success(t *testing.T) {
	a, os, _ := newActivities()
	order := newTestOrder()
	require.NoError(t, os.Create(order))

	got, err := a.ValidateOrder(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
	assert.Equal(t, order.TemplateName, got.TemplateName)
}

func TestValidateOrder_NotFound(t *testing.T) {
	a, _, _ := newActivities()

	_, err := a.ValidateOrder(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve order")
}

func TestUpdateOrderStatus_Success(t *testing.T) {
	a, os, _ := newActivities()
	order := newTestOrder()
	require.NoError(t, os.Create(order))

	err := a.UpdateOrderStatus(context.Background(), order.ID, OrderStatusProvisioning, "provisioning started")
	require.NoError(t, err)

	got, err := os.Get(order.ID)
	require.NoError(t, err)
	assert.Equal(t, OrderStatusProvisioning, got.Status)
	assert.Equal(t, "provisioning started", got.StatusMessage)
}

func TestUpdateOrderStatus_NotFound(t *testing.T) {
	a, _, _ := newActivities()

	err := a.UpdateOrderStatus(context.Background(), uuid.New(), OrderStatusProvisioning, "msg")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve order")
}

func TestCreateService_Success(t *testing.T) {
	a, os, ss := newActivities()
	order := newTestOrder()
	require.NoError(t, os.Create(order))

	svc, err := a.CreateService(context.Background(), order)
	require.NoError(t, err)
	assert.Equal(t, ServiceStatusProvisioning, svc.Status)
	assert.Equal(t, order.ID, svc.OrderID)
	assert.Equal(t, order.TenantID, svc.TenantID)

	// Service should exist in store.
	got, err := ss.Get(svc.ID)
	require.NoError(t, err)
	assert.Equal(t, svc.ID, got.ID)

	// Order should be linked to the service.
	updatedOrder, err := os.Get(order.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedOrder.ServiceID)
	assert.Equal(t, svc.ID, *updatedOrder.ServiceID)
}

func TestMarkServiceActive_Success(t *testing.T) {
	a, _, ss := newActivities()
	svc := newTestService(uuid.New())
	require.NoError(t, ss.Create(svc))

	err := a.MarkServiceActive(context.Background(), svc.ID)
	require.NoError(t, err)

	got, err := ss.Get(svc.ID)
	require.NoError(t, err)
	assert.Equal(t, ServiceStatusActive, got.Status)
}

func TestMarkServiceActive_NotFound(t *testing.T) {
	a, _, _ := newActivities()

	err := a.MarkServiceActive(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve service")
}

func TestMarkServiceTerminated_Success(t *testing.T) {
	a, _, ss := newActivities()
	svc := newTestService(uuid.New())
	svc.Status = ServiceStatusActive
	require.NoError(t, ss.Create(svc))

	err := a.MarkServiceTerminated(context.Background(), svc.ID)
	require.NoError(t, err)

	got, err := ss.Get(svc.ID)
	require.NoError(t, err)
	assert.Equal(t, ServiceStatusTerminated, got.Status)
}

func TestMarkServiceTerminated_NotFound(t *testing.T) {
	a, _, _ := newActivities()

	err := a.MarkServiceTerminated(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve service")
}
