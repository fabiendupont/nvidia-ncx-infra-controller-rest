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
	"time"

	"github.com/google/uuid"
)

// Blueprint visibility levels control who can see and order from a blueprint.
const (
	VisibilityPublic       = "public"       // All tenants can see and order
	VisibilityOrganization = "organization" // Same tenant/org only
	VisibilityPrivate      = "private"      // Author only
)

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
	Locked      *bool       `json:"locked,omitempty" yaml:"locked,omitempty"`
}

// BlueprintResource declares a single resource in the blueprint's DAG.
type BlueprintResource struct {
	Type       string                 `json:"type" yaml:"type"`
	DependsOn  []string               `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Condition  string                 `json:"condition,omitempty" yaml:"condition,omitempty"`
	Count      string                 `json:"count,omitempty" yaml:"count,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// PricingSpec describes the cost of ordering and running a blueprint.
type PricingSpec struct {
	Rate            float64 `json:"rate" yaml:"rate"`
	Unit            string  `json:"unit" yaml:"unit"`                                             // "hour", "month", "one-time"
	Currency        string  `json:"currency" yaml:"currency"`                                     // ISO 4217 (e.g., "USD")
	BillingInterval *int    `json:"billing_interval,omitempty" yaml:"billing_interval,omitempty"` // seconds between billing ticks
}

// Blueprint is a composable, DAG-based service definition.
// Every catalog item — atomic or composed — is a blueprint.
type Blueprint struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name" yaml:"name"`
	Version     string                        `json:"version" yaml:"version"`
	Description string                        `json:"description" yaml:"description"`
	Parameters  map[string]BlueprintParameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Resources   map[string]BlueprintResource  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Labels      map[string]string             `json:"labels,omitempty" yaml:"labels,omitempty"`
	Pricing     *PricingSpec                  `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	TenantID    *uuid.UUID                    `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	Visibility  string                        `json:"visibility" yaml:"visibility"`
	BasedOn     string                        `json:"based_on,omitempty" yaml:"based_on,omitempty"`
	IsActive    bool                          `json:"is_active"`
	Created     time.Time                     `json:"created"`
	Updated     time.Time                     `json:"updated"`
}

// IsTenantOwned returns true if the blueprint belongs to a specific tenant.
func (b *Blueprint) IsTenantOwned() bool {
	return b.TenantID != nil
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

// MaxNestingDepth is the maximum allowed depth for nested blueprint references.
const MaxNestingDepth = 5

// MaxResourceCount is the maximum total resources allowed after DAG expansion.
const MaxResourceCount = 100
