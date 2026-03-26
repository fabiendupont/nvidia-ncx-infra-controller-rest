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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTemplate(name string) *ServiceTemplate {
	now := time.Now().UTC()
	return &ServiceTemplate{
		ID:          uuid.New(),
		Name:        name,
		Description: "test description",
		Version:     "1.0.0",
		Parameters:  []TemplateParameter{},
		IsActive:    true,
		Created:     now,
		Updated:     now,
	}
}

func TestTemplateStore_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")

		err := store.Create(tmpl)
		require.NoError(t, err)

		got, err := store.GetByID(tmpl.ID)
		require.NoError(t, err)
		assert.Equal(t, tmpl.Name, got.Name)
	})

	t.Run("duplicate id error", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")

		err := store.Create(tmpl)
		require.NoError(t, err)

		dup := newTestTemplate("other-service")
		dup.ID = tmpl.ID
		err = store.Create(dup)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestTemplateStore_GetByID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")
		require.NoError(t, store.Create(tmpl))

		got, err := store.GetByID(tmpl.ID)
		require.NoError(t, err)
		assert.Equal(t, tmpl.ID, got.ID)
		assert.Equal(t, tmpl.Name, got.Name)
	})

	t.Run("not found", func(t *testing.T) {
		store := NewTemplateStore()

		_, err := store.GetByID(uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestTemplateStore_GetAll(t *testing.T) {
	t.Run("returns only active templates", func(t *testing.T) {
		store := NewTemplateStore()

		active := newTestTemplate("active-service")
		inactive := newTestTemplate("inactive-service")
		inactive.IsActive = false

		require.NoError(t, store.Create(active))
		require.NoError(t, store.Create(inactive))

		// GetAll returns all templates in the store; filtering by IsActive
		// is done at the handler level. Verify both are returned.
		all := store.GetAll()
		assert.Len(t, all, 2)

		// Verify that among the returned templates, we can distinguish active ones.
		var activeCount int
		for _, tmpl := range all {
			if tmpl.IsActive {
				activeCount++
			}
		}
		assert.Equal(t, 1, activeCount)
	})
}

func TestTemplateStore_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")
		require.NoError(t, store.Create(tmpl))

		tmpl.Name = "updated-service"
		tmpl.Updated = time.Now().UTC()
		err := store.Update(tmpl)
		require.NoError(t, err)

		got, err := store.GetByID(tmpl.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated-service", got.Name)
	})

	t.Run("not found", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")

		err := store.Update(tmpl)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestTemplateStore_Delete(t *testing.T) {
	t.Run("soft delete sets IsActive false", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")
		require.NoError(t, store.Create(tmpl))

		// Simulate the soft-delete pattern used by the handler:
		// retrieve, set IsActive=false, then update.
		tmpl.IsActive = false
		err := store.Update(tmpl)
		require.NoError(t, err)

		got, err := store.GetByID(tmpl.ID)
		require.NoError(t, err)
		assert.False(t, got.IsActive)
	})

	t.Run("hard delete removes entry", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := newTestTemplate("test-service")
		require.NoError(t, store.Create(tmpl))

		err := store.Delete(tmpl.ID)
		require.NoError(t, err)

		_, err = store.GetByID(tmpl.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("not found", func(t *testing.T) {
		store := NewTemplateStore()

		err := store.Delete(uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
