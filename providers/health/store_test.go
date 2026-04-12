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

// ---------- FaultEventStore ----------

func TestFaultEventStore_CreateAndGetByID(t *testing.T) {
	store := NewFaultEventStore(nil)

	event := &FaultEvent{
		Source:    "dcgm",
		Severity: SeverityCritical,
		Component: ComponentGPU,
		Message:  "GPU ECC double-bit error",
		SiteID:   "site-1",
	}

	err := store.Create(event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
	assert.Equal(t, FaultStateOpen, event.State)
	assert.False(t, event.CreatedAt.IsZero())

	got, err := store.GetByID(event.ID)
	require.NoError(t, err)
	assert.Equal(t, event.ID, got.ID)
	assert.Equal(t, "dcgm", got.Source)
}

func TestFaultEventStore_GetByID_NotFound(t *testing.T) {
	store := NewFaultEventStore(nil)

	_, err := store.GetByID("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFaultEventStore_Update(t *testing.T) {
	store := NewFaultEventStore(nil)

	event := &FaultEvent{
		Source:    "dcgm",
		Severity: SeverityCritical,
		Component: ComponentGPU,
		Message:  "GPU fault",
		SiteID:   "site-1",
	}
	require.NoError(t, store.Create(event))

	event.State = FaultStateAcknowledged
	updated, err := store.Update(event)
	require.NoError(t, err)
	assert.Equal(t, FaultStateAcknowledged, updated.State)
}

func TestFaultEventStore_Update_NotFound(t *testing.T) {
	store := NewFaultEventStore(nil)

	_, err := store.Update(&FaultEvent{ID: "nonexistent"})
	require.Error(t, err)
}

func TestFaultEventStore_Delete(t *testing.T) {
	store := NewFaultEventStore(nil)

	event := &FaultEvent{
		Source:    "test",
		Severity: SeverityWarning,
		Component: ComponentNetwork,
		Message:  "link flap",
		SiteID:   "site-1",
	}
	require.NoError(t, store.Create(event))

	err := store.Delete(event.ID)
	require.NoError(t, err)

	_, err = store.GetByID(event.ID)
	require.Error(t, err)
}

func TestFaultEventStore_Delete_NotFound(t *testing.T) {
	store := NewFaultEventStore(nil)

	err := store.Delete("nonexistent")
	require.Error(t, err)
}

func TestFaultEventStore_GetAll_WithFilter(t *testing.T) {
	store := NewFaultEventStore(nil)

	require.NoError(t, store.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "gpu-1", SiteID: "site-1",
	}))
	require.NoError(t, store.Create(&FaultEvent{
		Source: "nhc", Severity: SeverityWarning, Component: ComponentNetwork,
		Message: "net-1", SiteID: "site-2",
	}))
	require.NoError(t, store.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "gpu-2", SiteID: "site-1",
	}))

	// Filter by severity
	events := store.GetAll(FaultEventFilter{Severity: []string{SeverityCritical}})
	assert.Len(t, events, 2)

	// Filter by component
	events = store.GetAll(FaultEventFilter{Component: []string{ComponentNetwork}})
	assert.Len(t, events, 1)

	// Filter by site
	siteID := "site-1"
	events = store.GetAll(FaultEventFilter{SiteID: &siteID})
	assert.Len(t, events, 2)

	// No filter returns all
	events = store.GetAll(FaultEventFilter{})
	assert.Len(t, events, 3)
}

func TestFaultEventStore_GetSummary(t *testing.T) {
	store := NewFaultEventStore(nil)

	require.NoError(t, store.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "gpu fault", SiteID: "site-1", State: FaultStateOpen,
	}))
	require.NoError(t, store.Create(&FaultEvent{
		Source: "nhc", Severity: SeverityWarning, Component: ComponentNetwork,
		Message: "link down", SiteID: "site-2", State: FaultStateResolved,
	}))

	summary := store.GetSummary()
	assert.Equal(t, 1, summary.BySeverity[SeverityCritical])
	assert.Equal(t, 1, summary.BySeverity[SeverityWarning])
	assert.Equal(t, 1, summary.ByComponent[ComponentGPU])
	assert.Equal(t, 1, summary.ByState[FaultStateOpen])
	assert.Equal(t, 1, summary.ByState[FaultStateResolved])
}

