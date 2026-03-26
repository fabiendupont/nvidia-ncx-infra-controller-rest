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

package showback

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartMetering_CreatesOpenRecord(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()
	resourceID := uuid.New()

	store.StartMetering(tenantID, resourceID, "gpu-hours")

	// The record should exist and be open (no EndTime).
	require.Len(t, store.records, 1)
	for _, rec := range store.records {
		assert.Equal(t, tenantID, rec.TenantID)
		assert.Equal(t, resourceID, rec.ResourceID)
		assert.Equal(t, "gpu-hours", rec.MetricName)
		assert.False(t, rec.StartTime.IsZero())
		assert.Nil(t, rec.EndTime)
		assert.Equal(t, 0.0, rec.Value)
	}

	// The resource should be tracked in the byRes index.
	_, ok := store.byRes[resourceID]
	assert.True(t, ok)
}

func TestStopMetering_ClosesRecord(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()
	resourceID := uuid.New()

	store.StartMetering(tenantID, resourceID, "gpu-hours")

	err := store.StopMetering(resourceID)
	require.NoError(t, err)

	// The record should now have an EndTime and a positive Value.
	require.Len(t, store.records, 1)
	for _, rec := range store.records {
		require.NotNil(t, rec.EndTime)
		assert.False(t, rec.EndTime.IsZero())
		assert.GreaterOrEqual(t, rec.Value, 0.0)
	}

	// The resource should no longer be in the byRes index.
	_, ok := store.byRes[resourceID]
	assert.False(t, ok)
}

func TestStopMetering_UnknownResource(t *testing.T) {
	store := NewUsageStore()

	err := store.StopMetering(uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active metering record")
}

func TestGetUsageByTenant_ReturnsSummary(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()
	resourceID := uuid.New()

	store.StartMetering(tenantID, resourceID, "gpu-hours")
	err := store.StopMetering(resourceID)
	require.NoError(t, err)

	summary := store.GetUsageByTenant(tenantID)

	assert.Equal(t, tenantID, summary.TenantID)
	assert.Equal(t, "current-month", summary.Period)
	assert.Contains(t, summary.Metrics, "gpu-hours")
	assert.GreaterOrEqual(t, summary.Metrics["gpu-hours"], 0.0)
}

func TestGetUsageByTenant_OpenRecordIncluded(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()
	resourceID := uuid.New()

	store.StartMetering(tenantID, resourceID, "gpu-hours")

	// An open record should still contribute to the summary.
	summary := store.GetUsageByTenant(tenantID)
	assert.Contains(t, summary.Metrics, "gpu-hours")
	assert.GreaterOrEqual(t, summary.Metrics["gpu-hours"], 0.0)
}

func TestGetUsageByTenant_NoRecords(t *testing.T) {
	store := NewUsageStore()
	summary := store.GetUsageByTenant(uuid.New())

	assert.Equal(t, "current-month", summary.Period)
	assert.Empty(t, summary.Metrics)
}

func TestGetUsageByService_ReturnsSummary(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()
	resourceID := uuid.New()
	serviceID := uuid.New()

	store.StartMetering(tenantID, resourceID, "gpu-hours")

	// Manually set the ServiceID since StartMetering defaults to uuid.Nil.
	for _, rec := range store.records {
		rec.ServiceID = serviceID
	}

	err := store.StopMetering(resourceID)
	require.NoError(t, err)

	summary := store.GetUsageByService(serviceID)

	assert.Equal(t, tenantID, summary.TenantID)
	assert.Equal(t, "current-month", summary.Period)
	assert.Contains(t, summary.Metrics, "gpu-hours")
}

func TestGetUsageByService_NoMatch(t *testing.T) {
	store := NewUsageStore()
	summary := store.GetUsageByService(uuid.New())

	assert.Equal(t, "current-month", summary.Period)
	assert.Empty(t, summary.Metrics)
}

func TestMultipleRecords_SameTenant(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()

	res1 := uuid.New()
	res2 := uuid.New()

	store.StartMetering(tenantID, res1, "gpu-hours")
	store.StartMetering(tenantID, res2, "storage-gb-hours")

	err := store.StopMetering(res1)
	require.NoError(t, err)
	err = store.StopMetering(res2)
	require.NoError(t, err)

	summary := store.GetUsageByTenant(tenantID)

	assert.Equal(t, tenantID, summary.TenantID)
	assert.Contains(t, summary.Metrics, "gpu-hours")
	assert.Contains(t, summary.Metrics, "storage-gb-hours")
}

func TestMultipleRecords_SameMetric(t *testing.T) {
	store := NewUsageStore()
	tenantID := uuid.New()

	res1 := uuid.New()
	res2 := uuid.New()

	store.StartMetering(tenantID, res1, "gpu-hours")
	err := store.StopMetering(res1)
	require.NoError(t, err)

	store.StartMetering(tenantID, res2, "gpu-hours")
	err = store.StopMetering(res2)
	require.NoError(t, err)

	summary := store.GetUsageByTenant(tenantID)

	// Both records contribute to the same metric.
	assert.Contains(t, summary.Metrics, "gpu-hours")
	require.Len(t, store.records, 2)
}
