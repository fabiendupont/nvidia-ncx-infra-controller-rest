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
	"sync"
	"time"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// FaultEventStore — in-memory store for fault events
// ---------------------------------------------------------------------------

// FaultEventStore is a thread-safe in-memory store for fault events.
// The db session is accepted for forward-compatibility with persistent
// storage but is not used by the in-memory implementation.
type FaultEventStore struct {
	mu     sync.RWMutex
	events map[string]*FaultEvent
	db     *cdb.Session
}

// FaultStore is an alias for FaultEventStore used by the provider struct.
type FaultStore = FaultEventStore

// NewFaultEventStore creates an empty FaultEventStore.
func NewFaultEventStore(db *cdb.Session) *FaultEventStore {
	return &FaultEventStore{
		events: make(map[string]*FaultEvent),
		db:     db,
	}
}

// NewFaultStore creates an empty FaultStore (alias for NewFaultEventStore).
func NewFaultStore(db *cdb.Session) *FaultStore {
	return NewFaultEventStore(db)
}

// Create inserts a new FaultEvent into the store.
func (s *FaultEventStore) Create(event *FaultEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	event.UpdatedAt = now
	if event.State == "" {
		event.State = FaultStateOpen
	}

	stored := *event
	s.events[stored.ID] = &stored
	return nil
}

// GetByID returns a fault event by ID or an error if not found.
func (s *FaultEventStore) GetByID(id string) (*FaultEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return nil, fmt.Errorf("fault event not found: %s", id)
	}
	out := *event
	return &out, nil
}

// GetAll returns fault events matching the provided filter.
func (s *FaultEventStore) GetAll(filter FaultEventFilter) []*FaultEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*FaultEvent
	for _, event := range s.events {
		if !matchesFaultFilter(event, filter) {
			continue
		}
		out := *event
		result = append(result, &out)
	}

	return result
}

// Update replaces a fault event in the store and returns the updated copy.
// The event must already exist.
func (s *FaultEventStore) Update(event *FaultEvent) (*FaultEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.events[event.ID]; !ok {
		return nil, fmt.Errorf("fault event not found: %s", event.ID)
	}

	event.UpdatedAt = time.Now().UTC()
	stored := *event
	s.events[stored.ID] = &stored
	out := stored
	return &out, nil
}

// Delete removes a fault event by ID.
func (s *FaultEventStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.events[id]; !ok {
		return fmt.Errorf("fault event not found: %s", id)
	}
	delete(s.events, id)
	return nil
}

// GetSummary computes an aggregated FaultSummary across all stored events.
func (s *FaultEventStore) GetSummary() *FaultSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &FaultSummary{
		BySeverity:  make(map[string]int),
		ByComponent: make(map[string]int),
		ByState:     make(map[string]int),
	}

	siteOpen := make(map[string]int)
	siteCritical := make(map[string]int)

	for _, event := range s.events {
		summary.BySeverity[event.Severity]++
		summary.ByComponent[event.Component]++
		summary.ByState[event.State]++

		if event.State == FaultStateOpen {
			siteOpen[event.SiteID]++
		}
		if event.Severity == SeverityCritical {
			siteCritical[event.SiteID]++
		}
	}

	siteIDs := make(map[string]struct{})
	for id := range siteOpen {
		siteIDs[id] = struct{}{}
	}
	for id := range siteCritical {
		siteIDs[id] = struct{}{}
	}

	for siteID := range siteIDs {
		summary.BySite = append(summary.BySite, SiteFaultSummary{
			SiteID:   siteID,
			Open:     siteOpen[siteID],
			Critical: siteCritical[siteID],
		})
	}

	return summary
}

// ResolveMachineContext looks up the site, tenant, and instance associated
// with a machine. In the in-memory implementation this always returns an
// error because there is no backing database; the persistent implementation
// will query the machine table with bun relations.
func (s *FaultEventStore) ResolveMachineContext(machineID string) (*MachineContext, error) {
	return nil, fmt.Errorf("machine context resolution not available in-memory for machine %s", machineID)
}

// ListOpenCriticalByMachine returns open fault events with critical severity
// for a given machine.
func (s *FaultEventStore) ListOpenCriticalByMachine(_ context.Context, machineID string) ([]*FaultEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*FaultEvent
	for _, event := range s.events {
		if event.MachineID == nil || *event.MachineID != machineID {
			continue
		}
		if event.State != FaultStateOpen {
			continue
		}
		if event.Severity != SeverityCritical {
			continue
		}
		out := *event
		result = append(result, &out)
	}
	return result, nil
}

