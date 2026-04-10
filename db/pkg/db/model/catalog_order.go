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
	CatalogOrderStatusPending      = "Pending"
	CatalogOrderStatusProvisioning = "Provisioning"
	CatalogOrderStatusReady        = "Ready"
	CatalogOrderStatusFailed       = "Failed"
	CatalogOrderStatusCancelled    = "Cancelled"
)

// CatalogOrder represents a request to provision a service from a catalog blueprint.
type CatalogOrder struct {
	bun.BaseModel `bun:"table:catalog_order,alias:co"`

	ID            uuid.UUID              `bun:"id,type:uuid,pk"`
	BlueprintID   uuid.UUID              `bun:"blueprint_id,type:uuid,notnull"`
	BlueprintName string                 `bun:"blueprint_name,notnull"`
	TenantID      uuid.UUID              `bun:"tenant_id,type:uuid,notnull"`
	Parameters    map[string]interface{} `bun:"parameters,type:jsonb"`
	Status        string                 `bun:"status,notnull,default:'Pending'"`
	StatusMessage string                 `bun:"status_message"`
	WorkflowID    string                 `bun:"workflow_id"`
	ServiceID     *uuid.UUID             `bun:"service_id,type:uuid"`
	Created       time.Time              `bun:"created,nullzero,notnull,default:current_timestamp"`
	Updated       time.Time              `bun:"updated,nullzero,notnull,default:current_timestamp"`
	Deleted       *time.Time             `bun:"deleted,soft_delete"`

	Blueprint *Blueprint `bun:"rel:belongs-to,join:blueprint_id=id"`
}

var _ bun.BeforeAppendModelHook = (*CatalogOrder)(nil)

// BeforeAppendModel is a hook that is called before the model is appended to the query
func (o *CatalogOrder) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		o.Created = db.GetCurTime()
		o.Updated = db.GetCurTime()
	case *bun.UpdateQuery:
		o.Updated = db.GetCurTime()
	}
	return nil
}

// CatalogOrderDAO is an interface for interacting with the CatalogOrder model
type CatalogOrderDAO interface {
	//
	Create(ctx context.Context, tx *db.Tx, o *CatalogOrder) (*CatalogOrder, error)
	//
	GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*CatalogOrder, error)
	//
	GetAll(ctx context.Context, tx *db.Tx, tenantID *uuid.UUID, status *string) ([]CatalogOrder, error)
	//
	Update(ctx context.Context, tx *db.Tx, o *CatalogOrder) (*CatalogOrder, error)
	//
	DeleteByID(ctx context.Context, tx *db.Tx, id uuid.UUID) error
}

// CatalogOrderSQLDAO is an implementation of the CatalogOrderDAO interface
type CatalogOrderSQLDAO struct {
	dbSession *db.Session
	CatalogOrderDAO
	tracerSpan *stracer.TracerSpan
}

// Create inserts a new CatalogOrder
func (d CatalogOrderSQLDAO) Create(ctx context.Context, tx *db.Tx, o *CatalogOrder) (*CatalogOrder, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogOrderDAO.Create")
	if span != nil {
		defer span.End()
	}

	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}

	_, err := db.GetIDB(tx, d.dbSession).NewInsert().Model(o).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, o.ID)
}

// GetByID returns a CatalogOrder by ID
func (d CatalogOrderSQLDAO) GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*CatalogOrder, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogOrderDAO.GetByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "order_id", id.String())
	}

	o := &CatalogOrder{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(o).Where("co.id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}
	return o, nil
}

// GetAll returns all CatalogOrders matching optional filters
func (d CatalogOrderSQLDAO) GetAll(ctx context.Context, tx *db.Tx, tenantID *uuid.UUID, status *string) ([]CatalogOrder, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogOrderDAO.GetAll")
	if span != nil {
		defer span.End()
	}

	orders := []CatalogOrder{}
	query := db.GetIDB(tx, d.dbSession).NewSelect().Model(&orders)

	if tenantID != nil {
		query = query.Where("co.tenant_id = ?", *tenantID)
	}
	if status != nil {
		query = query.Where("co.status = ?", *status)
	}

	query = query.Order("co.created DESC")

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// Update updates an existing CatalogOrder
func (d CatalogOrderSQLDAO) Update(ctx context.Context, tx *db.Tx, o *CatalogOrder) (*CatalogOrder, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogOrderDAO.Update")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "order_id", o.ID.String())
	}

	_, err := db.GetIDB(tx, d.dbSession).NewUpdate().Model(o).WherePK().Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, o.ID)
}

// DeleteByID soft-deletes a CatalogOrder by ID
func (d CatalogOrderSQLDAO) DeleteByID(ctx context.Context, tx *db.Tx, id uuid.UUID) error {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "CatalogOrderDAO.DeleteByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "order_id", id.String())
	}

	o := &CatalogOrder{ID: id}
	_, err := db.GetIDB(tx, d.dbSession).NewDelete().Model(o).Where("id = ?", id).Exec(ctx)
	return err
}

// NewCatalogOrderDAO returns a new CatalogOrderDAO
func NewCatalogOrderDAO(dbSession *db.Session) CatalogOrderDAO {
	return &CatalogOrderSQLDAO{
		dbSession:  dbSession,
		tracerSpan: stracer.NewTracerSpan(),
	}
}
