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
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers reactions and sync hooks for NICo networking
// events so the physical fabric stays in sync with tenant intent.
func (p *NetrisFabricProvider) registerHooks(registry *provider.Registry) {
	// When NICo creates a VPC, create a matching Netris VPC on the fabric.
	registry.RegisterReaction(provider.Reaction{
		Feature:        "networking",
		Event:          provider.EventPostCreateVPC,
		TargetWorkflow: "netris-fabric-sync",
		SignalName:     "vpc-created",
	})

	// When NICo deletes a VPC, clean up the Netris VPC.
	registry.RegisterReaction(provider.Reaction{
		Feature:        "networking",
		Event:          provider.EventPostDeleteVPC,
		TargetWorkflow: "netris-fabric-sync",
		SignalName:     "vpc-deleted",
	})

	// When NICo creates a subnet, create a matching Netris VNET.
	registry.RegisterReaction(provider.Reaction{
		Feature:        "networking",
		Event:          provider.EventPostCreateSubnet,
		TargetWorkflow: "netris-fabric-sync",
		SignalName:     "subnet-created",
	})

	// When NICo deletes a subnet, remove the Netris VNET.
	registry.RegisterReaction(provider.Reaction{
		Feature:        "networking",
		Event:          provider.EventPostDeleteSubnet,
		TargetWorkflow: "netris-fabric-sync",
		SignalName:     "subnet-deleted",
	})

	// Sync hook: validate IPAM doesn't conflict with Netris allocations
	// before subnet creation. This runs inline in the subnet creation
	// activity and can abort the operation if a conflict is detected.
	registry.RegisterHook(provider.SyncHook{
		Feature: "networking",
		Event:   provider.EventPreCreateSubnet,
		Handler: p.validateIPAMNoConflict,
	})
}

// validateIPAMNoConflict checks Netris IPAM for IP range conflicts before
// NICo creates a subnet. The payload is expected to contain a "prefix"
// field with the requested CIDR prefix.
//
// If the prefix overlaps with an existing Netris allocation or subnet,
// the subnet creation is aborted with a descriptive error.
func (p *NetrisFabricProvider) validateIPAMNoConflict(ctx context.Context, payload interface{}) error {
	// Extract prefix from payload
	prefix, err := extractPrefix(payload)
	if err != nil {
		// If we can't extract the prefix, don't block — log and allow
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract prefix from subnet creation payload, skipping IPAM validation")
		return nil
	}

	log.Info().
		Str("provider", p.Name()).
		Str("prefix", prefix).
		Msg("validating subnet prefix against Netris IPAM")

	if err := p.CheckIPAMConflict(ctx, prefix); err != nil {
		log.Error().
			Str("provider", p.Name()).
			Str("prefix", prefix).
			Err(err).
			Msg("IPAM conflict detected with Netris")
		return fmt.Errorf("netris IPAM conflict: %w", err)
	}

	log.Info().
		Str("provider", p.Name()).
		Str("prefix", prefix).
		Msg("no IPAM conflict with Netris")
	return nil
}

// extractPrefix attempts to extract a "prefix" or "cidr" field from
// the hook payload. The payload can be a map or a struct with those fields.
func extractPrefix(payload interface{}) (string, error) {
	if payload == nil {
		return "", fmt.Errorf("nil payload")
	}

	if m, ok := payload.(map[string]interface{}); ok {
		if prefix, ok := m["prefix"].(string); ok && prefix != "" {
			return prefix, nil
		}
		if cidr, ok := m["cidr"].(string); ok && cidr != "" {
			return cidr, nil
		}
	}

	if m, ok := payload.(map[string]string); ok {
		if prefix, ok := m["prefix"]; ok && prefix != "" {
			return prefix, nil
		}
		if cidr, ok := m["cidr"]; ok && cidr != "" {
			return cidr, nil
		}
	}

	return "", fmt.Errorf("no prefix or cidr field found in payload")
}
