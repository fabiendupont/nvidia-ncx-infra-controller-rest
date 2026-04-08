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

package health

import (
	"net/http"

	echo "github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterRoutes satisfies the provider.APIProvider interface.
// The health provider currently exposes no versioned API routes; the
// /healthz and /readyz system routes are registered separately by the
// core. This method also registers fault event (operator-facing) and
// service event (tenant-facing) routes when the stores are available.
func (p *HealthProvider) RegisterRoutes(group *echo.Group) {
	p.registerFaultRoutes(group)
	p.registerServiceEventRoutes(group)
	p.registerMetricsRoute(group)
}

// registerFaultRoutes adds operator-facing fault management endpoints.
func (p *HealthProvider) registerFaultRoutes(group *echo.Group) {
	if p.faultHandler == nil {
		return
	}

	prefix := p.apiPathPrefix + "/health/events"

	group.Add(http.MethodPost, prefix+"/ingest", p.faultHandler.handleIngestFault)
	group.Add(http.MethodGet, prefix+"/summary", p.faultHandler.handleGetFaultSummary)
	group.Add(http.MethodGet, prefix, p.faultHandler.handleListFaultEvents)
	group.Add(http.MethodGet, prefix+"/:id", p.faultHandler.handleGetFaultEvent)
	group.Add(http.MethodPatch, prefix+"/:id", p.faultHandler.handleUpdateFaultEvent)
	group.Add(http.MethodPost, prefix+"/:id/remediate", p.faultHandler.handleTriggerRemediation)
}

// registerServiceEventRoutes adds tenant-facing service event endpoints.
func (p *HealthProvider) registerServiceEventRoutes(group *echo.Group) {
	if p.serviceEventHandler == nil {
		return
	}

	prefix := p.apiPathPrefix + "/tenant/:tenantId/service-events"

	group.Add(http.MethodGet, prefix, p.serviceEventHandler.handleListServiceEvents)
	group.Add(http.MethodGet, prefix+"/active", p.serviceEventHandler.handleGetActiveServiceEvents)
	group.Add(http.MethodGet, prefix+"/:id", p.serviceEventHandler.handleGetServiceEvent)
}

// registerMetricsRoute exposes fault event Prometheus metrics.
// The metrics endpoint is separate from NICo's global /metrics so that
// fault-specific metrics can be scraped independently.
func (p *HealthProvider) registerMetricsRoute(group *echo.Group) {
	if p.metrics == nil {
		return
	}

	// Refresh gauges before each scrape
	handler := promhttp.HandlerFor(p.metricsRegistry, promhttp.HandlerOpts{})

	group.Add(http.MethodGet, p.apiPathPrefix+"/health/metrics", func(c echo.Context) error {
		p.metrics.RefreshOpenFaults()
		handler.ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