func TestFaultEventStore_ListOpenCriticalByMachine(t *testing.T) {
	store := NewFaultEventStore(nil)

	machineID := "machine-1"
	otherMachine := "machine-2"

	// Open critical on target machine
	require.NoError(t, store.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "gpu fault", SiteID: "site-1", State: FaultStateOpen,
		MachineID: &machineID,
	}))
	// Resolved critical on target machine (should not match)
	require.NoError(t, store.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "old fault", SiteID: "site-1", State: FaultStateResolved,
		MachineID: &machineID,
	}))
	// Open warning on target machine (should not match)
	require.NoError(t, store.Create(&FaultEvent{
		Source: "nhc", Severity: SeverityWarning, Component: ComponentNetwork,
		Message: "link flap", SiteID: "site-1", State: FaultStateOpen,
		MachineID: &machineID,
	}))
	// Open critical on different machine (should not match)
	require.NoError(t, store.Create(&FaultEvent{
		Source: "dcgm", Severity: SeverityCritical, Component: ComponentGPU,
		Message: "other gpu", SiteID: "site-1", State: FaultStateOpen,
		MachineID: &otherMachine,
	}))

	faults, err := store.ListOpenCriticalByMachine(context.Background(), machineID)
	require.NoError(t, err)
	assert.Len(t, faults, 1)
	assert.Equal(t, "gpu fault", faults[0].Message)
}

// ---------- ServiceEventStore ----------

func TestServiceEventStore_CreateAndGetByID(t *testing.T) {
	store := NewServiceEventStore(nil)

	event := &ServiceEvent{
		TenantID: "tenant-1",
		Summary:  "GPU maintenance",
		Impact:   "Instance performance may be degraded",
	}

	created, err := store.Create(event)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "active", created.State)

	got, err := store.GetByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", got.TenantID)
}

func TestServiceEventStore_GetByID_NotFound(t *testing.T) {
	store := NewServiceEventStore(nil)

	_, err := store.GetByID("nonexistent")
	require.Error(t, err)
}

func TestServiceEventStore_GetByTenantID(t *testing.T) {
	store := NewServiceEventStore(nil)

	_, err := store.Create(&ServiceEvent{TenantID: "tenant-1", Summary: "event-1", Impact: "low"})
	require.NoError(t, err)
	_, err = store.Create(&ServiceEvent{TenantID: "tenant-2", Summary: "event-2", Impact: "high"})
	require.NoError(t, err)
	_, err = store.Create(&ServiceEvent{TenantID: "tenant-1", Summary: "event-3", Impact: "medium"})
	require.NoError(t, err)

	events := store.GetByTenantID("tenant-1")
	assert.Len(t, events, 2)

	events = store.GetByTenantID("tenant-3")
	assert.Len(t, events, 0)
}

func TestServiceEventStore_Update(t *testing.T) {
	store := NewServiceEventStore(nil)

	created, err := store.Create(&ServiceEvent{
		TenantID: "tenant-1", Summary: "event", Impact: "low",
	})
	require.NoError(t, err)

	created.State = "resolved"
	updated, err := store.Update(created)
	require.NoError(t, err)
	assert.Equal(t, "resolved", updated.State)
}

func TestServiceEventStore_Update_NotFound(t *testing.T) {
	store := NewServiceEventStore(nil)

	_, err := store.Update(&ServiceEvent{ID: "nonexistent"})
	require.Error(t, err)
}

// ---------- FaultServiceEventStore ----------

func TestFaultServiceEventStore_LinkAndQuery(t *testing.T) {
	store := NewFaultServiceEventStore(nil)

	store.Link("fault-1", "service-1")
	store.Link("fault-1", "service-2")
	store.Link("fault-2", "service-1")

	serviceIDs := store.GetServiceEventsByFaultID("fault-1")
	assert.Len(t, serviceIDs, 2)

	faultIDs := store.GetFaultEventsByServiceID("service-1")
	assert.Len(t, faultIDs, 2)

	// Non-existent returns empty
	assert.Empty(t, store.GetServiceEventsByFaultID("fault-99"))
	assert.Empty(t, store.GetFaultEventsByServiceID("service-99"))
}

// ---------- ClassificationStore ----------

func TestClassificationStore_GetAndSet(t *testing.T) {
	store := NewClassificationStore(map[string]*ClassificationMapping{
		"gpu-xid-48": {
			Classification: "gpu-xid-48",
			Component:      ComponentGPU,
			Severity:       SeverityCritical,
			Remediation:    "gpu-reset",
			MaxRetries:     3,
		},
	})

	m, err := store.Get("gpu-xid-48")
	require.NoError(t, err)
	assert.Equal(t, "gpu-reset", m.Remediation)

	_, err = store.Get("unknown")
	require.Error(t, err)

	store.Set("new-class", &ClassificationMapping{
		Classification: "new-class",
		Remediation:    "reboot",
	})

	m, err = store.Get("new-class")
	require.NoError(t, err)
	assert.Equal(t, "reboot", m.Remediation)
}

func TestClassificationStore_GetAll(t *testing.T) {
	store := NewClassificationStore(map[string]*ClassificationMapping{
		"a": {Classification: "a"},
		"b": {Classification: "b"},
	})

	all := store.GetAll()
	assert.Len(t, all, 2)
}
