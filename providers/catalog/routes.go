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

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	echo "github.com/labstack/echo/v4"
)

// withRole wraps a handler with role-based access control.
func withRole(handler echo.HandlerFunc, roles ...string) echo.HandlerFunc {
	mw := provider.RequireRole(roles...)
	return mw(handler)
}

// withAuth wraps a handler with authentication check.
func withAuth(handler echo.HandlerFunc) echo.HandlerFunc {
	mw := provider.RequireAuth()
	return mw(handler)
}

// RegisterRoutes registers all catalog blueprint endpoints on the given Echo group.
func (p *CatalogProvider) RegisterRoutes(group *echo.Group) {
	bp := p.apiPathPrefix + "/catalog/blueprints"

	// Read endpoints — any authenticated org member
	group.Add(http.MethodGet, bp, withAuth(p.blueprintHandler.handleListBlueprints))
	group.Add(http.MethodGet, bp+"/:id", withAuth(p.blueprintHandler.handleGetBlueprint))
	group.Add(http.MethodGet, bp+"/:id/resolved", withAuth(p.blueprintHandler.handleResolvedBlueprint))
	group.Add(http.MethodPost, bp+"/:id/estimate", withAuth(p.blueprintHandler.handleEstimateCost))
	group.Add(http.MethodPost, bp+"/:id/validate", withAuth(p.blueprintHandler.handleValidateBlueprint))
	group.Add(http.MethodGet, p.apiPathPrefix+"/catalog/resource-types", withAuth(p.blueprintHandler.handleListResourceTypes))

	// Write endpoints — require blueprint author or provider admin
	group.Add(http.MethodPost, bp, withRole(p.blueprintHandler.handleCreateBlueprint,
		provider.RoleProviderAdmin, provider.RoleTenantAdmin, provider.RoleBlueprintAuthor))
	group.Add(http.MethodPatch, bp+"/:id", withRole(p.blueprintHandler.handleUpdateBlueprint,
		provider.RoleProviderAdmin, provider.RoleTenantAdmin, provider.RoleBlueprintAuthor))

	// Delete — provider admin only
	group.Add(http.MethodDelete, bp+"/:id", withRole(p.blueprintHandler.handleDeleteBlueprint,
		provider.RoleProviderAdmin))
}
