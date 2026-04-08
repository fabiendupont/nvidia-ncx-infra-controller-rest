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

import "time"

// BlueprintParameter describes a configurable parameter within a blueprint.
type BlueprintParameter struct {
	Name        string      `json:"name" yaml:"name"`
	Type        string      `json:"type" yaml:"type"`
	Description string      `json:"description" yaml:"description"`
	Required    bool        `json:"required" yaml:"required"`
	Default     interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty" yaml:"enum,omitempty"`
	Min         *int        `json:"min,omitempty" yaml:"min,omitempty"`
	Max         *int        `json:"max,omitempty" yaml:"max,omitempty"`
}

// BlueprintResource declares a single resource in the blueprint's DAG.
type BlueprintResource struct {
	Type       string                 `json:"type" yaml:"type"`
	DependsOn  []string               `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Condition  string                 `json:"condition,omitempty" yaml:"condition,omitempty"`
	Count      string                 `json:"count,omitempty" yaml:"count,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// Blueprint is a composable, DAG-based service definition.
type Blueprint struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name" yaml:"name"`
	Version     string                        `json:"version" yaml:"version"`
	Description string                        `json:"description" yaml:"description"`
	Parameters  map[string]BlueprintParameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Resources   map[string]BlueprintResource  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Labels      map[string]string             `json:"labels,omitempty" yaml:"labels,omitempty"`
	IsActive    bool                          `json:"is_active"`
	Created     time.Time                     `json:"created"`
	Updated     time.Time                     `json:"updated"`
}

// AvailableResourceTypes returns the NICo resource types blueprints can reference.
var AvailableResourceTypes = []string{
	"nico/vpc",
	"nico/subnet",
	"nico/instance",
	"nico/allocation",
	"nico/network-security-group",
	"nico/vpc-peering",
	"nico/infiniband-partition",
	"nico/nvlink-partition",
	"nico/ip-block",
	"nico/ssh-key-group",
	"nico/operating-system",
	"nico/site",
}
