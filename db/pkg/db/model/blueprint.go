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

// Blueprint is a composable, DAG-based service definition stored in the catalog.
type Blueprint struct {
	bun.BaseModel `bun:"table:blueprint,alias:bp"`

	ID          uuid.UUID              `bun:"id,type:uuid,pk"`
	Name        string                 `bun:"name,notnull"`
	Version     string                 `bun:"version,notnull"`
	Description string                 `bun:"description"`
	Parameters  map[string]interface{} `bun:"parameters,type:jsonb"`
	Resources   map[string]interface{} `bun:"resources,type:jsonb"`
	Labels      map[string]string      `bun:"labels,type:jsonb"`
	Pricing     map[string]interface{} `bun:"pricing,type:jsonb"`
	TenantID    *uuid.UUID             `bun:"tenant_id,type:uuid"`
	Visibility  string                 `bun:"visibility,notnull,default:'public'"`
	BasedOn     *string                `bun:"based_on"`
	IsActive    bool                   `bun:"is_active,notnull,default:true"`
	Created     time.Time              `bun:"created,nullzero,notnull,default:current_timestamp"`
	Updated     time.Time              `bun:"updated,nullzero,notnull,default:current_timestamp"`
	Deleted     *time.Time             `bun:"deleted,soft_delete"`
}

var _ bun.BeforeAppendModelHook = (*Blueprint)(nil)

// BeforeAppendModel is a hook that is called before the model is appended to the query
func (b *Blueprint) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		b.Created = db.GetCurTime()
		b.Updated = db.GetCurTime()
	case *bun.UpdateQuery:
		b.Updated = db.GetCurTime()
	}
	return nil
}

// BlueprintDAO is an interface for interacting with the Blueprint model
type BlueprintDAO interface {
	//
	Create(ctx context.Context, tx *db.Tx, b *Blueprint) (*Blueprint, error)
	//
	GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*Blueprint, error)
	//
	GetAll(ctx context.Context, tx *db.Tx, tenantID *uuid.UUID, visibility *string, isActive *bool) ([]Blueprint, error)
	//
	Update(ctx context.Context, tx *db.Tx, b *Blueprint) (*Blueprint, error)
	//
	DeleteByID(ctx context.Context, tx *db.Tx, id uuid.UUID) error
}

// BlueprintSQLDAO is an implementation of the BlueprintDAO interface
type BlueprintSQLDAO struct {
	dbSession *db.Session
	BlueprintDAO
	tracerSpan *stracer.TracerSpan
}

// Create inserts a new Blueprint
func (d BlueprintSQLDAO) Create(ctx context.Context, tx *db.Tx, b *Blueprint) (*Blueprint, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "BlueprintDAO.Create")
	if span != nil {
		defer span.End()
	}

	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}

	_, err := db.GetIDB(tx, d.dbSession).NewInsert().Model(b).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, b.ID)
}

// GetByID returns a Blueprint by ID
func (d BlueprintSQLDAO) GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*Blueprint, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "BlueprintDAO.GetByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "blueprint_id", id.String())
	}

	b := &Blueprint{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(b).Where("bp.id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}
	return b, nil
}

// GetAll returns all Blueprints matching optional filters
func (d BlueprintSQLDAO) GetAll(ctx context.Context, tx *db.Tx, tenantID *uuid.UUID, visibility *string, isActive *bool) ([]Blueprint, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "BlueprintDAO.GetAll")
	if span != nil {
		defer span.End()
	}

	blueprints := []Blueprint{}
	query := db.GetIDB(tx, d.dbSession).NewSelect().Model(&blueprints)

	if tenantID != nil {
		// Return provider-published (tenant_id IS NULL) and tenant-owned blueprints
		query = query.Where("bp.tenant_id IS NULL OR bp.tenant_id = ?", *tenantID)
	}
	if visibility != nil {
		query = query.Where("bp.visibility = ?", *visibility)
	}
	if isActive != nil {
		query = query.Where("bp.is_active = ?", *isActive)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return blueprints, nil
}

// Update updates an existing Blueprint
func (d BlueprintSQLDAO) Update(ctx context.Context, tx *db.Tx, b *Blueprint) (*Blueprint, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "BlueprintDAO.Update")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "blueprint_id", b.ID.String())
	}

	_, err := db.GetIDB(tx, d.dbSession).NewUpdate().Model(b).WherePK().Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, b.ID)
}

// DeleteByID soft-deletes a Blueprint by ID
func (d BlueprintSQLDAO) DeleteByID(ctx context.Context, tx *db.Tx, id uuid.UUID) error {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "BlueprintDAO.DeleteByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "blueprint_id", id.String())
	}

	b := &Blueprint{ID: id}
	_, err := db.GetIDB(tx, d.dbSession).NewDelete().Model(b).Where("id = ?", id).Exec(ctx)
	return err
}

// NewBlueprintDAO returns a new BlueprintDAO
func NewBlueprintDAO(dbSession *db.Session) BlueprintDAO {
	return &BlueprintSQLDAO{
		dbSession:  dbSession,
		tracerSpan: stracer.NewTracerSpan(),
	}
}
