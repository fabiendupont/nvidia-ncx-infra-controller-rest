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
)

// ResolveBlueprint returns the effective blueprint after resolving variant
// inheritance. If the blueprint has no BasedOn reference, it is returned as-is.
// Otherwise the parent chain is walked (up to MaxNestingDepth) and parameters
// and resources are merged.
func ResolveBlueprint(b *Blueprint, store BlueprintStoreInterface) (*Blueprint, error) {
	if b.BasedOn == "" {
		return b, nil
	}

	seen := map[string]bool{b.ID: true}
	current := b
	var chain []*Blueprint
	chain = append(chain, current)

	for depth := 0; depth < MaxNestingDepth; depth++ {
		if current.BasedOn == "" {
			break
		}
		parentID := extractBlueprintID(current.BasedOn)
		if seen[parentID] {
			return nil, fmt.Errorf("circular based_on chain detected at blueprint %s", parentID)
		}
		parent, err := store.GetByID(parentID)
		if err != nil {
			return nil, fmt.Errorf("parent blueprint %s not found", parentID)
		}
		seen[parentID] = true
		chain = append(chain, parent)
		current = parent
	}

	if current.BasedOn != "" {
		return nil, fmt.Errorf("based_on chain exceeds maximum depth of %d", MaxNestingDepth)
	}

	root := chain[len(chain)-1]
	resolved := Blueprint{
		ID:          b.ID,
		Name:        b.Name,
		Version:     b.Version,
		Description: b.Description,
		Labels:      b.Labels,
		Pricing:     b.Pricing,
		TenantID:    b.TenantID,
		Visibility:  b.Visibility,
		BasedOn:     b.BasedOn,
		IsActive:    b.IsActive,
		Created:     b.Created,
		Updated:     b.Updated,
	}

	// Resources come from the root ancestor.
	resolved.Resources = make(map[string]BlueprintResource, len(root.Resources))
	for k, v := range root.Resources {
		resolved.Resources[k] = v
	}

	// Parameters: start from root and overlay each descendant in order.
	resolved.Parameters = make(map[string]BlueprintParameter)
	for i := len(chain) - 1; i >= 0; i-- {
		for name, param := range chain[i].Parameters {
			existing, exists := resolved.Parameters[name]
			if !exists {
				resolved.Parameters[name] = param
				continue
			}
			if param.Default != nil {
				existing.Default = param.Default
			}
			if param.Description != "" {
				existing.Description = param.Description
			}
			if param.Enum != nil {
				existing.Enum = param.Enum
			}
			if param.Min != nil {
				existing.Min = param.Min
			}
			if param.Max != nil {
				existing.Max = param.Max
			}
			if param.Locked != nil {
				existing.Locked = param.Locked
			}
			resolved.Parameters[name] = existing
		}
	}

	return &resolved, nil
}

// FilterUnlockedParameters returns a copy of the parameters map with locked
// parameters removed. Used by the resolved endpoint to show only parameters
// that can be changed at order time.
func FilterUnlockedParameters(params map[string]BlueprintParameter) map[string]BlueprintParameter {
	if params == nil {
		return nil
	}
	result := make(map[string]BlueprintParameter, len(params))
	for name, param := range params {
		if param.Locked != nil && *param.Locked {
			continue
		}
		result[name] = param
	}
	return result
}