// matchesFaultFilter returns true if the event satisfies all non-nil/non-empty
// filter fields.
func matchesFaultFilter(event *FaultEvent, f FaultEventFilter) bool {
	if f.SiteID != nil && event.SiteID != *f.SiteID {
		return false
	}
	if f.MachineID != nil && (event.MachineID == nil || *event.MachineID != *f.MachineID) {
		return false
	}
	if len(f.Severity) > 0 && !contains(f.Severity, event.Severity) {
		return false
	}
	if len(f.Component) > 0 && !contains(f.Component, event.Component) {
		return false
	}
	if len(f.State) > 0 && !contains(f.State, event.State) {
		return false
	}
	if f.Source != nil && event.Source != *f.Source {
		return false
	}
	return true
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ServiceEventStore — in-memory store for service events
// ---------------------------------------------------------------------------

// ServiceEventStore is a thread-safe in-memory store for service events.
type ServiceEventStore struct {
	mu     sync.RWMutex
	events map[string]*ServiceEvent
	db     *cdb.Session
}

// NewServiceEventStore creates an empty ServiceEventStore.
func NewServiceEventStore(db *cdb.Session) *ServiceEventStore {
	return &ServiceEventStore{
		events: make(map[string]*ServiceEvent),
		db:     db,
	}
}

// Create inserts a new ServiceEvent, assigning an ID and timestamps.
// Returns the stored copy.
func (s *ServiceEventStore) Create(event *ServiceEvent) (*ServiceEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	event.UpdatedAt = now
	if event.State == "" {
		event.State = "active"
	}

	stored := *event
	s.events[stored.ID] = &stored
	out := stored
	return &out, nil
}

// GetByID returns a service event by ID or an error if not found.
func (s *ServiceEventStore) GetByID(id string) (*ServiceEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return nil, fmt.Errorf("service event not found: %s", id)
	}
	out := *event
	return &out, nil
}

// GetByTenantID returns all service events for a given tenant.
func (s *ServiceEventStore) GetByTenantID(tenantID string) []*ServiceEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ServiceEvent
	for _, event := range s.events {
		if event.TenantID == tenantID {
			out := *event
			result = append(result, &out)
		}
	}
	return result
}

// Update replaces a service event in the store and returns the updated copy.
// The event must already exist.
func (s *ServiceEventStore) Update(event *ServiceEvent) (*ServiceEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.events[event.ID]; !ok {
		return nil, fmt.Errorf("service event not found: %s", event.ID)
	}

	event.UpdatedAt = time.Now().UTC()
	stored := *event
	s.events[stored.ID] = &stored
	out := stored
	return &out, nil
}

// ---------------------------------------------------------------------------
// FaultServiceEventStore — in-memory join table
// ---------------------------------------------------------------------------

// FaultServiceEventStore is a thread-safe in-memory store for the
// many-to-many relationship between fault events and service events.
type FaultServiceEventStore struct {
	mu             sync.RWMutex
	faultToService map[string]map[string]struct{}
	serviceToFault map[string]map[string]struct{}
	db             *cdb.Session
}

// NewFaultServiceEventStore creates an empty FaultServiceEventStore.
func NewFaultServiceEventStore(db *cdb.Session) *FaultServiceEventStore {
	return &FaultServiceEventStore{
		faultToService: make(map[string]map[string]struct{}),
		serviceToFault: make(map[string]map[string]struct{}),
		db:             db,
	}
}

// Link creates a bidirectional association between a fault event and a
// service event.
func (s *FaultServiceEventStore) Link(faultID, serviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.faultToService[faultID] == nil {
		s.faultToService[faultID] = make(map[string]struct{})
	}
	s.faultToService[faultID][serviceID] = struct{}{}

	if s.serviceToFault[serviceID] == nil {
		s.serviceToFault[serviceID] = make(map[string]struct{})
	}
	s.serviceToFault[serviceID][faultID] = struct{}{}
}

// GetServiceEventsByFaultID returns the service event IDs linked to a fault.
func (s *FaultServiceEventStore) GetServiceEventsByFaultID(faultID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.faultToService[faultID]))
	for id := range s.faultToService[faultID] {
		ids = append(ids, id)
	}
	return ids
}

// GetFaultEventsByServiceID returns the fault event IDs linked to a service
// event.
func (s *FaultServiceEventStore) GetFaultEventsByServiceID(serviceID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.serviceToFault[serviceID]))
	for id := range s.serviceToFault[serviceID] {
		ids = append(ids, id)
	}
	return ids
}

// ---------------------------------------------------------------------------
// ClassificationStore — in-memory store for classification mappings
// ---------------------------------------------------------------------------

// ClassificationStore is a thread-safe in-memory store for classification-
// to-remediation mappings.
type ClassificationStore struct {
	mu       sync.RWMutex
	mappings map[string]*ClassificationMapping
}

// NewClassificationStore creates a ClassificationStore seeded with the
// provided default mappings.
func NewClassificationStore(defaults map[string]*ClassificationMapping) *ClassificationStore {
	m := make(map[string]*ClassificationMapping, len(defaults))
	for k, v := range defaults {
		stored := *v
		m[k] = &stored
	}
	return &ClassificationStore{
		mappings: m,
	}
}

// Get returns the mapping for a classification or an error if not found.
func (s *ClassificationStore) Get(classification string) (*ClassificationMapping, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.mappings[classification]
	if !ok {
		return nil, fmt.Errorf("classification not found: %s", classification)
	}
	out := *m
	return &out, nil
}

// Set stores or replaces a classification mapping.
func (s *ClassificationStore) Set(classification string, mapping *ClassificationMapping) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := *mapping
	s.mappings[classification] = &stored
}

// GetAll returns all classification mappings.
func (s *ClassificationStore) GetAll() []*ClassificationMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ClassificationMapping, 0, len(s.mappings))
	for _, m := range s.mappings {
		out := *m
		result = append(result, &out)
	}
	return result
}
