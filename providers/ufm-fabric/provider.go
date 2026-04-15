/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ufmfabric

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/fabiendupont/nvidia-ufm-api/pkg/ufmclient"
)

// UFMFabricProvider manages InfiniBand partition (PKEY) lifecycle on
// UFM Enterprise directly via its REST API. It reacts to NICo IB partition
// events and translates them into UFM PKEY operations.
//
// Unlike the ansible-fabric provider (which delegates to AAP + nvidia.ufm
// collection), this provider communicates directly with UFM's REST API.
type UFMFabricProvider struct {
	config ProviderConfig
	client *ufmclient.Client
	syncMu sync.Mutex
}

// New creates a new UFMFabricProvider with explicit configuration.
func New(cfg ProviderConfig) *UFMFabricProvider {
	return &UFMFabricProvider{
		config: cfg,
	}
}

// NewFromEnv creates a new UFMFabricProvider reading configuration
// from environment variables.
func NewFromEnv() *UFMFabricProvider {
	return New(ConfigFromEnv())
}

func (p *UFMFabricProvider) Name() string           { return "ufm-fabric" }
func (p *UFMFabricProvider) Version() string        { return "0.1.0" }
func (p *UFMFabricProvider) Features() []string     { return []string{"ufm-fabric", "ib-fabric"} }
func (p *UFMFabricProvider) Dependencies() []string { return []string{"nico-networking"} }

func (p *UFMFabricProvider) Init(ctx provider.ProviderContext) error {
	logger := log.With().Str("provider", p.Name()).Logger()

	logger.Info().
		Str("url", p.config.UFMURL).
		Msg("initializing ufm-fabric provider")

	if err := p.config.Validate(); err != nil {
		return err
	}

	opts := []ufmclient.Option{}
	if p.config.TLSSkipVerify {
		opts = append(opts, ufmclient.WithTLSSkipVerify())
	}
	if p.config.JobTimeout > 0 {
		opts = append(opts, ufmclient.WithTimeout(p.config.JobTimeout))
	}

	p.client = ufmclient.New(p.config.UFMURL, p.config.UFMUsername, p.config.UFMPassword, opts...)

	// Verify connectivity by fetching UFM version.
	version, err := p.client.GetVersion(context.Background())
	if err != nil {
		return fmt.Errorf("UFM connectivity check failed: %w", err)
	}

	logger.Info().
		Str("ufm_version", version.UFMReleaseVersion).
		Msg("UFM connectivity verified")

	p.registerHooks(ctx.Registry)

	logger.Info().
		Bool("sync_ib_partition", p.config.Features.SyncIBPartition).
		Msg("ufm-fabric provider initialized")

	return nil
}

func (p *UFMFabricProvider) Shutdown(_ context.Context) error {
	log.Info().
		Str("provider", p.Name()).
		Msg("shutting down ufm-fabric provider")
	return nil
}
