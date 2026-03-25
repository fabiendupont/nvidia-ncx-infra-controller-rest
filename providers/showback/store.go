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
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// UsageStore is an in-memory store for usage records.
type UsageStore struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*UsageRecord // keyed by record ID
	byRes   map[uuid.UUID]uuid.UUID    // resourceID → active record ID
}

// NewUsageStore creates a new in-memory usage store.
func NewUsageStore() *UsageStore {
	return &UsageStore{
		records: make(map[uuid.UUID]*UsageRecord),
		byRes:   make(map[uuid.UUID]uuid.UUID),
	}
}

// StartMetering creates an open-ended usage record for the given resource.
func (s *UsageStore) StartMetering(tenantID, resourceID uuid.UUID, metricName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New()
	record := &UsageRecord{
		ID:         id,
		TenantID:   tenantID,
		ServiceID:  uuid.Nil,
		ResourceID: resourceID,
		MetricName: metricName,
		StartTime:  time.Now(),
	}
	s.records[id] = record
	s.byRes[resourceID] = id
}

// StopMetering closes the active usage record for the given resource by
// setting its EndTime and computing the elapsed value in hours.
func (s *UsageStore) StopMetering(resourceID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recID, ok := s.byRes[resourceID]
	if !ok {
		return fmt.Errorf("no active metering record for resource %s", resourceID)
	}

	record := s.records[recID]
	now := time.Now()
	record.EndTime = &now
	record.Value = now.Sub(record.StartTime).Hours()
	delete(s.byRes, resourceID)
	return nil
}

// GetUsageByTenant returns a usage summary for the given tenant.
func (s *UsageStore) GetUsageByTenant(tenantID uuid.UUID) UsageSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make(map[string]float64)
	for _, record := range s.records {
		if record.TenantID != tenantID {
			continue
		}
		val := record.Value
		if record.EndTime == nil {
			val = time.Since(record.StartTime).Hours()
		}
		metrics[record.MetricName] += val
	}
	return UsageSummary{
		TenantID: tenantID,
		Period:   "current-month",
		Metrics:  metrics,
	}
}

// GetUsageByService returns a usage summary for a specific service.
func (s *UsageStore) GetUsageByService(serviceID uuid.UUID) UsageSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make(map[string]float64)
	var tenantID uuid.UUID
	for _, record := range s.records {
		if record.ServiceID != serviceID {
			continue
		}
		tenantID = record.TenantID
		val := record.Value
		if record.EndTime == nil {
			val = time.Since(record.StartTime).Hours()
		}
		metrics[record.MetricName] += val
	}
	return UsageSummary{
		TenantID: tenantID,
		Period:   "current-month",
		Metrics:  metrics,
	}
}
