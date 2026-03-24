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

package computesvc

import (
	"context"

	"github.com/google/uuid"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
)

// Service defines the operations that other providers can use to interact
// with compute entities. This is the cross-domain API contract.
type Service interface {
	GetInstanceByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.Instance, error)
	GetInstances(ctx context.Context, tx *db.Tx, filter cdbm.InstanceFilterInput, page paginator.PageInput) ([]cdbm.Instance, int, error)
	GetAllocationsCount(ctx context.Context, tx *db.Tx, filter cdbm.AllocationFilterInput) (int, error)
	GetMachineByID(ctx context.Context, tx *db.Tx, id string) (*cdbm.Machine, error)
}

// SQLService implements the Service interface using the existing SQL DAOs.
type SQLService struct {
	dbSession *db.Session
}

// New creates a new SQLService.
func New(dbSession *db.Session) *SQLService {
	return &SQLService{dbSession: dbSession}
}

func (s *SQLService) GetInstanceByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.Instance, error) {
	dao := cdbm.NewInstanceDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil)
}

func (s *SQLService) GetInstances(ctx context.Context, tx *db.Tx, filter cdbm.InstanceFilterInput, page paginator.PageInput) ([]cdbm.Instance, int, error) {
	dao := cdbm.NewInstanceDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) GetAllocationsCount(ctx context.Context, tx *db.Tx, filter cdbm.AllocationFilterInput) (int, error) {
	dao := cdbm.NewAllocationDAO(s.dbSession)
	return dao.GetCount(ctx, tx, filter)
}

func (s *SQLService) GetMachineByID(ctx context.Context, tx *db.Tx, id string) (*cdbm.Machine, error) {
	dao := cdbm.NewMachineDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil, false)
}
