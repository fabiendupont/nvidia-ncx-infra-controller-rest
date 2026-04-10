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
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	// TODO: Uncomment when nvue-client-go is available.
	// nvue "github.com/NVIDIA/nvue-client-go"
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
	// TODO: Uncomment when nvue-client-go is available.
	// client *nvue.Client
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

	// TODO: Create the NVUE REST client when nvue-client-go is available.
	//
	// c, err := nvue.NewClient(nvue.Config{
	// 	URL:           p.config.NVUEURL,
	// 	Username:      p.config.NVUEUsername,
	// 	Password:      p.config.NVUEPassword,
	// 	TLSSkipVerify: p.config.TLSSkipVerify,
	// })
	// if err != nil {
	// 	return err
	// }
	//
	// // Verify connectivity by fetching the system version.
	// if _, err := c.GetSystemVersion(context.Background()); err != nil {
	// 	return fmt.Errorf("NVUE connectivity check failed: %w", err)
	// }
	//
	// p.client = c

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
