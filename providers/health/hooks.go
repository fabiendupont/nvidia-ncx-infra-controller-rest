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

package health

import (
	"context"
	"fmt"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers hooks and reactions for the fault management system.
func (p *HealthProvider) registerHooks(registry *provider.Registry) {
	// After a fault is ingested, start remediation workflow
	registry.RegisterReaction(provider.Reaction{
		Feature:        "health",
		Event:          provider.EventPostHealthEventIngested,
		TargetWorkflow: "health-fault-remediation",
		SignalName:     "fault-ingested",
	})

	// Block instance creation on machines with open critical faults
	registry.RegisterHook(provider.SyncHook{
		Feature: "compute",
		Event:   provider.EventPreCreateInstance,
		Handler: p.blockInstanceOnFaultyMachine,
	})
}

// blockInstanceOnFaultyMachine prevents instance creation on machines that
// have open critical faults. It extracts the machine_id from the payload,
// queries the fault store for open critical faults on that machine, and
// returns an error if any are found.
func (p *HealthProvider) blockInstanceOnFaultyMachine(ctx context.Context, payload interface{}) error {
	// Extract machine_id from payload
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}
	machineID, ok := data["machine_id"].(string)
	if !ok || machineID == "" {
		return nil
	}

	// Query faultStore for open critical faults on that machine
	faults, err := p.faultStore.ListOpenCriticalByMachine(ctx, machineID)
	if err != nil {
		return fmt.Errorf("failed to check faults for machine %s: %w", machineID, err)
	}

	if len(faults) > 0 {
		return fmt.Errorf("machine %s has %d open critical fault(s); instance creation blocked", machineID, len(faults))
	}

	return nil
}
