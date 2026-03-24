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

package networking

import (
	"context"

	tClient "go.temporal.io/sdk/client"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/client/site"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/config"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/networking/networkingsvc"
	sc "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/client/site"
)

// NetworkingProvider implements the networking feature provider.
type NetworkingProvider struct {
	service                *networkingsvc.SQLService
	dbSession              *cdb.Session
	tc                     tClient.Client
	scp                    *site.ClientPool
	cfg                    *config.Config
	apiPathPrefix          string
	temporalNamespace      string
	temporalQueue          string
	workflowSiteClientPool *sc.ClientPool
	hooks                  *provider.HookRunner
}

// New creates a new NetworkingProvider.
func New() *NetworkingProvider {
	return &NetworkingProvider{}
}

func (p *NetworkingProvider) Name() string           { return "nico-networking" }
func (p *NetworkingProvider) Version() string        { return "1.0.6" }
func (p *NetworkingProvider) Features() []string     { return []string{"networking"} }
func (p *NetworkingProvider) Dependencies() []string { return nil }

func (p *NetworkingProvider) Init(ctx provider.ProviderContext) error {
	p.service = networkingsvc.New(ctx.DB)
	p.dbSession = ctx.DB
	p.tc = ctx.Temporal
	p.scp = ctx.SiteClientPool
	p.cfg = ctx.Config.(*config.Config)
	p.apiPathPrefix = ctx.APIPathPrefix
	p.temporalNamespace = ctx.TemporalNamespace
	p.temporalQueue = ctx.TemporalQueue
	if ctx.WorkflowSiteClientPool != nil {
		p.workflowSiteClientPool = ctx.WorkflowSiteClientPool.(*sc.ClientPool)
	}
	p.hooks = ctx.Hooks
	return nil
}

func (p *NetworkingProvider) Shutdown(_ context.Context) error {
	return nil
}

// Service returns the networking service for cross-domain access.
func (p *NetworkingProvider) Service() networkingsvc.Service {
	return p.service
}
