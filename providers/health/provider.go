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
	"context"

	"github.com/prometheus/client_golang/prometheus"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// HealthProvider implements the health and fault-management feature provider.
type HealthProvider struct {
	dbSession     *cdb.Session
	apiPathPrefix string

	// Stores
	faultStore             *FaultStore
	serviceEventStore      *ServiceEventStore
	faultServiceEventStore *FaultServiceEventStore
	classificationStore    *ClassificationStore

	// Handlers
	faultHandler        *FaultHandler
	serviceEventHandler *ServiceEventHandler

	// Metrics
	metrics         *FaultMetrics
	metricsRegistry *prometheus.Registry
}

// New creates a new HealthProvider.
func New() *HealthProvider {
	return &HealthProvider{}
}

func (p *HealthProvider) Name() string           { return "nico-health" }
func (p *HealthProvider) Version() string        { return "2.0.0" }
func (p *HealthProvider) Features() []string     { return []string{"health", "fault-management"} }
func (p *HealthProvider) Dependencies() []string { return []string{"nico-compute"} }

func (p *HealthProvider) Init(ctx provider.ProviderContext) error {
	p.dbSession = ctx.DB
	p.apiPathPrefix = ctx.APIPathPrefix

	// Create stores
	p.faultStore = NewFaultStore(ctx.DB)
	p.serviceEventStore = NewServiceEventStore(ctx.DB)
	p.faultServiceEventStore = NewFaultServiceEventStore(ctx.DB)
	p.classificationStore = NewClassificationStore(loadDefaultClassifications())

	// Create handlers
	p.faultHandler = NewFaultHandler(p.faultStore, p.classificationStore)
	p.serviceEventHandler = NewServiceEventHandler(p.serviceEventStore, p.faultServiceEventStore)

	// Create Prometheus metrics
	p.metricsRegistry = prometheus.NewRegistry()
	p.metrics = NewFaultMetrics(p.metricsRegistry, p.faultStore)

	// Register hooks for fault management integration
	p.registerHooks(ctx.Registry)

	return nil
}

func (p *HealthProvider) Shutdown(_ context.Context) error {
	return nil
}
