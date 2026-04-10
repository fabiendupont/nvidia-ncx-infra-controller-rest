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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool { return &v }

func TestResolveBlueprint_NoBasedOn(t *testing.T) {
	store := NewBlueprintStore()
	b := &Blueprint{
		Name:    "standalone",
		Version: "1.0.0",
		Parameters: map[string]BlueprintParameter{
			"name": {Name: "name", Type: "string", Required: true},
		},
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc"},
		},
	}
	require.NoError(t, store.Create(b))

	resolved, err := ResolveBlueprint(b, store)
	require.NoError(t, err)
	assert.Equal(t, b.ID, resolved.ID)
	assert.Equal(t, b.Name, resolved.Name)
	assert.Len(t, resolved.Parameters, 1)
	assert.Len(t, resolved.Resources, 1)
}

func TestResolveBlueprint_SimpleVariant(t *testing.T) {
	store := NewBlueprintStore()

	parent := &Blueprint{
		Name:    "gpu-workstation",
		Version: "1.0.0",
		Parameters: map[string]BlueprintParameter{
			"gpu_count":      {Name: "gpu_count", Type: "integer", Required: true, Default: 4},
			"security_group": {Name: "security_group", Type: "string", Required: true},
			"name":           {Name: "name", Type: "string", Required: true},
		},
		Resources: map[string]BlueprintResource{
			"instance": {Type: "nico/instance"},
			"nsg":      {Type: "nico/network-security-group", DependsOn: []string{"instance"}},
		},
	}
	require.NoError(t, store.Create(parent))

	variant := &Blueprint{
		Name:    "corp-workstation",
		Version: "1.0.0",
		BasedOn: parent.ID,
		Parameters: map[string]BlueprintParameter{
			"security_group": {Name: "security_group", Default: "sg-corp", Locked: boolPtr(true)},
			"gpu_count":      {Name: "gpu_count", Default: 8},
		},
	}
	require.NoError(t, store.Create(variant))

	resolved, err := ResolveBlueprint(variant, store)
	require.NoError(t, err)

	// Should have variant's metadata
	assert.Equal(t, variant.ID, resolved.ID)
	assert.Equal(t, "corp-workstation", resolved.Name)

	// Should have parent's resources
	assert.Len(t, resolved.Resources, 2)
	assert.Contains(t, resolved.Resources, "instance")
	assert.Contains(t, resolved.Resources, "nsg")

	// Should have merged parameters
	assert.Len(t, resolved.Parameters, 3)

	// security_group should be locked with overridden default
	sg := resolved.Parameters["security_group"]
	assert.Equal(t, "sg-corp", sg.Default)
	assert.True(t, *sg.Locked)

	// gpu_count should have overridden default but not locked
	gpu := resolved.Parameters["gpu_count"]
	assert.Equal(t, 8, gpu.Default)
	assert.Nil(t, gpu.Locked)

	// name should come from parent unchanged
	name := resolved.Parameters["name"]
	assert.True(t, name.Required)
}

func TestResolveBlueprint_MissingParent(t *testing.T) {
	store := NewBlueprintStore()
	b := &Blueprint{
		Name:    "orphan",
		Version: "1.0.0",
		BasedOn: "nonexistent-id",
	}
	require.NoError(t, store.Create(b))

	_, err := ResolveBlueprint(b, store)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveBlueprint_CircularChain(t *testing.T) {
	store := NewBlueprintStore()

	a := &Blueprint{Name: "a", Version: "1.0.0"}
	require.NoError(t, store.Create(a))

	b := &Blueprint{Name: "b", Version: "1.0.0", BasedOn: a.ID}
	require.NoError(t, store.Create(b))

	// Create circular reference: a -> b
	a.BasedOn = b.ID
	require.NoError(t, store.Update(a))

	_, err := ResolveBlueprint(a, store)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}

func TestFilterUnlockedParameters(t *testing.T) {
	params := map[string]BlueprintParameter{
		"open":   {Name: "open", Type: "string"},
		"locked": {Name: "locked", Type: "string", Default: "fixed", Locked: boolPtr(true)},
		"free":   {Name: "free", Type: "integer", Locked: boolPtr(false)},
	}

	filtered := FilterUnlockedParameters(params)
	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, "open")
	assert.Contains(t, filtered, "free")
	assert.NotContains(t, filtered, "locked")
}

func TestFilterUnlockedParameters_Nil(t *testing.T) {
	assert.Nil(t, FilterUnlockedParameters(nil))
}
