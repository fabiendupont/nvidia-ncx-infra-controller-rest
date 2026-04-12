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

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers async reactions and sync hooks for NICo networking
// events so the Spectrum switch fabric stays in sync with tenant intent via
// direct NVUE API calls.
func (p *SpectrumFabricProvider) registerHooks(registry *provider.Registry) {
	// --- VPC lifecycle → VRF management on Spectrum switches ---

	if p.config.Features.SyncVPC {
		// When NICo creates a VPC, create a matching VRF on the fabric.
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostCreateVPC,
			TargetWorkflow: "spectrum-fabric-sync",
			SignalName:     "vpc-created",
		})

		// When NICo deletes a VPC, remove the VRF from the fabric.
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostDeleteVPC,
			TargetWorkflow: "spectrum-fabric-sync",
			SignalName:     "vpc-deleted",
		})
	}

	// --- Subnet lifecycle → VxLAN VNI management on Spectrum switches ---

	if p.config.Features.SyncSubnet {
		// When NICo creates a subnet, create a matching VxLAN VNI.
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostCreateSubnet,
			TargetWorkflow: "spectrum-fabric-sync",
			SignalName:     "subnet-created",
		})

		// When NICo deletes a subnet, remove the VxLAN VNI.
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostDeleteSubnet,
			TargetWorkflow: "spectrum-fabric-sync",
			SignalName:     "subnet-deleted",
		})

		// Pre-create subnet validation: dry-run the NVUE config patch
		// to verify the switch can accept the configuration before NICo
		// commits the subnet to its database.
		registry.RegisterHook(provider.SyncHook{
			Feature: "networking",
			Event:   provider.EventPreCreateSubnet,
			Handler: p.validateSubnetConfig,
		})
	}
}

// validateSubnetConfig performs a dry-run NVUE configuration check to verify
// that the requested subnet can be provisioned on the Spectrum switches
// before NICo commits the subnet to the database.
//
// The validation creates an NVUE revision with the proposed VxLAN VNI
// configuration but does not apply it. If the revision creation or patch
// fails, the subnet creation is aborted with a descriptive error.
func (p *SpectrumFabricProvider) validateSubnetConfig(ctx context.Context, payload interface{}) error {
	prefix, err := extractPayloadField(payload, "prefix", "cidr")
	if err != nil {
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract prefix from subnet creation payload, skipping NVUE validation")
		return nil
	}

	vpcID, err := extractPayloadField(payload, "vpc_id")
	if err != nil {
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract vpc_id from subnet creation payload, skipping NVUE validation")
		return nil
	}

	logger := log.With().
		Str("provider", p.Name()).
		Str("prefix", prefix).
		Str("vpc_id", vpcID).
		Logger()

	logger.Info().Msg("validating subnet config against Spectrum fabric (NVUE dry-run)")

	// Create a revision and patch the proposed config to validate it,
	// but do NOT apply. This verifies that NVUE accepts the config
	// before NICo commits the subnet to the database.
	revID, err := p.client.CreateRevisionID(ctx)
	if err != nil {
		return fmt.Errorf("NVUE dry-run: creating revision: %w", err)
	}

	vrfName := fmt.Sprintf("nico-%s", vpcID)
	sviConfig := map[string]any{
		"type": "svi",
		"ip": map[string]any{
			"address": map[string]any{
				prefix: map[string]any{},
			},
			"vrf": vrfName,
		},
	}

	// Patch a dummy SVI config into the revision to validate it.
	if err := p.client.Patch(ctx, "/interface/validation-probe", revID, sviConfig); err != nil {
		// Delete the revision before returning the validation error.
		_ = p.client.Delete(ctx, fmt.Sprintf("/revision/%s", revID), "")
		return fmt.Errorf("NVUE dry-run: subnet config rejected by Spectrum fabric: %w", err)
	}

	// Clean up the validation revision without applying it.
	_ = p.client.Delete(ctx, fmt.Sprintf("/revision/%s", revID), "")

	logger.Info().Msg("subnet config validated against Spectrum fabric")

	return nil
}

// extractPayloadField attempts to extract a string field from the hook
// payload by trying multiple field names in order. The payload can be a
// map[string]interface{} or map[string]string.
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
