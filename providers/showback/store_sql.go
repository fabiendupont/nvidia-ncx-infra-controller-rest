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
	"context"
	"time"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
)

// UsageStoreInterface defines the contract for usage record storage.
type UsageStoreInterface interface {
	StartMetering(tenantID, resourceID uuid.UUID, metricName string)
	StopMetering(resourceID uuid.UUID) error
	GetUsageByTenant(tenantID uuid.UUID) UsageSummary
	GetUsageByService(serviceID uuid.UUID) UsageSummary
}

// UsageSQLStore is a PostgreSQL-backed usage store.
type UsageSQLStore struct {
	dao model.UsageRecordDAO
}

// NewUsageSQLStore creates a new SQL-backed usage store.
func NewUsageSQLStore(dbSession *cdb.Session) *UsageSQLStore {
	return &UsageSQLStore{dao: model.NewUsageRecordDAO(dbSession)}
}

// StartMetering creates an open-ended usage record for the given resource.
func (s *UsageSQLStore) StartMetering(tenantID, resourceID uuid.UUID, metricName string) {
	record := &model.UsageRecord{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ResourceID: resourceID,
		MetricName: metricName,
		StartTime:  time.Now(),
	}
	// Best-effort: errors are logged by the DAO tracing
	s.dao.Create(context.Background(), nil, record) //nolint:errcheck // fire-and-forget metering start
}

// StopMetering closes the active usage record for the given resource.
func (s *UsageSQLStore) StopMetering(resourceID uuid.UUID) error {
	record, err := s.dao.GetByResourceID(context.Background(), nil, resourceID)
	if err != nil {
		return err
	}

	now := time.Now()
	record.EndTime = &now
	record.Value = now.Sub(record.StartTime).Hours()

	_, err = s.dao.Update(context.Background(), nil, record)
	return err
}

// GetUsageByTenant returns a usage summary for the given tenant.
func (s *UsageSQLStore) GetUsageByTenant(tenantID uuid.UUID) UsageSummary {
	records, err := s.dao.GetAllByTenant(context.Background(), nil, tenantID)
	if err != nil {
		return UsageSummary{TenantID: tenantID, Period: "current-month", Metrics: map[string]float64{}}
	}

	metrics := make(map[string]float64)
	for _, r := range records {
		val := r.Value
		if r.EndTime == nil {
			val = time.Since(r.StartTime).Hours()
		}
		metrics[r.MetricName] += val
	}

	return UsageSummary{
		TenantID: tenantID,
		Period:   "current-month",
		Metrics:  metrics,
	}
}

// GetUsageByService returns a usage summary for a specific service.
func (s *UsageSQLStore) GetUsageByService(serviceID uuid.UUID) UsageSummary {
	records, err := s.dao.GetAllByService(context.Background(), nil, serviceID)
	if err != nil {
		return UsageSummary{Period: "current-month", Metrics: map[string]float64{}}
	}

	metrics := make(map[string]float64)
	var tenantID uuid.UUID
	for _, r := range records {
		tenantID = r.TenantID
		val := r.Value
		if r.EndTime == nil {
			val = time.Since(r.StartTime).Hours()
		}
		metrics[r.MetricName] += val
	}

	return UsageSummary{
		TenantID: tenantID,
		Period:   "current-month",
		Metrics:  metrics,
	}
}
