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

package ansiblefabric

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers async reactions and sync hooks for NICo networking
// and InfiniBand events so the physical fabric stays in sync with tenant
// intent via AAP job templates.
func (p *AnsibleFabricProvider) registerHooks(registry *provider.Registry) {
	// --- Ethernet fabric (nvidia.nvue collection) ---

	// VPC lifecycle → VRF management on physical switches.
	if p.config.Templates.HasTemplate(p.config.Templates.CreateVPC) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostCreateVPC,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "vpc-created",
		})
	}

	if p.config.Templates.HasTemplate(p.config.Templates.DeleteVPC) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostDeleteVPC,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "vpc-deleted",
		})
	}

	// Subnet lifecycle → VxLAN VNET management on physical switches.
	if p.config.Templates.HasTemplate(p.config.Templates.CreateSubnet) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostCreateSubnet,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "subnet-created",
		})
	}

	if p.config.Templates.HasTemplate(p.config.Templates.DeleteSubnet) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostDeleteSubnet,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "subnet-deleted",
		})
	}

	// Instance lifecycle → port configuration on ToR switches.
	if p.config.Templates.HasTemplate(p.config.Templates.ConfigureInstance) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostCreateInstance,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "instance-created",
		})
	}

	if p.config.Templates.HasTemplate(p.config.Templates.DeconfigureInstance) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostDeleteInstance,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "instance-deleted",
		})
	}

	// --- InfiniBand fabric (UFM REST API) ---

	// IB partition lifecycle → PKEY management on IB switches via UFM.
	if p.config.Templates.HasTemplate(p.config.Templates.CreateIBPartition) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostCreateInfiniBandPartition,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "ib-partition-created",
		})
	}

	if p.config.Templates.HasTemplate(p.config.Templates.DeleteIBPartition) {
		registry.RegisterReaction(provider.Reaction{
			Feature:        "networking",
			Event:          provider.EventPostDeleteInfiniBandPartition,
			TargetWorkflow: "ansible-fabric-sync",
			SignalName:     "ib-partition-deleted",
		})
	}

	// Pre-create subnet validation: run the playbook in check mode to
	// validate the configuration before committing to NICo's database.
	if p.config.Templates.HasTemplate(p.config.Templates.CreateSubnet) {
		registry.RegisterHook(provider.SyncHook{
			Feature: "networking",
			Event:   provider.EventPreCreateSubnet,
			Handler: p.validateSubnetConfig,
		})
	}

	// Pre-create IB partition validation: verify UFM can accept the
	// PKEY configuration before committing.
	if p.config.Templates.HasTemplate(p.config.Templates.CreateIBPartition) {
		registry.RegisterHook(provider.SyncHook{
			Feature: "networking",
			Event:   provider.EventPreCreateInfiniBandPartition,
			Handler: p.validateIBPartitionConfig,
		})
	}
}

// validateSubnetConfig runs the subnet creation playbook in check mode
// (dry run) via AAP to verify the switch configuration is valid before
// NICo commits the subnet to the database.
func (p *AnsibleFabricProvider) validateSubnetConfig(ctx context.Context, payload interface{}) error {
	prefix, err := extractPayloadField(payload, "prefix", "cidr")
	if err != nil {
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract prefix from subnet creation payload, skipping validation")
		return nil
	}

	vpcID, err := extractPayloadField(payload, "vpc_id")
	if err != nil {
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract vpc_id from subnet creation payload, skipping validation")
		return nil
	}

	log.Info().
		Str("provider", p.Name()).
		Str("prefix", prefix).
		Str("vpc_id", vpcID).
		Msg("validating subnet config against physical fabric (AAP check mode)")

	job, err := p.launchAndWait(ctx, p.config.Templates.CreateSubnet, map[string]interface{}{
		"nico_vpc_id":        vpcID,
		"nico_subnet_prefix": prefix,
		"ansible_check_mode": true,
	})
	if err != nil {
		return fmt.Errorf("subnet validation failed: %w", err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("subnet config validation failed on physical fabric: AAP job %d status %s", job.ID, job.Status)
	}

	log.Info().
		Str("provider", p.Name()).
		Str("prefix", prefix).
		Msg("subnet config validated against physical fabric")

	return nil
}

// validateIBPartitionConfig verifies that the InfiniBand partition can be
// created on the UFM-managed fabric by running the playbook in check mode.
func (p *AnsibleFabricProvider) validateIBPartitionConfig(ctx context.Context, payload interface{}) error {
	partitionName, err := extractPayloadField(payload, "name", "partition_name")
	if err != nil {
		log.Warn().
			Str("provider", p.Name()).
			Err(err).
			Msg("cannot extract partition name from IB partition payload, skipping validation")
		return nil
	}

	pkey, _ := extractPayloadField(payload, "pkey")
	tenantID, _ := extractPayloadField(payload, "tenant_id")

	log.Info().
		Str("provider", p.Name()).
		Str("partition_name", partitionName).
		Str("pkey", pkey).
		Msg("validating IB partition config against UFM (AAP check mode)")

	job, err := p.launchAndWait(ctx, p.config.Templates.CreateIBPartition, map[string]interface{}{
		"nico_partition_name": partitionName,
		"nico_pkey":           pkey,
		"nico_tenant_id":      tenantID,
		"ansible_check_mode":  true,
	})
	if err != nil {
		return fmt.Errorf("IB partition validation failed: %w", err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("IB partition config validation failed on UFM: AAP job %d status %s", job.ID, job.Status)
	}

	log.Info().
		Str("provider", p.Name()).
		Str("partition_name", partitionName).
		Msg("IB partition config validated against UFM")

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
