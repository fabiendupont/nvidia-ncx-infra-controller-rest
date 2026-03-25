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

package dpfhcp

import (
	"fmt"
	"sync"
)

// ProvisioningStore provides CRUD operations for DPF HCP provisioning records.
// This is an in-memory implementation keyed by site ID.
type ProvisioningStore struct {
	mu      sync.RWMutex
	records map[string]*ProvisioningRecord
}

// NewProvisioningStore creates an empty ProvisioningStore.
func NewProvisioningStore() *ProvisioningStore {
	return &ProvisioningStore{
		records: make(map[string]*ProvisioningRecord),
	}
}

// Create adds a new provisioning record to the store.
func (s *ProvisioningStore) Create(r *ProvisioningRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[r.SiteID]; exists {
		return fmt.Errorf("provisioning record for site %s already exists", r.SiteID)
	}
	s.records[r.SiteID] = r
	return nil
}

// Get retrieves a provisioning record by site ID.
func (s *ProvisioningStore) Get(siteID string) (*ProvisioningRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.records[siteID]
	if !ok {
		return nil, fmt.Errorf("provisioning record for site %s not found", siteID)
	}
	return r, nil
}

// GetBySiteID retrieves a provisioning record by site ID.
// Alias for Get for compatibility.
func (s *ProvisioningStore) GetBySiteID(siteID string) (*ProvisioningRecord, error) {
	return s.Get(siteID)
}

// Update replaces an existing provisioning record in the store.
func (s *ProvisioningStore) Update(r *ProvisioningRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[r.SiteID]; !exists {
		return fmt.Errorf("provisioning record for site %s not found", r.SiteID)
	}
	s.records[r.SiteID] = r
	return nil
}

// Delete removes a provisioning record from the store.
func (s *ProvisioningStore) Delete(siteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[siteID]; !exists {
		return fmt.Errorf("provisioning record for site %s not found", siteID)
	}
	delete(s.records, siteID)
	return nil
}
