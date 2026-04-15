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
	"encoding/json"
	"fmt"

	"context"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
)

// BlueprintStoreInterface defines the contract for blueprint storage.
// Both in-memory and SQL implementations satisfy this interface.
type BlueprintStoreInterface interface {
	Create(b *Blueprint) error
	GetByID(id string) (*Blueprint, error)
	GetByNameVersion(name, version string) (*Blueprint, error)
	GetAll() []*Blueprint
	Update(b *Blueprint) error
	Delete(id string) error
}

// BlueprintSQLStore is a PostgreSQL-backed blueprint store.
type BlueprintSQLStore struct {
	dao model.BlueprintDAO
}

// NewBlueprintSQLStore creates a new SQL-backed blueprint store.
func NewBlueprintSQLStore(dbSession *cdb.Session) *BlueprintSQLStore {
	return &BlueprintSQLStore{
		dao: model.NewBlueprintDAO(dbSession),
	}
}

// Create adds a new blueprint to the database.
func (s *BlueprintSQLStore) Create(b *Blueprint) error {
	dbModel, err := blueprintToDBModel(b)
	if err != nil {
		return err
	}

	created, err := s.dao.Create(context.Background(), nil, dbModel)
	if err != nil {
		return err
	}

	// Update the blueprint with the generated ID and timestamps
	b.ID = created.ID.String()
	b.Created = created.Created
	b.Updated = created.Updated
	b.IsActive = created.IsActive
	return nil
}

// GetByID retrieves a blueprint by its ID.
func (s *BlueprintSQLStore) GetByID(id string) (*Blueprint, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("blueprint %s not found", id)
	}

	dbModel, err := s.dao.GetByID(context.Background(), nil, uid)
	if err != nil {
		return nil, fmt.Errorf("blueprint %s not found", id)
	}

	return dbModelToBlueprint(dbModel)
}

// GetByNameVersion retrieves a blueprint by name and optional version.
func (s *BlueprintSQLStore) GetByNameVersion(name, version string) (*Blueprint, error) {
	dbModel, err := s.dao.GetByNameVersion(context.Background(), nil, name, version)
	if err != nil {
		if version != "" {
			return nil, fmt.Errorf("blueprint %s@%s not found", name, version)
		}
		return nil, fmt.Errorf("blueprint %s not found", name)
	}
	return dbModelToBlueprint(dbModel)
}

// GetAll returns all active blueprints.
func (s *BlueprintSQLStore) GetAll() []*Blueprint {
	isActive := true
	dbModels, err := s.dao.GetAll(context.Background(), nil, nil, nil, &isActive)
	if err != nil {
		return nil
	}

	var result []*Blueprint
	for i := range dbModels {
		bp, err := dbModelToBlueprint(&dbModels[i])
		if err != nil {
			continue
		}
		result = append(result, bp)
	}
	return result
}

// Update replaces an existing blueprint in the database.
func (s *BlueprintSQLStore) Update(b *Blueprint) error {
	dbModel, err := blueprintToDBModel(b)
	if err != nil {
		return err
	}

	updated, err := s.dao.Update(context.Background(), nil, dbModel)
	if err != nil {
		return err
	}

	b.Updated = updated.Updated
	return nil
}

// Delete soft-deletes a blueprint by ID.
func (s *BlueprintSQLStore) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("blueprint %s not found", id)
	}

	return s.dao.DeleteByID(context.Background(), nil, uid)
}

// blueprintToDBModel converts a provider Blueprint to a DB model Blueprint.
func blueprintToDBModel(b *Blueprint) (*model.Blueprint, error) {
	uid := uuid.Nil
	if b.ID != "" {
		var err error
		uid, err = uuid.Parse(b.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid blueprint ID: %s", b.ID)
		}
	}

	// Convert typed maps to generic interface{} maps for JSONB storage
	params, err := toJSONMap(b.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters: %w", err)
	}
	resources, err := toJSONMap(b.Resources)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize resources: %w", err)
	}
	pricing, err := toJSONMap(b.Pricing)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize pricing: %w", err)
	}

	visibility := b.Visibility
	if visibility == "" {
		visibility = VisibilityPublic
	}

	var basedOn *string
	if b.BasedOn != "" {
		basedOn = &b.BasedOn
	}

	return &model.Blueprint{
		ID:          uid,
		Name:        b.Name,
		Version:     b.Version,
		Description: b.Description,
		Parameters:  params,
		Resources:   resources,
		Labels:      b.Labels,
		Pricing:     pricing,
		TenantID:    b.TenantID,
		Visibility:  visibility,
		BasedOn:     basedOn,
		IsActive:    b.IsActive,
		Created:     b.Created,
		Updated:     b.Updated,
	}, nil
}

// dbModelToBlueprint converts a DB model Blueprint to a provider Blueprint.
func dbModelToBlueprint(m *model.Blueprint) (*Blueprint, error) {
	params, err := fromJSONMap[BlueprintParameter](m.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize parameters: %w", err)
	}
	resources, err := fromJSONMap[BlueprintResource](m.Resources)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize resources: %w", err)
	}

	var pricing *PricingSpec
	if m.Pricing != nil {
		p, err := fromJSONValue[PricingSpec](m.Pricing)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize pricing: %w", err)
		}
		pricing = p
	}

	var basedOn string
	if m.BasedOn != nil {
		basedOn = *m.BasedOn
	}

	return &Blueprint{
		ID:          m.ID.String(),
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Parameters:  params,
		Resources:   resources,
		Labels:      m.Labels,
		Pricing:     pricing,
		TenantID:    m.TenantID,
		Visibility:  m.Visibility,
		BasedOn:     basedOn,
		IsActive:    m.IsActive,
		Created:     m.Created,
		Updated:     m.Updated,
	}, nil
}

// fromJSONValue converts a generic map[string]interface{} to a typed value via JSON round-trip.
func fromJSONValue[T any](m map[string]interface{}) (*T, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// toJSONMap converts a typed map to a generic map[string]interface{} via JSON round-trip.
func toJSONMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// fromJSONMap converts a generic map[string]interface{} back to a typed map via JSON round-trip.
func fromJSONMap[T any](m map[string]interface{}) (map[string]T, error) {
	if m == nil {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var result map[string]T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
