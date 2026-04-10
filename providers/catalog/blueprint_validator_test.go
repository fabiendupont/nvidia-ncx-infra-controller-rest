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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestValidateBlueprint_TenantCannotUseNicoTypes(t *testing.T) {
	tenantID := uuid.New()
	b := &Blueprint{
		Name:     "tenant-bp",
		Version:  "1.0.0",
		TenantID: &tenantID,
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc"},
		},
	}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "tenant blueprints can only reference blueprint/* types")
}

func TestValidateBlueprint_TenantCanUseBlueprintTypes(t *testing.T) {
	tenantID := uuid.New()
	b := &Blueprint{
		Name:     "tenant-bp",
		Version:  "1.0.0",
		TenantID: &tenantID,
		Resources: map[string]BlueprintResource{
			"base": {Type: "blueprint/gpu-slice"},
		},
	}
	result := ValidateBlueprint(b)
	assert.True(t, result.Valid)
}

func TestValidateBlueprint_ProviderCanUseNicoTypes(t *testing.T) {
	b := &Blueprint{
		Name:    "provider-bp",
		Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc"},
		},
	}
	result := ValidateBlueprint(b)
	assert.True(t, result.Valid)
}

func TestValidateBlueprint_InvalidPricingUnit(t *testing.T) {
	b := &Blueprint{
		Name:    "bad-pricing",
		Version: "1.0.0",
		Pricing: &PricingSpec{Rate: 5.0, Unit: "weekly", Currency: "USD"},
	}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "pricing unit must be hour, month, or one-time")
}

func TestValidateBlueprint_ValidPricingUnits(t *testing.T) {
	for _, unit := range []string{"hour", "month", "one-time"} {
		b := &Blueprint{
			Name:    "ok-pricing",
			Version: "1.0.0",
			Pricing: &PricingSpec{Rate: 5.0, Unit: unit, Currency: "USD"},
		}
		result := ValidateBlueprint(b)
		assert.True(t, result.Valid, "unit %q should be valid", unit)
	}
}

func TestValidateBlueprint_ExceedsMaxResourceCount(t *testing.T) {
	resources := make(map[string]BlueprintResource)
	for i := 0; i <= MaxResourceCount; i++ {
		resources[string(rune('a'+i%26))+string(rune('0'+i/26))] = BlueprintResource{Type: "nico/vpc"}
	}
	b := &Blueprint{
		Name:      "too-big",
		Version:   "1.0.0",
		Resources: resources,
	}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "maximum is")
}

func TestValidateBlueprint_MissingNameAndVersion(t *testing.T) {
	b := &Blueprint{}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 2)
}

func TestValidateBlueprint_CircularDependency(t *testing.T) {
	b := &Blueprint{
		Name:    "circular",
		Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"a": {Type: "nico/vpc", DependsOn: []string{"b"}},
			"b": {Type: "nico/subnet", DependsOn: []string{"a"}},
		},
	}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "circular")
}

func TestValidateBlueprint_InvalidDependsOnRef(t *testing.T) {
	b := &Blueprint{
		Name:    "bad-dep",
		Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"vpc": {Type: "nico/vpc", DependsOn: []string{"nonexistent"}},
		},
	}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "does not exist")
}

func TestValidateBlueprint_UnknownResourceType(t *testing.T) {
	b := &Blueprint{
		Name:    "bad-type",
		Version: "1.0.0",
		Resources: map[string]BlueprintResource{
			"x": {Type: "unknown/thing"},
		},
	}
	result := ValidateBlueprint(b)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "unknown type")
}
