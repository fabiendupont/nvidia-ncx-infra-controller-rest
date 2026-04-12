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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockInstanceOnFaultyMachine_NoFaults(t *testing.T) {
	p := &HealthProvider{
		faultStore: NewFaultEventStore(nil),
	}

	payload := map[string]interface{}{
		"machine_id": "machine-1",
	}

	err := p.blockInstanceOnFaultyMachine(context.Background(), payload)
	require.NoError(t, err)
}

func TestBlockInstanceOnFaultyMachine_WithOpenCritical(t *testing.T) {
	p := &HealthProvider{
		faultStore: NewFaultEventStore(nil),
	}

	machineID := "machine-1"
	require.NoError(t, p.faultStore.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "GPU fault", SiteID: "site-1", State: FaultStateOpen,
		MachineID: &machineID,
	}))

	payload := map[string]interface{}{
		"machine_id": machineID,
	}

	err := p.blockInstanceOnFaultyMachine(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 open critical fault")
	assert.Contains(t, err.Error(), "blocked")
}

func TestBlockInstanceOnFaultyMachine_NilPayload(t *testing.T) {
	p := &HealthProvider{
		faultStore: NewFaultEventStore(nil),
	}

	// Non-map payload should be a no-op.
	err := p.blockInstanceOnFaultyMachine(context.Background(), nil)
	require.NoError(t, err)
}

func TestBlockInstanceOnFaultyMachine_MissingMachineID(t *testing.T) {
	p := &HealthProvider{
		faultStore: NewFaultEventStore(nil),
	}

	payload := map[string]interface{}{
		"other": "value",
	}

	err := p.blockInstanceOnFaultyMachine(context.Background(), payload)
	require.NoError(t, err)
}

func TestBlockInstanceOnFaultyMachine_ResolvedFaultsAllowed(t *testing.T) {
	p := &HealthProvider{
		faultStore: NewFaultEventStore(nil),
	}

	machineID := "machine-1"
	require.NoError(t, p.faultStore.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "old fault", SiteID: "site-1", State: FaultStateResolved,
		MachineID: &machineID,
	}))

	payload := map[string]interface{}{
		"machine_id": machineID,
	}

	// Resolved critical faults should not block.
	err := p.blockInstanceOnFaultyMachine(context.Background(), payload)
	require.NoError(t, err)
}
