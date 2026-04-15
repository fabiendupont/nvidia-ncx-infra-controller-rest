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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOrder() *Order {
	return &Order{
		ID:            uuid.New(),
		BlueprintID:   uuid.New(),
		BlueprintName: "test-template",
		TenantID:      uuid.New(),
		Parameters:    map[string]interface{}{"key": "value"},
		Status:        OrderStatusPending,
		Created:       time.Now().UTC(),
		Updated:       time.Now().UTC(),
	}
}

func newTestService(tenantID uuid.UUID) *Service {
	return &Service{
		ID:            uuid.New(),
		OrderID:       uuid.New(),
		BlueprintID:   uuid.New(),
		BlueprintName: "test-template",
		TenantID:      tenantID,
		Name:          "test-service",
		Status:        ServiceStatusProvisioning,
		Created:       time.Now().UTC(),
		Updated:       time.Now().UTC(),
	}
}

// --- OrderStore tests ---

func TestOrderStore_Create(t *testing.T) {
	store := NewOrderStore()
	order := newTestOrder()

	err := store.Create(order)
	require.NoError(t, err)

	// Creating the same order again should fail.
	err = store.Create(order)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestOrderStore_Get(t *testing.T) {
	store := NewOrderStore()
	order := newTestOrder()
	require.NoError(t, store.Create(order))

	got, err := store.Get(order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
	assert.Equal(t, order.BlueprintName, got.BlueprintName)

	// Get a non-existent order.
	_, err = store.Get(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrderStore_Update(t *testing.T) {
	store := NewOrderStore()
	order := newTestOrder()
	require.NoError(t, store.Create(order))

	order.Status = OrderStatusProvisioning
	err := store.Update(order)
	require.NoError(t, err)

	got, err := store.Get(order.ID)
	require.NoError(t, err)
	assert.Equal(t, OrderStatusProvisioning, got.Status)

	// Update a non-existent order.
	missing := newTestOrder()
	err = store.Update(missing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrderStore_Delete(t *testing.T) {
	store := NewOrderStore()
	order := newTestOrder()
	require.NoError(t, store.Create(order))

	err := store.Delete(order.ID)
	require.NoError(t, err)

	_, err = store.Get(order.ID)
	assert.Error(t, err)

	// Delete a non-existent order.
	err = store.Delete(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrderStore_List(t *testing.T) {
	store := NewOrderStore()

	// Empty store.
	orders := store.List()
	assert.Empty(t, orders)

	o1 := newTestOrder()
	o2 := newTestOrder()
	require.NoError(t, store.Create(o1))
	require.NoError(t, store.Create(o2))

	orders = store.List()
	assert.Len(t, orders, 2)
}

// --- ServiceStore tests ---

func TestServiceStore_Create(t *testing.T) {
	store := NewServiceStore()
	svc := newTestService(uuid.New())

	err := store.Create(svc)
	require.NoError(t, err)

	// Duplicate should fail.
	err = store.Create(svc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestServiceStore_Get(t *testing.T) {
	store := NewServiceStore()
	svc := newTestService(uuid.New())
	require.NoError(t, store.Create(svc))

	got, err := store.Get(svc.ID)
	require.NoError(t, err)
	assert.Equal(t, svc.ID, got.ID)

	// Not found.
	_, err = store.Get(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceStore_Update(t *testing.T) {
	store := NewServiceStore()
	svc := newTestService(uuid.New())
	require.NoError(t, store.Create(svc))

	svc.Status = ServiceStatusActive
	err := store.Update(svc)
	require.NoError(t, err)

	got, err := store.Get(svc.ID)
	require.NoError(t, err)
	assert.Equal(t, ServiceStatusActive, got.Status)

	// Update non-existent.
	missing := newTestService(uuid.New())
	err = store.Update(missing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceStore_Delete(t *testing.T) {
	store := NewServiceStore()
	svc := newTestService(uuid.New())
	require.NoError(t, store.Create(svc))

	err := store.Delete(svc.ID)
	require.NoError(t, err)

	_, err = store.Get(svc.ID)
	assert.Error(t, err)

	// Delete non-existent.
	err = store.Delete(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceStore_List(t *testing.T) {
	store := NewServiceStore()

	assert.Empty(t, store.List())

	tenantID := uuid.New()
	s1 := newTestService(tenantID)
	s2 := newTestService(uuid.New())
	require.NoError(t, store.Create(s1))
	require.NoError(t, store.Create(s2))

	services := store.List()
	assert.Len(t, services, 2)
}

func TestServiceStore_ListByTenant(t *testing.T) {
	store := NewServiceStore()
	tenantA := uuid.New()
	tenantB := uuid.New()

	s1 := newTestService(tenantA)
	s2 := newTestService(tenantA)
	s3 := newTestService(tenantB)
	require.NoError(t, store.Create(s1))
	require.NoError(t, store.Create(s2))
	require.NoError(t, store.Create(s3))

	byA := store.ListByTenant(tenantA)
	assert.Len(t, byA, 2)

	byB := store.ListByTenant(tenantB)
	assert.Len(t, byB, 1)

	// Tenant with no services.
	byNone := store.ListByTenant(uuid.New())
	assert.Empty(t, byNone)
}
