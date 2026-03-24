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
	GetSubnets(ctx context.Context, tx *db.Tx, filter cdbm.SubnetFilterInput, page paginator.PageInput) ([]cdbm.Subnet, int, error)
	GetNetworkSecurityGroupByID(ctx context.Context, tx *db.Tx, id string) (*cdbm.NetworkSecurityGroup, error)
	GetInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.Interface, error)
	GetInfiniBandInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.InfiniBandInterface, error)
	GetNVLinkInterfacesByInstanceID(ctx context.Context, tx *db.Tx, instanceID uuid.UUID) ([]cdbm.NVLinkInterface, error)
	GetIPBlockByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.IPBlock, error)
	CreateIPBlock(ctx context.Context, tx *db.Tx, input cdbm.IPBlockCreateInput) (*cdbm.IPBlock, error)
	UpdateIPBlock(ctx context.Context, tx *db.Tx, input cdbm.IPBlockUpdateInput) (*cdbm.IPBlock, error)
	DeleteIPBlock(ctx context.Context, tx *db.Tx, id uuid.UUID) error
	GetVpcPrefixes(ctx context.Context, tx *db.Tx, filter cdbm.VpcPrefixFilterInput, page paginator.PageInput) ([]cdbm.VpcPrefix, int, error)
	GetDpuExtensionServices(ctx context.Context, tx *db.Tx, filter cdbm.DpuExtensionServiceFilterInput, page paginator.PageInput) ([]cdbm.DpuExtensionService, int, error)
	GetInfiniBandPartitions(ctx context.Context, tx *db.Tx, filter cdbm.InfiniBandPartitionFilterInput, page paginator.PageInput) ([]cdbm.InfiniBandPartition, int, error)
	GetNVLinkLogicalPartitions(ctx context.Context, tx *db.Tx, filter cdbm.NVLinkLogicalPartitionFilterInput, page paginator.PageInput) ([]cdbm.NVLinkLogicalPartition, int, error)
	CreateMultipleInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.InterfaceCreateInput) ([]cdbm.Interface, error)
	CreateMultipleInfiniBandInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.InfiniBandInterfaceCreateInput) ([]cdbm.InfiniBandInterface, error)
	CreateMultipleNVLinkInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.NVLinkInterfaceCreateInput) ([]cdbm.NVLinkInterface, error)
	GetInterfaces(ctx context.Context, tx *db.Tx, filter cdbm.InterfaceFilterInput, page paginator.PageInput, includeRelations []string) ([]cdbm.Interface, int, error)
	GetInfiniBandInterfaces(ctx context.Context, tx *db.Tx, filter cdbm.InfiniBandInterfaceFilterInput, page paginator.PageInput, includeRelations []string) ([]cdbm.InfiniBandInterface, int, error)
	GetNVLinkInterfaces(ctx context.Context, tx *db.Tx, filter cdbm.NVLinkInterfaceFilterInput, page paginator.PageInput, includeRelations []string) ([]cdbm.NVLinkInterface, int, error)
	CreateInterface(ctx context.Context, tx *db.Tx, input cdbm.InterfaceCreateInput) (*cdbm.Interface, error)
	CreateInfiniBandInterface(ctx context.Context, tx *db.Tx, input cdbm.InfiniBandInterfaceCreateInput) (*cdbm.InfiniBandInterface, error)
	CreateNVLinkInterface(ctx context.Context, tx *db.Tx, input cdbm.NVLinkInterfaceCreateInput) (*cdbm.NVLinkInterface, error)
	UpdateInterface(ctx context.Context, tx *db.Tx, input cdbm.InterfaceUpdateInput) (*cdbm.Interface, error)
	UpdateInfiniBandInterface(ctx context.Context, tx *db.Tx, input cdbm.InfiniBandInterfaceUpdateInput) (*cdbm.InfiniBandInterface, error)
	UpdateMultipleNVLinkInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.NVLinkInterfaceUpdateInput) ([]cdbm.NVLinkInterface, error)
	GetNVLinkLogicalPartitionByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.NVLinkLogicalPartition, error)
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

func (s *SQLService) GetSubnets(ctx context.Context, tx *db.Tx, filter cdbm.SubnetFilterInput, page paginator.PageInput) ([]cdbm.Subnet, int, error) {
	dao := cdbm.NewSubnetDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) GetIPBlockByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.IPBlock, error) {
	dao := cdbm.NewIPBlockDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil)
}

func (s *SQLService) CreateIPBlock(ctx context.Context, tx *db.Tx, input cdbm.IPBlockCreateInput) (*cdbm.IPBlock, error) {
	dao := cdbm.NewIPBlockDAO(s.dbSession)
	return dao.Create(ctx, tx, input)
}

func (s *SQLService) UpdateIPBlock(ctx context.Context, tx *db.Tx, input cdbm.IPBlockUpdateInput) (*cdbm.IPBlock, error) {
	dao := cdbm.NewIPBlockDAO(s.dbSession)
	return dao.Update(ctx, tx, input)
}

func (s *SQLService) DeleteIPBlock(ctx context.Context, tx *db.Tx, id uuid.UUID) error {
	dao := cdbm.NewIPBlockDAO(s.dbSession)
	return dao.Delete(ctx, tx, id)
}

