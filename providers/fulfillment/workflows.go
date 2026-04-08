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

package fulfillment

import (
	tsdkWorker "go.temporal.io/sdk/worker"
)

// TaskQueue returns the Temporal task queue name for the fulfillment provider.
func (p *FulfillmentProvider) TaskQueue() string {
	return "fulfillment-tasks"
}

// RegisterWorkflows registers all fulfillment-domain Temporal workflows
// on the given worker.
func (p *FulfillmentProvider) RegisterWorkflows(w tsdkWorker.Worker) {
	w.RegisterWorkflow(TenantProvisioningWorkflow)
	w.RegisterWorkflow(TenantTeardownWorkflow)
	w.RegisterWorkflow(ServiceScaleWorkflow)
	w.RegisterWorkflow(BlueprintExecutionWorkflow)
}

// RegisterActivities registers all fulfillment-domain Temporal activities
// on the given worker.
func (p *FulfillmentProvider) RegisterActivities(w tsdkWorker.Worker) {
	activities := &FulfillmentActivities{
		orderStore:   p.orderStore,
		serviceStore: p.serviceStore,
	}
	w.RegisterActivity(activities)

	execActivities := NewExecutionActivities(p.orderStore, p.serviceStore)
	w.RegisterActivity(execActivities)
}
