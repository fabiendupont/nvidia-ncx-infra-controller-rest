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

package dpfhcp

import (
	"context"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// DPFHCPProvider implements the DPF hosted control plane feature provider.
type DPFHCPProvider struct {
	store         *ProvisioningStore
	apiPathPrefix string
	k8sClient     *DPFHCPClient
}


// DPFHCPActivities holds the dependencies needed by DPF HCP workflow activities.
type DPFHCPActivities struct {
	store  *ProvisioningStore
	client *DPFHCPClient
}

// New creates a new DPFHCPProvider.
func New() *DPFHCPProvider {
	return &DPFHCPProvider{}
}

func (p *DPFHCPProvider) Name() string           { return "nico-dpfhcp" }
func (p *DPFHCPProvider) Version() string        { return "0.1.0" }
func (p *DPFHCPProvider) Features() []string     { return []string{"dpf-hcp"} }
func (p *DPFHCPProvider) Dependencies() []string { return []string{"nico-site"} }

func (p *DPFHCPProvider) Init(ctx provider.ProviderContext) error {
	p.store = NewProvisioningStore()
	p.apiPathPrefix = ctx.APIPathPrefix
	return nil
}

func (p *DPFHCPProvider) Shutdown(_ context.Context) error {
	return nil
}
