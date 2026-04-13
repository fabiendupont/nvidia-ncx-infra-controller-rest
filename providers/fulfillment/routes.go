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

package fulfillment

import (
	"net/http"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/google/uuid"
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

// RegisterRoutes registers all fulfillment-related API routes on the given group.
func (p *FulfillmentProvider) RegisterRoutes(group *echo.Group) {
	prefix := p.apiPathPrefix

	orderHandler := NewOrderHandler(p.orderStore)
	if p.catalog != nil {
		orderHandler.WithBlueprintValidator(func(id uuid.UUID) (string, error) {
			bp, err := p.catalog.BlueprintStore().GetByID(id.String())
			if err != nil {
				return "", err
			}
			return bp.Name, nil
		})
	}
	serviceHandler := NewServiceHandler(p.serviceStore)

	// Order endpoints — any authenticated org member can order and view
	group.Add(http.MethodPost, prefix+"/catalog/orders", withAuth(orderHandler.Create))
	group.Add(http.MethodGet, prefix+"/catalog/orders", withAuth(orderHandler.List))
	group.Add(http.MethodGet, prefix+"/catalog/orders/:id", withAuth(orderHandler.Get))
	group.Add(http.MethodDelete, prefix+"/catalog/orders/:id", withAuth(orderHandler.Cancel))

	// Service endpoints — read for any member, write for tenant admin+
	group.Add(http.MethodGet, prefix+"/services", withAuth(serviceHandler.List))
	group.Add(http.MethodGet, prefix+"/services/:id", withAuth(serviceHandler.Get))
	group.Add(http.MethodPatch, prefix+"/services/:id", withRole(serviceHandler.Update,
		provider.RoleProviderAdmin, provider.RoleTenantAdmin))
	group.Add(http.MethodDelete, prefix+"/services/:id", withRole(serviceHandler.Delete,
		provider.RoleProviderAdmin, provider.RoleTenantAdmin))
}
