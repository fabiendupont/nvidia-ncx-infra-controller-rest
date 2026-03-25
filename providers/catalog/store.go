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

package catalog

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// TemplateStore provides CRUD operations for service templates.
// This is an in-memory implementation; production would use PostgreSQL via MigrationProvider.
type TemplateStore struct {
	mu        sync.RWMutex
	templates map[uuid.UUID]*ServiceTemplate
}

// NewTemplateStore creates an empty TemplateStore.
func NewTemplateStore() *TemplateStore {
	return &TemplateStore{
		templates: make(map[uuid.UUID]*ServiceTemplate),
	}
}

// Create adds a new service template to the store.
func (s *TemplateStore) Create(t *ServiceTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.templates[t.ID]; exists {
		return fmt.Errorf("template with id %s already exists", t.ID)
	}
	s.templates[t.ID] = t
	return nil
}

// GetByID retrieves a service template by its UUID.
func (s *TemplateStore) GetByID(id uuid.UUID) (*ServiceTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.templates[id]
	if !ok {
		return nil, fmt.Errorf("template with id %s not found", id)
	}
	return t, nil
}

// GetAll returns all service templates in the store.
func (s *TemplateStore) GetAll() []*ServiceTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ServiceTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// Update replaces an existing service template in the store.
func (s *TemplateStore) Update(t *ServiceTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.templates[t.ID]; !exists {
		return fmt.Errorf("template with id %s not found", t.ID)
	}
	s.templates[t.ID] = t
	return nil
}

// Delete removes a service template from the store.
func (s *TemplateStore) Delete(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.templates[id]; !exists {
		return fmt.Errorf("template with id %s not found", id)
	}
	delete(s.templates, id)
	return nil
}
