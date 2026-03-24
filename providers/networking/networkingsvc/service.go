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

package networkingsvc

import (
	"context"

	"github.com/google/uuid"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
)

// Service defines the operations that other providers can use to interact
// with networking entities. This is the cross-domain API contract.
type Service interface {
	GetVpcByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.Vpc, error)
	GetVpcs(ctx context.Context, tx *db.Tx, filter cdbm.VpcFilterInput, page paginator.PageInput) ([]cdbm.Vpc, int, error)
	GetSubnetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.Subnet, error)
	GetNetworkSecurityGroupByID(ctx context.Context, tx *db.Tx, id string) (*cdbm.NetworkSecurityGroup, error)
	GetInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.Interface, error)
	GetInfiniBandInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.InfiniBandInterface, error)
	GetNVLinkInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.NVLinkInterface, error)
}

// SQLService implements the Service interface using the existing SQL DAOs.
type SQLService struct {
	dbSession *db.Session
}

// New creates a new SQLService.
func New(dbSession *db.Session) *SQLService {
	return &SQLService{dbSession: dbSession}
}

func (s *SQLService) GetVpcByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.Vpc, error) {
	dao := cdbm.NewVpcDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil)
}

func (s *SQLService) GetVpcs(ctx context.Context, tx *db.Tx, filter cdbm.VpcFilterInput, page paginator.PageInput) ([]cdbm.Vpc, int, error) {
	dao := cdbm.NewVpcDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) GetSubnetByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.Subnet, error) {
	dao := cdbm.NewSubnetDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil)
}

func (s *SQLService) GetNetworkSecurityGroupByID(ctx context.Context, tx *db.Tx, id string) (*cdbm.NetworkSecurityGroup, error) {
	dao := cdbm.NewNetworkSecurityGroupDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil)
}

func (s *SQLService) GetInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.Interface, error) {
	dao := cdbm.NewInterfaceDAO(s.dbSession)
	filter := cdbm.InterfaceFilterInput{InstanceIDs: []uuid.UUID{instanceID}}
	interfaces, _, err := dao.GetAll(ctx, tx, filter, paginator.PageInput{}, nil)
	return interfaces, err
}

func (s *SQLService) GetInfiniBandInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.InfiniBandInterface, error) {
	dao := cdbm.NewInfiniBandInterfaceDAO(s.dbSession)
	filter := cdbm.InfiniBandInterfaceFilterInput{InstanceIDs: []uuid.UUID{instanceID}}
	interfaces, _, err := dao.GetAll(ctx, tx, filter, paginator.PageInput{}, nil)
	return interfaces, err
}

func (s *SQLService) GetNVLinkInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.NVLinkInterface, error) {
	dao := cdbm.NewNVLinkInterfaceDAO(s.dbSession)
	filter := cdbm.NVLinkInterfaceFilterInput{InstanceIDs: []uuid.UUID{instanceID}}
	interfaces, _, err := dao.GetAll(ctx, tx, filter, paginator.PageInput{}, nil)
	return interfaces, err
}
