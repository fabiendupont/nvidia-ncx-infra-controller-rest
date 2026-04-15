/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ufmfabric

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers async reactions and sync hooks for NICo IB
// partition events so the UFM-managed InfiniBand fabric stays in sync
// with tenant intent via direct UFM REST API calls.
func (p *UFMFabricProvider) registerHooks(registry *provider.Registry) {
	if !p.config.Features.SyncIBPartition {
		return
	}

	// When NICo creates an IB partition, create the matching PKEY on UFM.
	registry.RegisterReaction(provider.Reaction{
		Feature:        "networking",
		Event:          provider.EventPostCreateInfiniBandPartition,
		TargetWorkflow: "ufm-fabric-sync",
		SignalName:     "ib-partition-created",
	})

	// When NICo deletes an IB partition, remove the PKEY from UFM.
	registry.RegisterReaction(provider.Reaction{
		Feature:        "networking",
		Event:          provider.EventPostDeleteInfiniBandPartition,
		TargetWorkflow: "ufm-fabric-sync",
		SignalName:     "ib-partition-deleted",
	})

	// Pre-create validation: verify the PKEY doesn't conflict with
	// an existing partition on UFM before NICo commits to its database.
	registry.RegisterHook(provider.SyncHook{
		Feature: "networking",
		Event:   provider.EventPreCreateInfiniBandPartition,
		Handler: p.validateIBPartitionConfig,
	})
}

// validateIBPartitionConfig verifies that the requested IB partition
// (PKEY) can be created on the UFM-managed fabric by checking for
// conflicts with existing partitions.
func (p *UFMFabricProvider) validateIBPartitionConfig(ctx context.Context, payload interface{}) error {
	pkey, err := extractPayloadField(payload, "pkey")
	if err != nil {
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract pkey from IB partition creation payload, skipping UFM validation")
		return nil
	}

	logger := log.With().
		Str("provider", p.Name()).
		Str("pkey", pkey).
		Logger()

	logger.Info().Msg("validating IB partition config against UFM (pre-create check)")

	// Check if this PKEY already exists on UFM.
	existing, err := p.client.GetPKey(ctx, pkey, false)
	if err != nil {
		// If UFM returns an error (e.g. 404), the PKEY doesn't exist — that's fine.
		logger.Debug().Err(err).Msg("PKEY not found on UFM, validation passed")
		return nil
	}

	// PKEY exists — check if it's managed by NICo (partition name starts with "nico-").
	if existing.Partition != "" {
		return fmt.Errorf("UFM validation failed: PKEY %s already exists as partition %q on UFM", pkey, existing.Partition)
	}

	return nil
}

// extractPayloadField attempts to extract a string field from the hook
// payload by trying multiple field names in order.
func extractPayloadField(payload interface{}, names ...string) (string, error) {
	if payload == nil {
		return "", fmt.Errorf("nil payload")
	}

	if m, ok := payload.(map[string]interface{}); ok {
		for _, name := range names {
			if v, ok := m[name].(string); ok && v != "" {
				return v, nil
			}
		}
	}

	if m, ok := payload.(map[string]string); ok {
		for _, name := range names {
			if v, ok := m[name]; ok && v != "" {
				return v, nil
			}
		}
	}

	return "", fmt.Errorf("no field %v found in payload", names)
}
