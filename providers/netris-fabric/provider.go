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

package netrisfabric

import (
	"context"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/netris-fabric/client"
)

// NetrisFabricProvider manages physical network fabric via Netris Controller.
// It reacts to NICo networking events (VPC/Subnet/Instance lifecycle) and
// syncs the physical switch configuration accordingly.
//
// This provider is complementary to nico-networking, not a replacement.
// Netris manages the physical switch fabric. NICo manages tenant-level
// networking constructs. This provider syncs NICo events to the Netris
// Controller so the physical fabric matches tenant intent.
type NetrisFabricProvider struct {
	netrisURL  string
	netrisUser string
	netrisPass string
	client     *client.Client
	vpcIDs     *idMap // NICo VPC UUID → Netris VPC int ID
	subnetIDs  *idMap // NICo Subnet UUID → Netris VNET int ID
}

// New creates a new NetrisFabricProvider with explicit credentials.
func New(url, user, pass string) *NetrisFabricProvider {
	return &NetrisFabricProvider{
		netrisURL:  url,
		netrisUser: user,
		netrisPass: pass,
		vpcIDs:     newIDMap(),
		subnetIDs:  newIDMap(),
	}
}

// NewFromEnv creates a new NetrisFabricProvider reading credentials from
// environment variables NETRIS_URL, NETRIS_USERNAME, and NETRIS_PASSWORD.
func NewFromEnv() *NetrisFabricProvider {
	return New(
		os.Getenv("NETRIS_URL"),
		os.Getenv("NETRIS_USERNAME"),
		os.Getenv("NETRIS_PASSWORD"),
	)
}

func (p *NetrisFabricProvider) Name() string           { return "netris-fabric" }
func (p *NetrisFabricProvider) Version() string        { return "0.1.0" }
func (p *NetrisFabricProvider) Features() []string     { return []string{"fabric"} }
func (p *NetrisFabricProvider) Dependencies() []string { return []string{"nico-networking"} }

func (p *NetrisFabricProvider) Init(ctx provider.ProviderContext) error {
	log.Info().
		Str("provider", p.Name()).
		Str("url", p.netrisURL).
		Msg("initializing netris-fabric provider")

	c, err := client.New(client.Config{
		URL:      p.netrisURL,
		Username: p.netrisUser,
		Password: p.netrisPass,
	})
	if err != nil {
		return err
	}

	if err := c.Login(context.Background()); err != nil {
		return err
	}

	p.client = c

	p.registerHooks(ctx.Registry)

	log.Info().
		Str("provider", p.Name()).
		Msg("netris-fabric provider initialized")

	return nil
}

func (p *NetrisFabricProvider) Shutdown(_ context.Context) error {
	log.Info().
		Str("provider", p.Name()).
		Msg("shutting down netris-fabric provider")
	return nil
}
