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

package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/google/uuid"

	stracer "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/tracer"
	"github.com/uptrace/bun"
)

const (
	CatalogServiceStatusProvisioning = "Provisioning"
	CatalogServiceStatusActive       = "Active"
	CatalogServiceStatusUpdating     = "Updating"
	CatalogServiceStatusTerminating  = "Terminating"
	CatalogServiceStatusTerminated   = "Terminated"
)

// CatalogService represents a provisioned tenant environment with its associated resources.
type CatalogService struct {
	bun.BaseModel `bun:"table:catalog_service,alias:cs"`

	ID            uuid.UUID         `bun:"id,type:uuid,pk"`
	OrderID       uuid.UUID         `bun:"order_id,type:uuid,notnull"`
	BlueprintID   uuid.UUID         `bun:"blueprint_id,type:uuid,notnull"`
	BlueprintName string            `bun:"blueprint_name,notnull"`
	TenantID      uuid.UUID         `bun:"tenant_id,type:uuid,notnull"`
	Name          string            `bun:"name,notnull"`
	Status        string            `bun:"status,notnull,default:'Provisioning'"`
	Resources     map[string]string `bun:"resources,type:jsonb"`
	Created       time.Time         `bun:"created,nullzero,notnull,default:current_timestamp"`
	Updated       time.Time         `bun:"updated,nullzero,notnull,default:current_timestamp"`
	Deleted       *time.Time        `bun:"deleted,soft_delete"`

	Order     *CatalogOrder `bun:"rel:belongs-to,join:order_id=id"`
	Blueprint *Blueprint    `bun:"rel:belongs-to,join:blueprint_id=id"`
}

var _ bun.BeforeAppendModelHook = (*CatalogService)(nil)

// BeforeAppendModel is a hook that is called before the model is appended to the query
func (s *CatalogService) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		s.Created = db.GetCurTime()
		s.Updated = db.GetCurTime()
	case *bun.UpdateQuery:
		s.Updated = db.GetCurTime()
	}
	return nil
}

// CatalogServiceDAO is an interface for interacting with the CatalogService model
type CatalogServiceDAO interface {
	//
	Create(ctx context.Context, tx *db.Tx, s *CatalogService) (*CatalogService, error)
	//
	GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*CatalogService, error)
	//
	GetAll(ctx context.Context, tx *db.Tx, tenantID *uuid.UUID, status *string) ([]CatalogService, error)
	//
	Update(ctx context.Context, tx *db.Tx, s *CatalogService) (*CatalogService, error)
	//
	DeleteByID(ctx context.Context, tx *db.Tx, id uuid.UUID) error
}

// CatalogServiceSQLDAO is an implementation of the CatalogServiceDAO interface
type CatalogServiceSQLDAO struct {
	dbSession *db.Session
	CatalogServiceDAO
	tracerSpan *stracer.TracerSpan
}

// Create inserts a new CatalogService
func (d CatalogServiceSQLDAO) Create(ctx context.Context, tx *db.Tx, s *CatalogService) (*CatalogService, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogServiceDAO.Create")
	if span != nil {
		defer span.End()
	}

	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	_, err := db.GetIDB(tx, d.dbSession).NewInsert().Model(s).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, s.ID)
}

// GetByID returns a CatalogService by ID
func (d CatalogServiceSQLDAO) GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*CatalogService, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogServiceDAO.GetByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "service_id", id.String())
	}

	s := &CatalogService{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(s).Where("cs.id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}
	return s, nil
}

// GetAll returns all CatalogServices matching optional filters
func (d CatalogServiceSQLDAO) GetAll(ctx context.Context, tx *db.Tx, tenantID *uuid.UUID, status *string) ([]CatalogService, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogServiceDAO.GetAll")
	if span != nil {
		defer span.End()
	}

	services := []CatalogService{}
	query := db.GetIDB(tx, d.dbSession).NewSelect().Model(&services)

	if tenantID != nil {
		query = query.Where("cs.tenant_id = ?", *tenantID)
	}
	if status != nil {
		query = query.Where("cs.status = ?", *status)
	}

	query = query.Order("cs.created DESC")

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return services, nil
}

// Update updates an existing CatalogService
func (d CatalogServiceSQLDAO) Update(ctx context.Context, tx *db.Tx, s *CatalogService) (*CatalogService, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogServiceDAO.Update")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "service_id", s.ID.String())
	}

	_, err := db.GetIDB(tx, d.dbSession).NewUpdate().Model(s).WherePK().Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, s.ID)
}

// DeleteByID soft-deletes a CatalogService by ID
func (d CatalogServiceSQLDAO) DeleteByID(ctx context.Context, tx *db.Tx, id uuid.UUID) error {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogServiceDAO.DeleteByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "service_id", id.String())
	}

	s := &CatalogService{ID: id}
	_, err := db.GetIDB(tx, d.dbSession).NewDelete().Model(s).Where("id = ?", id).Exec(ctx)
	return err
}

// NewCatalogServiceDAO returns a new CatalogServiceDAO
func NewCatalogServiceDAO(dbSession *db.Session) CatalogServiceDAO {
	return &CatalogServiceSQLDAO{
		dbSession:  dbSession,
		tracerSpan: stracer.NewTracerSpan(),
	}
}
