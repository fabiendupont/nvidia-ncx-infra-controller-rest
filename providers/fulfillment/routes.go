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

	echo "github.com/labstack/echo/v4"
)

// RegisterRoutes registers all fulfillment-related API routes on the given group.
func (p *FulfillmentProvider) RegisterRoutes(group *echo.Group) {
	prefix := p.apiPathPrefix

	orderHandler := NewOrderHandler(p.orderStore)
	serviceHandler := NewServiceHandler(p.serviceStore)

	// Order endpoints
	group.Add(http.MethodPost, prefix+"/catalog/orders", orderHandler.Create)
	group.Add(http.MethodGet, prefix+"/catalog/orders/:id", orderHandler.Get)
	group.Add(http.MethodDelete, prefix+"/catalog/orders/:id", orderHandler.Cancel)

	// Service endpoints
	group.Add(http.MethodGet, prefix+"/services", serviceHandler.List)
	group.Add(http.MethodGet, prefix+"/services/:id", serviceHandler.Get)
	group.Add(http.MethodPatch, prefix+"/services/:id", serviceHandler.Update)
	group.Add(http.MethodDelete, prefix+"/services/:id", serviceHandler.Delete)
}
