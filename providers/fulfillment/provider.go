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
	"context"
	"fmt"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/compute"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/networking"
)

// FulfillmentProvider implements the fulfillment feature provider.
type FulfillmentProvider struct {
	orderStore    *OrderStore
	serviceStore  *ServiceStore
	apiPathPrefix string
	networking    *networking.NetworkingProvider
	compute       *compute.ComputeProvider
}

// New creates a new FulfillmentProvider.
func New() *FulfillmentProvider {
	return &FulfillmentProvider{}
}

func (p *FulfillmentProvider) Name() string       { return "nico-fulfillment" }
func (p *FulfillmentProvider) Version() string     { return "0.1.0" }
func (p *FulfillmentProvider) Features() []string  { return []string{"fulfillment"} }
func (p *FulfillmentProvider) Dependencies() []string {
	return []string{"nico-networking", "nico-compute", "nico-catalog"}
}

func (p *FulfillmentProvider) Init(ctx provider.ProviderContext) error {
	p.orderStore = NewOrderStore()
	p.serviceStore = NewServiceStore()
	p.apiPathPrefix = ctx.APIPathPrefix

	if ctx.Registry != nil {
		if np, ok := ctx.Registry.Get("nico-networking"); ok {
			netProvider, ok := np.(*networking.NetworkingProvider)
			if !ok {
				return fmt.Errorf("nico-networking provider has unexpected type")
			}
			p.networking = netProvider
		}

		if cp, ok := ctx.Registry.Get("nico-compute"); ok {
			compProvider, ok := cp.(*compute.ComputeProvider)
			if !ok {
				return fmt.Errorf("nico-compute provider has unexpected type")
			}
			p.compute = compProvider
		}
	}

	return nil
}

func (p *FulfillmentProvider) Shutdown(_ context.Context) error {
	return nil
}
