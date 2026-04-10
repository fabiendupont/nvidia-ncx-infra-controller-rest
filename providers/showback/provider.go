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

package showback

import (
	"context"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// ShowbackProvider implements the showback feature provider.
type ShowbackProvider struct {
	store         UsageStoreInterface
	dbSession     *cdb.Session
	apiPathPrefix string
}

// New creates a new ShowbackProvider.
func New() *ShowbackProvider {
	return &ShowbackProvider{}
}

func (p *ShowbackProvider) Name() string           { return "nico-showback" }
func (p *ShowbackProvider) Version() string        { return "0.1.0" }
func (p *ShowbackProvider) Features() []string     { return []string{"showback"} }
func (p *ShowbackProvider) Dependencies() []string { return []string{"nico-compute"} }

func (p *ShowbackProvider) Init(ctx provider.ProviderContext) error {
	p.apiPathPrefix = ctx.APIPathPrefix

	// Use PostgreSQL if DB is available, else in-memory
	if ctx.DB != nil {
		p.dbSession = ctx.DB
		p.store = NewUsageSQLStore(ctx.DB)
	} else {
		p.store = NewUsageStore()
	}

	p.registerHooks(ctx.Registry)
	return nil
}

func (p *ShowbackProvider) Shutdown(_ context.Context) error {
	return nil
}
