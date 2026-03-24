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

package networking

import (
	"net/http"

	echo "github.com/labstack/echo/v4"

	apiHandler "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler"
)

// RegisterRoutes registers all networking-related API routes on the given group.
func (p *NetworkingProvider) RegisterRoutes(group *echo.Group) {
	prefix := p.apiPathPrefix

	// VPC endpoints
	group.Add(http.MethodPost, prefix+"/vpc", apiHandler.NewCreateVPCHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/vpc", apiHandler.NewGetAllVPCHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/vpc/:id", apiHandler.NewGetVPCHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/vpc/:id", apiHandler.NewUpdateVPCHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/vpc/:id", apiHandler.NewDeleteVPCHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/vpc/:id/virtualization", apiHandler.NewUpdateVPCVirtualizationHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// VpcPrefix endpoints
	group.Add(http.MethodPost, prefix+"/vpc-prefix", apiHandler.NewCreateVpcPrefixHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/vpc-prefix", apiHandler.NewGetAllVpcPrefixHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/vpc-prefix/:id", apiHandler.NewGetVpcPrefixHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/vpc-prefix/:id", apiHandler.NewUpdateVpcPrefixHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/vpc-prefix/:id", apiHandler.NewDeleteVpcPrefixHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// IPBlock endpoints
	group.Add(http.MethodPost, prefix+"/ipblock", apiHandler.NewCreateIPBlockHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/ipblock", apiHandler.NewGetAllIPBlockHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/ipblock/:id", apiHandler.NewGetIPBlockHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/ipblock/:id/derived", apiHandler.NewGetAllDerivedIPBlockHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/ipblock/:id", apiHandler.NewUpdateIPBlockHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/ipblock/:id", apiHandler.NewDeleteIPBlockHandler(p.dbSession, p.tc, p.cfg).Handle)

	// Subnet endpoints
	group.Add(http.MethodPost, prefix+"/subnet", apiHandler.NewCreateSubnetHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/subnet", apiHandler.NewGetAllSubnetHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/subnet/:id", apiHandler.NewGetSubnetHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/subnet/:id", apiHandler.NewUpdateSubnetHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/subnet/:id", apiHandler.NewDeleteSubnetHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// NetworkSecurityGroup endpoints
	group.Add(http.MethodPost, prefix+"/network-security-group", apiHandler.NewCreateNetworkSecurityGroupHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/network-security-group", apiHandler.NewGetAllNetworkSecurityGroupHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/network-security-group/:id", apiHandler.NewGetNetworkSecurityGroupHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/network-security-group/:id", apiHandler.NewUpdateNetworkSecurityGroupHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/network-security-group/:id", apiHandler.NewDeleteNetworkSecurityGroupHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// Interface endpoints
	group.Add(http.MethodGet, prefix+"/instance/:instanceId/interface", apiHandler.NewGetAllInterfaceHandler(p.dbSession, p.tc, p.cfg).Handle)

	// Instance InfiniBandInterface endpoints
	group.Add(http.MethodGet, prefix+"/instance/:instanceId/infiniband-interface", apiHandler.NewGetAllInstanceInfiniBandInterfaceHandler(p.dbSession, p.tc, p.cfg).Handle)

	// Instance NVLinkInterface endpoints
	group.Add(http.MethodGet, prefix+"/instance/:instanceId/nvlink-interface", apiHandler.NewGetAllInstanceNVLinkInterfaceHandler(p.dbSession, p.tc, p.cfg).Handle)

	// InfiniBandInterface endpoints
	group.Add(http.MethodGet, prefix+"/infiniband-interface", apiHandler.NewGetAllInfiniBandInterfaceHandler(p.dbSession, p.tc, p.cfg, nil).Handle)

	// NVLinkInterface endpoints
	group.Add(http.MethodGet, prefix+"/nvlink-interface", apiHandler.NewGetAllNVLinkInterfaceHandler(p.dbSession, p.tc, p.cfg, nil).Handle)

	// InfiniBandPartition endpoints
	group.Add(http.MethodPost, prefix+"/infiniband-partition", apiHandler.NewCreateInfiniBandPartitionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/infiniband-partition", apiHandler.NewGetAllInfiniBandPartitionHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/infiniband-partition/:id", apiHandler.NewGetInfiniBandPartitionHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/infiniband-partition/:id", apiHandler.NewUpdateInfiniBandPartitionHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/infiniband-partition/:id", apiHandler.NewDeleteInfiniBandPartitionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// NVLinkLogicalPartition endpoints
	group.Add(http.MethodPost, prefix+"/nvlink-logical-partition", apiHandler.NewCreateNVLinkLogicalPartitionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/nvlink-logical-partition", apiHandler.NewGetAllNVLinkLogicalPartitionHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/nvlink-logical-partition/:id", apiHandler.NewGetNVLinkLogicalPartitionHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/nvlink-logical-partition/:id", apiHandler.NewUpdateNVLinkLogicalPartitionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/nvlink-logical-partition/:id", apiHandler.NewDeleteNVLinkLogicalPartitionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// DPU Extension Service endpoints
	group.Add(http.MethodPost, prefix+"/dpu-extension-service", apiHandler.NewCreateDpuExtensionServiceHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/dpu-extension-service", apiHandler.NewGetAllDpuExtensionServiceHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/dpu-extension-service/:id", apiHandler.NewGetDpuExtensionServiceHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/dpu-extension-service/:id", apiHandler.NewUpdateDpuExtensionServiceHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/dpu-extension-service/:id", apiHandler.NewDeleteDpuExtensionServiceHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/dpu-extension-service/:id/version/:version", apiHandler.NewGetDpuExtensionServiceVersionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/dpu-extension-service/:id/version/:version", apiHandler.NewDeleteDpuExtensionServiceVersionHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
}
