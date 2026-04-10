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
	"fmt"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
)

// OrderStoreInterface defines the contract for order storage.
type OrderStoreInterface interface {
	Create(order *Order) error
	Get(id uuid.UUID) (*Order, error)
	Update(order *Order) error
	Delete(id uuid.UUID) error
	List() []*Order
	ListByTenant(tenantID uuid.UUID) []*Order
}

// ServiceStoreInterface defines the contract for service storage.
type ServiceStoreInterface interface {
	Create(svc *Service) error
	Get(id uuid.UUID) (*Service, error)
	Update(svc *Service) error
	Delete(id uuid.UUID) error
	List() []*Service
	ListByTenant(tenantID uuid.UUID) []*Service
}

// OrderSQLStore is a PostgreSQL-backed order store.
type OrderSQLStore struct {
	dao model.CatalogOrderDAO
}

// NewOrderSQLStore creates a new SQL-backed order store.
func NewOrderSQLStore(dbSession *cdb.Session) *OrderSQLStore {
	return &OrderSQLStore{dao: model.NewCatalogOrderDAO(dbSession)}
}

// Create adds a new order to the database.
func (s *OrderSQLStore) Create(order *Order) error {
	dbModel := orderToDBModel(order)
	created, err := s.dao.Create(context.Background(), nil, dbModel)
	if err != nil {
		return err
	}
	order.ID = created.ID
	order.Created = created.Created
	order.Updated = created.Updated
	return nil
}

// Get retrieves an order by ID.
func (s *OrderSQLStore) Get(id uuid.UUID) (*Order, error) {
	dbModel, err := s.dao.GetByID(context.Background(), nil, id)
	if err != nil {
		return nil, fmt.Errorf("order %s not found", id)
	}
	return dbModelToOrder(dbModel), nil
}

// Update replaces an existing order.
func (s *OrderSQLStore) Update(order *Order) error {
	dbModel := orderToDBModel(order)
	updated, err := s.dao.Update(context.Background(), nil, dbModel)
	if err != nil {
		return err
	}
	order.Updated = updated.Updated
	return nil
}

// Delete soft-deletes an order.
func (s *OrderSQLStore) Delete(id uuid.UUID) error {
	return s.dao.DeleteByID(context.Background(), nil, id)
}

// List returns all orders.
func (s *OrderSQLStore) List() []*Order {
	dbModels, err := s.dao.GetAll(context.Background(), nil, nil, nil)
	if err != nil {
		return nil
	}
	result := make([]*Order, 0, len(dbModels))
	for i := range dbModels {
		result = append(result, dbModelToOrder(&dbModels[i]))
	}
	return result
}

// ListByTenant returns all orders for a given tenant.
func (s *OrderSQLStore) ListByTenant(tenantID uuid.UUID) []*Order {
	dbModels, err := s.dao.GetAll(context.Background(), nil, &tenantID, nil)
	if err != nil {
		return nil
	}
	result := make([]*Order, 0, len(dbModels))
	for i := range dbModels {
		result = append(result, dbModelToOrder(&dbModels[i]))
	}
	return result
}

// ServiceSQLStore is a PostgreSQL-backed service store.
type ServiceSQLStore struct {
	dao model.CatalogServiceDAO
}

// NewServiceSQLStore creates a new SQL-backed service store.
func NewServiceSQLStore(dbSession *cdb.Session) *ServiceSQLStore {
	return &ServiceSQLStore{dao: model.NewCatalogServiceDAO(dbSession)}
}

// Create adds a new service to the database.
func (s *ServiceSQLStore) Create(svc *Service) error {
	dbModel := serviceToDBModel(svc)
	created, err := s.dao.Create(context.Background(), nil, dbModel)
	if err != nil {
		return err
	}
	svc.ID = created.ID
	svc.Created = created.Created
	svc.Updated = created.Updated
	return nil
}

// Get retrieves a service by ID.
func (s *ServiceSQLStore) Get(id uuid.UUID) (*Service, error) {
	dbModel, err := s.dao.GetByID(context.Background(), nil, id)
	if err != nil {
		return nil, fmt.Errorf("service %s not found", id)
	}
	return dbModelToService(dbModel), nil
}

// Update replaces an existing service.
func (s *ServiceSQLStore) Update(svc *Service) error {
	dbModel := serviceToDBModel(svc)
	updated, err := s.dao.Update(context.Background(), nil, dbModel)
	if err != nil {
		return err
	}
	svc.Updated = updated.Updated
	return nil
}

// Delete soft-deletes a service.
func (s *ServiceSQLStore) Delete(id uuid.UUID) error {
	return s.dao.DeleteByID(context.Background(), nil, id)
}

// List returns all services.
func (s *ServiceSQLStore) List() []*Service {
	dbModels, err := s.dao.GetAll(context.Background(), nil, nil, nil)
	if err != nil {
		return nil
	}
	result := make([]*Service, 0, len(dbModels))
	for i := range dbModels {
		result = append(result, dbModelToService(&dbModels[i]))
	}
	return result
}

// ListByTenant returns all services for a given tenant.
func (s *ServiceSQLStore) ListByTenant(tenantID uuid.UUID) []*Service {
	dbModels, err := s.dao.GetAll(context.Background(), nil, &tenantID, nil)
	if err != nil {
		return nil
	}
	result := make([]*Service, 0, len(dbModels))
	for i := range dbModels {
		result = append(result, dbModelToService(&dbModels[i]))
	}
	return result
}

// --- Conversion helpers ---

func orderToDBModel(o *Order) *model.CatalogOrder {
	return &model.CatalogOrder{
		ID:            o.ID,
		BlueprintID:   o.TemplateID,
		BlueprintName: o.TemplateName,
		TenantID:      o.TenantID,
		Parameters:    o.Parameters,
		Status:        string(o.Status),
		StatusMessage: o.StatusMessage,
		WorkflowID:    o.WorkflowID,
		ServiceID:     o.ServiceID,
		Created:       o.Created,
		Updated:       o.Updated,
	}
}

func dbModelToOrder(m *model.CatalogOrder) *Order {
	return &Order{
		ID:            m.ID,
		TemplateID:    m.BlueprintID,
		TemplateName:  m.BlueprintName,
		TenantID:      m.TenantID,
		Parameters:    m.Parameters,
		Status:        OrderStatus(m.Status),
		StatusMessage: m.StatusMessage,
		WorkflowID:    m.WorkflowID,
		ServiceID:     m.ServiceID,
		Created:       m.Created,
		Updated:       m.Updated,
	}
}

func serviceToDBModel(s *Service) *model.CatalogService {
	return &model.CatalogService{
		ID:            s.ID,
		OrderID:       s.OrderID,
		BlueprintID:   s.TemplateID,
		BlueprintName: s.TemplateName,
		TenantID:      s.TenantID,
		Name:          s.Name,
		Status:        string(s.Status),
		Resources:     s.Resources,
		Created:       s.Created,
		Updated:       s.Updated,
	}
}

func dbModelToService(m *model.CatalogService) *Service {
	return &Service{
		ID:           m.ID,
		OrderID:      m.OrderID,
		TemplateID:   m.BlueprintID,
		TemplateName: m.BlueprintName,
		TenantID:     m.TenantID,
		Name:         m.Name,
		Status:       ServiceStatus(m.Status),
		Resources:    m.Resources,
		Created:      m.Created,
		Updated:      m.Updated,
	}
}
