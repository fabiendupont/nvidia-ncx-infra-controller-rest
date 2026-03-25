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

// Package ansiblefabric implements a NICo provider that automates physical
// network fabric configuration via Ansible Automation Platform (AAP).
//
// Instead of calling switch APIs directly, this provider triggers AAP job
// templates that execute Ansible playbooks using the nvidia.nvue collection
// (for Ethernet/Cumulus switches) and UFM REST API modules (for InfiniBand
// fabric). This approach:
//
//   - Uses Red Hat's own AAP product (already in NCP deployments)
//   - Leverages nvidia.nvue, a Red Hat-certified Ansible collection
//   - Delegates credential management to AAP Controller
//   - Gets idempotency and check mode from Ansible modules for free
//   - Supports workflow job templates for validate → apply → verify chains
//
// The provider registers hooks on NICo networking events (VPC, Subnet,
// Instance, InfiniBand partition lifecycle) and translates them into AAP
// job template launches with appropriate extra_vars.
package ansiblefabric

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/ansible-fabric/client"
)

// AnsibleFabricProvider manages physical network fabric via AAP Controller.
// It reacts to NICo networking events (VPC/Subnet/Instance/IB partition
// lifecycle) and triggers Ansible playbooks to configure the physical
// switches accordingly.
//
// Playbooks use the nvidia.nvue collection for Ethernet switches running
// Cumulus Linux and the UFM REST API for InfiniBand switch fabric.
//
// This provider is complementary to nico-networking, not a replacement.
// NICo manages tenant-level networking constructs (VPCs, subnets, NSGs).
// This provider translates those constructs into physical switch
// configurations via Ansible.
type AnsibleFabricProvider struct {
	config ProviderConfig
	client *client.Client
}

// New creates a new AnsibleFabricProvider with explicit configuration.
func New(cfg ProviderConfig) *AnsibleFabricProvider {
	return &AnsibleFabricProvider{
		config: cfg,
	}
}

// NewFromEnv creates a new AnsibleFabricProvider reading configuration
// from environment variables.
func NewFromEnv() *AnsibleFabricProvider {
	return New(ConfigFromEnv())
}

func (p *AnsibleFabricProvider) Name() string           { return "ansible-fabric" }
func (p *AnsibleFabricProvider) Version() string        { return "0.1.0" }
func (p *AnsibleFabricProvider) Features() []string     { return []string{"fabric", "ib-fabric"} }
func (p *AnsibleFabricProvider) Dependencies() []string { return []string{"nico-networking"} }

func (p *AnsibleFabricProvider) Init(ctx provider.ProviderContext) error {
	logger := log.With().Str("provider", p.Name()).Logger()

	logger.Info().
		Str("url", p.config.AAPURL).
		Msg("initializing ansible-fabric provider")

	if err := p.config.Validate(); err != nil {
		return err
	}

	c, err := client.New(client.Config{
		URL:      p.config.AAPURL,
		Token:    p.config.AAPToken,
		Username: p.config.AAPUsername,
		Password: p.config.AAPPassword,
	})
	if err != nil {
		return err
	}

	if err := c.Ping(context.Background()); err != nil {
		return err
	}

	p.client = c

	p.registerHooks(ctx.Registry)

	logger.Info().
		Int("create_vpc_template", p.config.Templates.CreateVPC).
		Int("delete_vpc_template", p.config.Templates.DeleteVPC).
		Int("create_subnet_template", p.config.Templates.CreateSubnet).
		Int("delete_subnet_template", p.config.Templates.DeleteSubnet).
		Int("create_ib_partition_template", p.config.Templates.CreateIBPartition).
		Int("delete_ib_partition_template", p.config.Templates.DeleteIBPartition).
		Int("configure_instance_template", p.config.Templates.ConfigureInstance).
		Int("deconfigure_instance_template", p.config.Templates.DeconfigureInstance).
		Msg("ansible-fabric provider initialized")

	return nil
}

func (p *AnsibleFabricProvider) Shutdown(_ context.Context) error {
	log.Info().
		Str("provider", p.Name()).
		Msg("shutting down ansible-fabric provider")
	return nil
}
