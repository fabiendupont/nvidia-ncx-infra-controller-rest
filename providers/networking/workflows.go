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
	tsdkWorker "go.temporal.io/sdk/worker"

	dpuExtensionServiceActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/dpuextensionservice"
	ibpActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/infinibandpartition"
	networkSecurityGroupActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/networksecuritygroup"
	nvLinkLogicalPartitionActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/nvlinklogicalpartition"
	subnetActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/subnet"
	vpcActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/vpc"
	vpcPeeringActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/vpcpeering"
	vpcPrefixActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/vpcprefix"
	cwfn "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/namespace"
	dpuExtensionServiceWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/dpuextensionservice"
	ibpWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/infinibandpartition"
	networkSecurityGroupWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/networksecuritygroup"
	nvLinkLogicalPartitionWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/nvlinklogicalpartition"
	subnetWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/subnet"
	vpcWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/vpc"
	vpcPeeringWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/vpcpeering"
	vpcPrefixWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/vpcprefix"
)

// TaskQueue returns the Temporal task queue name. Currently all providers
// share a single queue for backward compatibility.
func (p *NetworkingProvider) TaskQueue() string {
	return p.temporalQueue
}

// RegisterWorkflows registers networking-domain Temporal workflows on the
// supplied worker. The set of workflows depends on the Temporal namespace
// this worker operates in (cloud vs site).
func (p *NetworkingProvider) RegisterWorkflows(w tsdkWorker.Worker) {
	if p.temporalNamespace == cwfn.CloudNamespace {
		// VPC workflows
		w.RegisterWorkflow(vpcWorkflow.CreateVpc)
		w.RegisterWorkflow(vpcWorkflow.DeleteVpc)

		// Subnet workflows
		w.RegisterWorkflow(subnetWorkflow.CreateSubnet)
		w.RegisterWorkflow(subnetWorkflow.DeleteSubnet)

		// InfiniBandPartition workflows
		w.RegisterWorkflow(ibpWorkflow.CreateInfiniBandPartition)
		w.RegisterWorkflow(ibpWorkflow.DeleteInfiniBandPartition)
	} else if p.temporalNamespace == cwfn.SiteNamespace {
		// VPC workflows
		w.RegisterWorkflow(vpcWorkflow.UpdateVpcInfo)
		w.RegisterWorkflow(vpcWorkflow.UpdateVpcInventory)

		// Subnet workflows
		w.RegisterWorkflow(subnetWorkflow.UpdateSubnetInfo)
		w.RegisterWorkflow(subnetWorkflow.UpdateSubnetInventory)

		// InfiniBandPartition workflows
		w.RegisterWorkflow(ibpWorkflow.UpdateInfiniBandPartitionInfo)
		w.RegisterWorkflow(ibpWorkflow.UpdateInfiniBandPartitionInventory)

		// NetworkSecurityGroup workflow
		w.RegisterWorkflow(networkSecurityGroupWorkflow.UpdateNetworkSecurityGroupInventory)

		// VpcPrefix workflow
		w.RegisterWorkflow(vpcPrefixWorkflow.UpdateVpcPrefixInventory)

		// VpcPeering workflow
		w.RegisterWorkflow(vpcPeeringWorkflow.UpdateVpcPeeringInventory)

		// DpuExtensionService workflow
		w.RegisterWorkflow(dpuExtensionServiceWorkflow.UpdateDpuExtensionServiceInventory)

		// NVLinkLogicalPartition workflow
		w.RegisterWorkflow(nvLinkLogicalPartitionWorkflow.UpdateNVLinkLogicalPartitionInventory)
	}
}

// RegisterActivities registers networking-domain Temporal activity managers
// on the supplied worker.
func (p *NetworkingProvider) RegisterActivities(w tsdkWorker.Worker) {
	vpcManager := vpcActivity.NewManageVpc(p.dbSession, p.workflowSiteClientPool, p.tc)
	if p.hooks != nil {
		vpcManager.SetHooks(p.hooks)
	}
	w.RegisterActivity(&vpcManager)

	subnetManager := subnetActivity.NewManageSubnet(p.dbSession, p.workflowSiteClientPool, p.tc)
	w.RegisterActivity(&subnetManager)

	ibpManager := ibpActivity.NewManageInfiniBandPartition(p.dbSession, p.workflowSiteClientPool)
	w.RegisterActivity(&ibpManager)

	networkSecurityGroupManager := networkSecurityGroupActivity.NewManageNetworkSecurityGroup(p.dbSession, p.workflowSiteClientPool)
	w.RegisterActivity(&networkSecurityGroupManager)

	vpcPrefixManager := vpcPrefixActivity.NewManageVpcPrefix(p.dbSession, p.workflowSiteClientPool)
	w.RegisterActivity(&vpcPrefixManager)

	vpcPeeringManager := vpcPeeringActivity.NewManageVpcPeering(p.dbSession, p.workflowSiteClientPool)
	w.RegisterActivity(&vpcPeeringManager)

	dpuExtensionServiceManager := dpuExtensionServiceActivity.NewManageDpuExtensionService(p.dbSession, p.workflowSiteClientPool)
	w.RegisterActivity(&dpuExtensionServiceManager)

	nvLinkLogicalPartitionManager := nvLinkLogicalPartitionActivity.NewManageNVLinkLogicalPartition(p.dbSession, p.workflowSiteClientPool)
	w.RegisterActivity(&nvLinkLogicalPartitionManager)
}
