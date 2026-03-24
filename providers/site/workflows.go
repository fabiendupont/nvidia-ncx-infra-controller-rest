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

package site

import (
	tsdkWorker "go.temporal.io/sdk/worker"

	sc "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/client/site"
	wfconfig "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/config"
	cwfn "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/namespace"

	expectedMachineActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/expectedmachine"
	expectedPowerShelfActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/expectedpowershelf"
	expectedSwitchActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/expectedswitch"
	siteActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/site"

	expectedMachineWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/expectedmachine"
	expectedPowerShelfWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/expectedpowershelf"
	expectedSwitchWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/expectedswitch"
	siteWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/site"
)

// TaskQueue returns the Temporal task queue name for the site provider.
func (p *SiteProvider) TaskQueue() string {
	return p.temporalQueue
}

// RegisterWorkflows registers all site-domain Temporal workflows
// on the given worker, filtered by the Temporal namespace.
func (p *SiteProvider) RegisterWorkflows(w tsdkWorker.Worker) {
	if p.temporalNamespace == cwfn.CloudNamespace {
		// Site workflows triggered by Cloud services
		w.RegisterWorkflow(siteWorkflow.DeleteSiteComponents)
		w.RegisterWorkflow(siteWorkflow.MonitorHealthForAllSites)
		w.RegisterWorkflow(siteWorkflow.CheckHealthForAllSites)
		w.RegisterWorkflow(siteWorkflow.MonitorTemporalCertExpirationForAllSites)
		w.RegisterWorkflow(siteWorkflow.MonitorSiteTemporalNamespaces)
	} else if p.temporalNamespace == cwfn.SiteNamespace {
		// Site workflows triggered by Site Agent
		w.RegisterWorkflow(siteWorkflow.UpdateAgentCertExpiry)

		// ExpectedMachine workflow
		w.RegisterWorkflow(expectedMachineWorkflow.UpdateExpectedMachineInventory)

		// ExpectedPowerShelf workflow
		w.RegisterWorkflow(expectedPowerShelfWorkflow.UpdateExpectedPowerShelfInventory)

		// ExpectedSwitch workflow
		w.RegisterWorkflow(expectedSwitchWorkflow.UpdateExpectedSwitchInventory)
	}
}

// RegisterActivities registers all site-domain Temporal activities
// on the given worker.
func (p *SiteProvider) RegisterActivities(w tsdkWorker.Worker) {
	siteClientPool := p.workflowSiteClientPool.(*sc.ClientPool)

	// Site activities
	var wfCfg *wfconfig.Config
	if p.workflowConfig != nil {
		wfCfg = p.workflowConfig.(*wfconfig.Config)
	}
	siteManager := siteActivity.NewManageSite(p.dbSession, siteClientPool, p.tc, wfCfg)
	w.RegisterActivity(&siteManager)

	// ExpectedMachine activities
	expectedMachineManager := expectedMachineActivity.NewManageExpectedMachine(p.dbSession, siteClientPool)
	w.RegisterActivity(&expectedMachineManager)

	// ExpectedPowerShelf activities
	expectedPowerShelfManager := expectedPowerShelfActivity.NewManageExpectedPowerShelf(p.dbSession, siteClientPool)
	w.RegisterActivity(&expectedPowerShelfManager)

	// ExpectedSwitch activities
	expectedSwitchManager := expectedSwitchActivity.NewManageExpectedSwitch(p.dbSession, siteClientPool)
	w.RegisterActivity(&expectedSwitchManager)
}
