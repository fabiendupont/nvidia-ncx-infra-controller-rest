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
	"context"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// CatalogProvider implements the service catalog feature provider.
type CatalogProvider struct {
	blueprintStore   BlueprintStoreInterface
	blueprintHandler *BlueprintHandler
	dbSession        *cdb.Session
	apiPathPrefix    string
}

// New creates a new CatalogProvider.
func New() *CatalogProvider {
	return &CatalogProvider{}
}

func (p *CatalogProvider) Name() string           { return "nico-catalog" }
func (p *CatalogProvider) Version() string        { return "0.1.0" }
func (p *CatalogProvider) Features() []string     { return []string{"catalog"} }
func (p *CatalogProvider) Dependencies() []string { return []string{} }

func (p *CatalogProvider) Init(ctx provider.ProviderContext) error {
	p.apiPathPrefix = ctx.APIPathPrefix

	// Blueprint store: use PostgreSQL if DB is available, else in-memory
	if ctx.DB != nil {
		p.dbSession = ctx.DB
		p.blueprintStore = NewBlueprintSQLStore(ctx.DB)
	} else {
		p.blueprintStore = NewBlueprintStore()
	}

	p.blueprintHandler = NewBlueprintHandler(p.blueprintStore)
	p.LoadSeedData()
	return nil
}

func (p *CatalogProvider) Shutdown(_ context.Context) error {
	return nil
}

// BlueprintStore returns the blueprint store for cross-provider access.
func (p *CatalogProvider) BlueprintStore() BlueprintStoreInterface {
	return p.blueprintStore
}
