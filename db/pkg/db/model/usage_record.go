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

// UsageRecord tracks a single metered usage period for a resource.
type UsageRecord struct {
	bun.BaseModel `bun:"table:usage_record,alias:ur"`

	ID         uuid.UUID  `bun:"id,type:uuid,pk"`
	TenantID   uuid.UUID  `bun:"tenant_id,type:uuid,notnull"`
	ServiceID  uuid.UUID  `bun:"service_id,type:uuid"`
	ResourceID uuid.UUID  `bun:"resource_id,type:uuid,notnull"`
	MetricName string     `bun:"metric_name,notnull"`
	Value      float64    `bun:"value,notnull,default:0"`
	StartTime  time.Time  `bun:"start_time,notnull,default:current_timestamp"`
	EndTime    *time.Time `bun:"end_time"`
	Created    time.Time  `bun:"created,nullzero,notnull,default:current_timestamp"`
	Updated    time.Time  `bun:"updated,nullzero,notnull,default:current_timestamp"`
}

var _ bun.BeforeAppendModelHook = (*UsageRecord)(nil)

// BeforeAppendModel is a hook that is called before the model is appended to the query
func (u *UsageRecord) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		u.Created = db.GetCurTime()
		u.Updated = db.GetCurTime()
	case *bun.UpdateQuery:
		u.Updated = db.GetCurTime()
	}
	return nil
}

// UsageRecordDAO is an interface for interacting with the UsageRecord model
type UsageRecordDAO interface {
	//
	Create(ctx context.Context, tx *db.Tx, r *UsageRecord) (*UsageRecord, error)
	//
	GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*UsageRecord, error)
	//
	GetByResourceID(ctx context.Context, tx *db.Tx, resourceID uuid.UUID) (*UsageRecord, error)
	//
	GetAllByTenant(ctx context.Context, tx *db.Tx, tenantID uuid.UUID) ([]UsageRecord, error)
	//
	GetAllByService(ctx context.Context, tx *db.Tx, serviceID uuid.UUID) ([]UsageRecord, error)
	//
	Update(ctx context.Context, tx *db.Tx, r *UsageRecord) (*UsageRecord, error)
}

// UsageRecordSQLDAO is an implementation of the UsageRecordDAO interface
type UsageRecordSQLDAO struct {
	dbSession *db.Session
	UsageRecordDAO
	tracerSpan *stracer.TracerSpan
}

// Create inserts a new UsageRecord
func (d UsageRecordSQLDAO) Create(ctx context.Context, tx *db.Tx, r *UsageRecord) (*UsageRecord, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "UsageRecordDAO.Create")
	if span != nil {
		defer span.End()
	}

	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}

	_, err := db.GetIDB(tx, d.dbSession).NewInsert().Model(r).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, r.ID)
}

// GetByID returns a UsageRecord by ID
func (d UsageRecordSQLDAO) GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*UsageRecord, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "UsageRecordDAO.GetByID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "record_id", id.String())
	}

	r := &UsageRecord{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(r).Where("ur.id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}
	return r, nil
}

// GetByResourceID returns the active (open-ended) UsageRecord for a resource
func (d UsageRecordSQLDAO) GetByResourceID(ctx context.Context, tx *db.Tx, resourceID uuid.UUID) (*UsageRecord, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "UsageRecordDAO.GetByResourceID")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "resource_id", resourceID.String())
	}

	r := &UsageRecord{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(r).
		Where("ur.resource_id = ?", resourceID).
		Where("ur.end_time IS NULL").
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}
	return r, nil
}

// GetAllByTenant returns all UsageRecords for a tenant
func (d UsageRecordSQLDAO) GetAllByTenant(ctx context.Context, tx *db.Tx, tenantID uuid.UUID) ([]UsageRecord, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "UsageRecordDAO.GetAllByTenant")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "tenant_id", tenantID.String())
	}

	records := []UsageRecord{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(&records).
		Where("ur.tenant_id = ?", tenantID).
		Order("ur.start_time DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetAllByService returns all UsageRecords for a service
func (d UsageRecordSQLDAO) GetAllByService(ctx context.Context, tx *db.Tx, serviceID uuid.UUID) ([]UsageRecord, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "UsageRecordDAO.GetAllByService")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "service_id", serviceID.String())
	}

	records := []UsageRecord{}
	err := db.GetIDB(tx, d.dbSession).NewSelect().Model(&records).
		Where("ur.service_id = ?", serviceID).
		Order("ur.start_time DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// Update updates an existing UsageRecord
func (d UsageRecordSQLDAO) Update(ctx context.Context, tx *db.Tx, r *UsageRecord) (*UsageRecord, error) {
	ctx, span := d.tracerSpan.CreateChildInCurrentContext(ctx, "UsageRecordDAO.Update")
	if span != nil {
		defer span.End()
		d.tracerSpan.SetAttribute(span, "record_id", r.ID.String())
	}

	_, err := db.GetIDB(tx, d.dbSession).NewUpdate().Model(r).WherePK().Exec(ctx)
	if err != nil {
		return nil, err
	}

	return d.GetByID(ctx, tx, r.ID)
}

// NewUsageRecordDAO returns a new UsageRecordDAO
func NewUsageRecordDAO(dbSession *db.Session) UsageRecordDAO {
	return &UsageRecordSQLDAO{
		dbSession:  dbSession,
		tracerSpan: stracer.NewTracerSpan(),
	}
}
