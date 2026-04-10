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
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// OrderStore is a thread-safe in-memory store for orders.
type OrderStore struct {
	mu     sync.RWMutex
	orders map[uuid.UUID]*Order
}

// NewOrderStore creates a new in-memory order store.
func NewOrderStore() *OrderStore {
	return &OrderStore{
		orders: make(map[uuid.UUID]*Order),
	}
}

// Create adds a new order to the store.
func (s *OrderStore) Create(order *Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.orders[order.ID]; exists {
		return fmt.Errorf("order %s already exists", order.ID)
	}
	s.orders[order.ID] = order
	return nil
}

// Get retrieves an order by ID.
func (s *OrderStore) Get(id uuid.UUID) (*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order, ok := s.orders[id]
	if !ok {
		return nil, fmt.Errorf("order %s not found", id)
	}
	copy := *order
	return &copy, nil
}

// Update replaces an existing order in the store.
func (s *OrderStore) Update(order *Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.orders[order.ID]; !exists {
		return fmt.Errorf("order %s not found", order.ID)
	}
	s.orders[order.ID] = order
	return nil
}

// Delete removes an order by ID.
func (s *OrderStore) Delete(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.orders[id]; !exists {
		return fmt.Errorf("order %s not found", id)
	}
	delete(s.orders, id)
	return nil
}

// List returns all orders.
func (s *OrderStore) List() []*Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Order, 0, len(s.orders))
	for _, o := range s.orders {
		copy := *o
		result = append(result, &copy)
	}
	return result
}

// ListByTenant returns all orders for a given tenant.
func (s *OrderStore) ListByTenant(tenantID uuid.UUID) []*Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Order
	for _, o := range s.orders {
		if o.TenantID == tenantID {
			copy := *o
			result = append(result, &copy)
		}
	}
	return result
}

// ServiceStore is a thread-safe in-memory store for services.
type ServiceStore struct {
	mu       sync.RWMutex
	services map[uuid.UUID]*Service
}

// NewServiceStore creates a new in-memory service store.
func NewServiceStore() *ServiceStore {
	return &ServiceStore{
		services: make(map[uuid.UUID]*Service),
	}
}

// Create adds a new service to the store.
func (s *ServiceStore) Create(svc *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.services[svc.ID]; exists {
		return fmt.Errorf("service %s already exists", svc.ID)
	}
	s.services[svc.ID] = svc
	return nil
}

// Get retrieves a service by ID.
func (s *ServiceStore) Get(id uuid.UUID) (*Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	svc, ok := s.services[id]
	if !ok {
		return nil, fmt.Errorf("service %s not found", id)
	}
	copy := *svc
	return &copy, nil
}

// Update replaces an existing service in the store.
func (s *ServiceStore) Update(svc *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.services[svc.ID]; !exists {
		return fmt.Errorf("service %s not found", svc.ID)
	}
	s.services[svc.ID] = svc
	return nil
}

// Delete removes a service by ID.
func (s *ServiceStore) Delete(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.services[id]; !exists {
		return fmt.Errorf("service %s not found", id)
	}
	delete(s.services, id)
	return nil
}

// List returns all services.
func (s *ServiceStore) List() []*Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Service, 0, len(s.services))
	for _, svc := range s.services {
		copy := *svc
		result = append(result, &copy)
	}
	return result
}

// ListByTenant returns all services for a given tenant.
func (s *ServiceStore) ListByTenant(tenantID uuid.UUID) []*Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Service
	for _, svc := range s.services {
		if svc.TenantID == tenantID {
			copy := *svc
			result = append(result, &copy)
		}
	}
	return result
}
