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

package compute

import (
	tsdkWorker "go.temporal.io/sdk/worker"

	cwfn "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/namespace"

	sc "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/client/site"
	wfconfig "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/config"

	instanceActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/instance"
	instanceTypeActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/instancetype"
	machineActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/machine"
	osImageActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/operatingsystem"
	skuActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/sku"
	sshKeyGroupActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/sshkeygroup"

	instanceWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/instance"
	instanceTypeWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/instancetype"
	machineWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/machine"
	osImageWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/operatingsystem"
	skuWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/sku"
	sshKeyGroupWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/sshkeygroup"
)

// TaskQueue returns the Temporal task queue name for the compute provider.
func (p *ComputeProvider) TaskQueue() string {
	return p.temporalQueue
}

// RegisterWorkflows registers all compute-domain Temporal workflows
// on the given worker, filtered by the Temporal namespace.
func (p *ComputeProvider) RegisterWorkflows(w tsdkWorker.Worker) {
	if p.temporalNamespace == cwfn.CloudNamespace {
		// Instance workflows
		w.RegisterWorkflow(instanceWorkflow.CreateInstance)
		w.RegisterWorkflow(instanceWorkflow.DeleteInstance)
		w.RegisterWorkflow(instanceWorkflow.RebootInstance)

		// SSHKeyGroup workflows
		w.RegisterWorkflow(sshKeyGroupWorkflow.SyncSSHKeyGroup)
		w.RegisterWorkflow(sshKeyGroupWorkflow.DeleteSSHKeyGroup)
	} else if p.temporalNamespace == cwfn.SiteNamespace {
		// Machine workflows
		w.RegisterWorkflow(machineWorkflow.UpdateMachineInventory)

		// Instance workflows
		w.RegisterWorkflow(instanceWorkflow.UpdateInstanceInfo)
		w.RegisterWorkflow(instanceWorkflow.UpdateInstanceInventory)
		w.RegisterWorkflow(instanceWorkflow.UpdateInstanceRebootInfo)

		// SSHKeyGroup workflows
		w.RegisterWorkflow(sshKeyGroupWorkflow.UpdateSSHKeyGroupInfo)
		w.RegisterWorkflow(sshKeyGroupWorkflow.UpdateSSHKeyGroupInventory)

		// InstanceType workflow
		w.RegisterWorkflow(instanceTypeWorkflow.UpdateInstanceTypeInventory)

		// OS Image workflow
		w.RegisterWorkflow(osImageWorkflow.UpdateOsImageInventory)

		// SKU workflow
		w.RegisterWorkflow(skuWorkflow.UpdateSkuInventory)
	}
}

// RegisterActivities registers all compute-domain Temporal activities
// on the given worker.
func (p *ComputeProvider) RegisterActivities(w tsdkWorker.Worker) {
	siteClientPool := p.workflowSiteClientPool.(*sc.ClientPool)

	machineManager := machineActivity.NewManageMachine(p.dbSession, siteClientPool)
	w.RegisterActivity(&machineManager)

	var wfCfg *wfconfig.Config
	if p.workflowConfig != nil {
		wfCfg = p.workflowConfig.(*wfconfig.Config)
	}
	instanceManager := instanceActivity.NewManageInstance(p.dbSession, siteClientPool, p.tc, wfCfg)
	if p.hooks != nil {
		instanceManager.SetHooks(p.hooks)
	}
	w.RegisterActivity(&instanceManager)

	sshKeyGroupManager := sshKeyGroupActivity.NewManageSSHKeyGroup(p.dbSession, siteClientPool)
	w.RegisterActivity(&sshKeyGroupManager)

	instanceTypeManager := instanceTypeActivity.NewManageInstanceType(p.dbSession, siteClientPool)
	w.RegisterActivity(&instanceTypeManager)

	osImageManager := osImageActivity.NewManageOsImage(p.dbSession, siteClientPool)
	w.RegisterActivity(&osImageManager)

	skuManager := skuActivity.NewManageSku(p.dbSession, siteClientPool)
	w.RegisterActivity(&skuManager)
}
