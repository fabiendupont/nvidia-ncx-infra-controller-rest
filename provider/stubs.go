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

package provider

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// stubRoutes maps feature names to route prefix patterns that should return
// 501 when no provider is registered.
var stubRoutes = map[string][]string{
	"catalog":     {"/catalog/*"},
	"fulfillment": {"/catalog/orders/*", "/services/*"},
	"showback":    {"/self/usage", "/self/quotas", "/services/:id/usage"},
	"storage":     {"/:orgName/*/storage/*"},
	"dcim":        {"/dcim/*"},
	"dpf-hcp":     {"/sites/:siteId/dpf-hcp", "/sites/:siteId/dpf-hcp/status"},
}

// RegisterStubs registers 501 handlers for features that have no provider.
func RegisterStubs(group *echo.Group, registry *Registry) {
	for feature, routes := range stubRoutes {
		if _, ok := registry.FeatureProvider(feature); ok {
			continue
		}
		for _, route := range routes {
			f := feature // capture for closure
			handler := func(c echo.Context) error {
				return c.JSON(http.StatusNotImplemented, map[string]string{
					"error":   "not_implemented",
					"feature": f,
					"message": fmt.Sprintf("Feature '%s' has no provider configured.", f),
				})
			}
			group.Any(route, handler)
		}
	}
}
