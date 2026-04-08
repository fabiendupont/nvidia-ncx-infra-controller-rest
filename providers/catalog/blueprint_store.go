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
	"time"

	"github.com/google/uuid"
)

// BlueprintStore provides in-memory CRUD for blueprints.
type BlueprintStore struct {
	mu         sync.RWMutex
	blueprints map[string]*Blueprint
}

// NewBlueprintStore creates an empty store.
func NewBlueprintStore() *BlueprintStore {
	return &BlueprintStore{blueprints: make(map[string]*Blueprint)}
}

func (s *BlueprintStore) Create(b *Blueprint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	if _, exists := s.blueprints[b.ID]; exists {
		return fmt.Errorf("blueprint %s already exists", b.ID)
	}
	now := time.Now().UTC()
	b.Created = now
	b.Updated = now
	b.IsActive = true
	stored := *b
	s.blueprints[b.ID] = &stored
	return nil
}

func (s *BlueprintStore) GetByID(id string) (*Blueprint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.blueprints[id]
	if !ok {
		return nil, fmt.Errorf("blueprint %s not found", id)
	}
	copy := *b
	return &copy, nil
}

func (s *BlueprintStore) GetAll() []*Blueprint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Blueprint
	for _, b := range s.blueprints {
		if b.IsActive {
			copy := *b
			result = append(result, &copy)
		}
	}
	return result
}

func (s *BlueprintStore) Update(b *Blueprint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.blueprints[b.ID]; !exists {
		return fmt.Errorf("blueprint %s not found", b.ID)
	}
	b.Updated = time.Now().UTC()
	stored := *b
	s.blueprints[b.ID] = &stored
	return nil
}

func (s *BlueprintStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, exists := s.blueprints[id]
	if !exists {
		return fmt.Errorf("blueprint %s not found", id)
	}
	b.IsActive = false
	b.Updated = time.Now().UTC()
	return nil
}
