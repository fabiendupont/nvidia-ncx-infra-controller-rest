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

package spectrumfabric

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/fabiendupont/nvidia-nvue-client-go/pkg/nvue"
)

// SpectrumFabricProvider manages Spectrum-X switch fabric directly via the
// NVUE REST API. It reacts to NICo networking events (VPC/Subnet lifecycle)
// and translates them into NVUE configuration changes on the physical
// Spectrum switches.
//
// Unlike the ansible-fabric provider (which delegates to AAP) or the
// netris-fabric provider (which delegates to Netris Controller), this
// provider communicates directly with the NVUE declarative API on each
// switch. This is the simplest deployment model for environments where
// no intermediary orchestration layer is needed.
//
// This provider is complementary to nico-networking, not a replacement.
// NICo manages tenant-level networking constructs (VPCs, subnets, NSGs).
// This provider syncs those constructs to the physical Spectrum switches.
type SpectrumFabricProvider struct {
	config ProviderConfig
	client *nvue.Client
	syncMu sync.Mutex
}

// New creates a new SpectrumFabricProvider with explicit configuration.
func New(cfg ProviderConfig) *SpectrumFabricProvider {
	return &SpectrumFabricProvider{
		config: cfg,
	}
}

// NewFromEnv creates a new SpectrumFabricProvider reading configuration
// from environment variables.
func NewFromEnv() *SpectrumFabricProvider {
	return New(ConfigFromEnv())
}

func (p *SpectrumFabricProvider) Name() string           { return "spectrum-fabric" }
func (p *SpectrumFabricProvider) Version() string        { return "0.1.0" }
func (p *SpectrumFabricProvider) Features() []string     { return []string{"spectrum-fabric"} }
func (p *SpectrumFabricProvider) Dependencies() []string { return []string{"nico-networking"} }

func (p *SpectrumFabricProvider) Init(ctx provider.ProviderContext) error {
	logger := log.With().Str("provider", p.Name()).Logger()

	logger.Info().
		Str("url", p.config.NVUEURL).
		Msg("initializing spectrum-fabric provider")

	if err := p.config.Validate(); err != nil {
		return err
	}

	opts := []nvue.Option{}
	if p.config.TLSSkipVerify {
		opts = append(opts, nvue.WithInsecureSkipVerify())
	}
	if p.config.RevisionPollInterval > 0 {
		opts = append(opts, nvue.WithPollInterval(p.config.RevisionPollInterval))
	}
	if p.config.RevisionTimeout > 0 {
		opts = append(opts, nvue.WithApplyTimeout(p.config.RevisionTimeout))
	}

	p.client = nvue.NewClientFromURL(p.config.NVUEURL, p.config.NVUEUsername, p.config.NVUEPassword, opts...)

	// Verify connectivity by fetching the system configuration.
	if _, err := p.client.GetSystem(context.Background(), ""); err != nil {
		return fmt.Errorf("NVUE connectivity check failed: %w", err)
	}

	p.registerHooks(ctx.Registry)

	logger.Info().
		Bool("sync_vpc", p.config.Features.SyncVPC).
		Bool("sync_subnet", p.config.Features.SyncSubnet).
		Msg("spectrum-fabric provider initialized")

	return nil
}

func (p *SpectrumFabricProvider) Shutdown(_ context.Context) error {
	log.Info().
		Str("provider", p.Name()).
		Msg("shutting down spectrum-fabric provider")
	return nil
}