func (s *SQLService) GetVpcPrefixes(ctx context.Context, tx *db.Tx, filter cdbm.VpcPrefixFilterInput, page paginator.PageInput) ([]cdbm.VpcPrefix, int, error) {
	dao := cdbm.NewVpcPrefixDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) GetDpuExtensionServices(ctx context.Context, tx *db.Tx, filter cdbm.DpuExtensionServiceFilterInput, page paginator.PageInput) ([]cdbm.DpuExtensionService, int, error) {
	dao := cdbm.NewDpuExtensionServiceDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) GetInfiniBandPartitions(ctx context.Context, tx *db.Tx, filter cdbm.InfiniBandPartitionFilterInput, page paginator.PageInput) ([]cdbm.InfiniBandPartition, int, error) {
	dao := cdbm.NewInfiniBandPartitionDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) GetNVLinkLogicalPartitions(ctx context.Context, tx *db.Tx, filter cdbm.NVLinkLogicalPartitionFilterInput, page paginator.PageInput) ([]cdbm.NVLinkLogicalPartition, int, error) {
	dao := cdbm.NewNVLinkLogicalPartitionDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, nil)
}

func (s *SQLService) CreateMultipleInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.InterfaceCreateInput) ([]cdbm.Interface, error) {
	dao := cdbm.NewInterfaceDAO(s.dbSession)
	return dao.CreateMultiple(ctx, tx, inputs)
}

func (s *SQLService) CreateMultipleInfiniBandInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.InfiniBandInterfaceCreateInput) ([]cdbm.InfiniBandInterface, error) {
	dao := cdbm.NewInfiniBandInterfaceDAO(s.dbSession)
	return dao.CreateMultiple(ctx, tx, inputs)
}

func (s *SQLService) CreateMultipleNVLinkInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.NVLinkInterfaceCreateInput) ([]cdbm.NVLinkInterface, error) {
	dao := cdbm.NewNVLinkInterfaceDAO(s.dbSession)
	return dao.CreateMultiple(ctx, tx, inputs)
}

func (s *SQLService) GetInterfaces(ctx context.Context, tx *db.Tx, filter cdbm.InterfaceFilterInput, page paginator.PageInput, includeRelations []string) ([]cdbm.Interface, int, error) {
	dao := cdbm.NewInterfaceDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, includeRelations)
}

func (s *SQLService) GetInfiniBandInterfaces(ctx context.Context, tx *db.Tx, filter cdbm.InfiniBandInterfaceFilterInput, page paginator.PageInput, includeRelations []string) ([]cdbm.InfiniBandInterface, int, error) {
	dao := cdbm.NewInfiniBandInterfaceDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, includeRelations)
}

func (s *SQLService) GetNVLinkInterfaces(ctx context.Context, tx *db.Tx, filter cdbm.NVLinkInterfaceFilterInput, page paginator.PageInput, includeRelations []string) ([]cdbm.NVLinkInterface, int, error) {
	dao := cdbm.NewNVLinkInterfaceDAO(s.dbSession)
	return dao.GetAll(ctx, tx, filter, page, includeRelations)
}

func (s *SQLService) CreateInterface(ctx context.Context, tx *db.Tx, input cdbm.InterfaceCreateInput) (*cdbm.Interface, error) {
	dao := cdbm.NewInterfaceDAO(s.dbSession)
	return dao.Create(ctx, tx, input)
}

func (s *SQLService) CreateInfiniBandInterface(ctx context.Context, tx *db.Tx, input cdbm.InfiniBandInterfaceCreateInput) (*cdbm.InfiniBandInterface, error) {
	dao := cdbm.NewInfiniBandInterfaceDAO(s.dbSession)
	return dao.Create(ctx, tx, input)
}

func (s *SQLService) CreateNVLinkInterface(ctx context.Context, tx *db.Tx, input cdbm.NVLinkInterfaceCreateInput) (*cdbm.NVLinkInterface, error) {
	dao := cdbm.NewNVLinkInterfaceDAO(s.dbSession)
	return dao.Create(ctx, tx, input)
}

func (s *SQLService) UpdateInterface(ctx context.Context, tx *db.Tx, input cdbm.InterfaceUpdateInput) (*cdbm.Interface, error) {
	dao := cdbm.NewInterfaceDAO(s.dbSession)
	return dao.Update(ctx, tx, input)
}

func (s *SQLService) UpdateInfiniBandInterface(ctx context.Context, tx *db.Tx, input cdbm.InfiniBandInterfaceUpdateInput) (*cdbm.InfiniBandInterface, error) {
	dao := cdbm.NewInfiniBandInterfaceDAO(s.dbSession)
	return dao.Update(ctx, tx, input)
}

func (s *SQLService) UpdateMultipleNVLinkInterfaces(ctx context.Context, tx *db.Tx, inputs []cdbm.NVLinkInterfaceUpdateInput) ([]cdbm.NVLinkInterface, error) {
	dao := cdbm.NewNVLinkInterfaceDAO(s.dbSession)
	return dao.UpdateMultiple(ctx, tx, inputs)
}

func (s *SQLService) GetNVLinkLogicalPartitionByID(ctx context.Context, tx *db.Tx, id uuid.UUID) (*cdbm.NVLinkLogicalPartition, error) {
	dao := cdbm.NewNVLinkLogicalPartitionDAO(s.dbSession)
	return dao.GetByID(ctx, tx, id, nil)
}
