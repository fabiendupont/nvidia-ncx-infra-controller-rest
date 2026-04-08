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
	"net/http"

	echo "github.com/labstack/echo/v4"
)

// RegisterRoutes registers all catalog template endpoints on the given Echo group.
func (p *CatalogProvider) RegisterRoutes(group *echo.Group) {
	prefix := p.apiPathPrefix + "/catalog/templates"

	group.Add(http.MethodPost, prefix, handleCreateTemplate(p.store))
	group.Add(http.MethodGet, prefix, handleListTemplates(p.store))
	group.Add(http.MethodGet, prefix+"/:id", handleGetTemplate(p.store))
	group.Add(http.MethodPatch, prefix+"/:id", handleUpdateTemplate(p.store))
	group.Add(http.MethodDelete, prefix+"/:id", handleDeleteTemplate(p.store))

	// Blueprint endpoints (composable service definitions)
	if p.blueprintHandler != nil {
		bp := p.apiPathPrefix + "/catalog/blueprints"
		group.Add(http.MethodPost, bp, p.blueprintHandler.handleCreateBlueprint)
		group.Add(http.MethodGet, bp, p.blueprintHandler.handleListBlueprints)
		group.Add(http.MethodGet, bp+"/:id", p.blueprintHandler.handleGetBlueprint)
		group.Add(http.MethodPatch, bp+"/:id", p.blueprintHandler.handleUpdateBlueprint)
		group.Add(http.MethodDelete, bp+"/:id", p.blueprintHandler.handleDeleteBlueprint)
		group.Add(http.MethodPost, bp+"/:id/validate", p.blueprintHandler.handleValidateBlueprint)
		group.Add(http.MethodGet, p.apiPathPrefix+"/catalog/resource-types", p.blueprintHandler.handleListResourceTypes)
	}
}
