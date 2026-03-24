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

package compute

import (
	"context"

	tClient "go.temporal.io/sdk/client"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/client/site"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/config"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/compute/computesvc"
)

// ComputeProvider implements the compute feature provider.
type ComputeProvider struct {
	service       *computesvc.SQLService
	dbSession     *cdb.Session
	tc            tClient.Client
	tnc           tClient.NamespaceClient
	scp           *site.ClientPool
	cfg           *config.Config
	apiPathPrefix string

	temporalNamespace      string
	temporalQueue          string
	workflowSiteClientPool interface{}
	workflowConfig         interface{}
	hooks                  *provider.HookRunner
}

// New creates a new ComputeProvider.
func New() *ComputeProvider {
	return &ComputeProvider{}
}

func (p *ComputeProvider) Name() string           { return "nico-compute" }
func (p *ComputeProvider) Version() string        { return "1.0.6" }
func (p *ComputeProvider) Features() []string     { return []string{"compute"} }
func (p *ComputeProvider) Dependencies() []string { return []string{"nico-networking"} }

func (p *ComputeProvider) Init(ctx provider.ProviderContext) error {
	p.service = computesvc.New(ctx.DB)
	p.dbSession = ctx.DB
	p.tc = ctx.Temporal
	p.tnc = ctx.TemporalNS
	p.scp = ctx.SiteClientPool
	if apiCfg, ok := ctx.Config.(*config.Config); ok {
		p.cfg = apiCfg
	}
	p.apiPathPrefix = ctx.APIPathPrefix
	p.temporalNamespace = ctx.TemporalNamespace
	p.temporalQueue = ctx.TemporalQueue
	p.workflowSiteClientPool = ctx.WorkflowSiteClientPool
	p.workflowConfig = ctx.Config
	p.hooks = ctx.Hooks
	return nil
}

func (p *ComputeProvider) Shutdown(_ context.Context) error {
	return nil
}

// Service returns the compute service for cross-domain access.
func (p *ComputeProvider) Service() computesvc.Service {
	return p.service
}
