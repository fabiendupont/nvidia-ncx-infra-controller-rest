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

// ServiceTemplate defines a service that tenants can order from the catalog.
type ServiceTemplate struct {
	ID          uuid.UUID           `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Version     string              `json:"version"`
	Parameters  []TemplateParameter `json:"parameters"`
	Labels      map[string]string   `json:"labels,omitempty"`
	IsActive    bool                `json:"is_active"`
	Created     time.Time           `json:"created"`
	Updated     time.Time           `json:"updated"`
}

// TemplateParameter describes a configurable parameter within a service template.
type TemplateParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "integer", "string", "boolean"
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Min         *int        `json:"min,omitempty"`
	Max         *int        `json:"max,omitempty"`
}
