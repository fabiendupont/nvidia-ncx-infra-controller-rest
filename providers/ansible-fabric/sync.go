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
	"time"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/providers/ansible-fabric/client"
)

// launchAndWait launches an AAP job template and waits for completion.
// Returns the final job state or an error if the job fails or times out.
func (p *AnsibleFabricProvider) launchAndWait(ctx context.Context, templateID int, extraVars map[string]interface{}) (*client.Job, error) {
	if p.client == nil {
		return nil, fmt.Errorf("AAP client not initialized")
	}

	if !p.config.Templates.HasTemplate(templateID) {
		return nil, fmt.Errorf("job template ID %d is not configured", templateID)
	}

	timeout := p.config.JobTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := p.client.LaunchJobTemplate(ctx, templateID, client.LaunchRequest{
		ExtraVars: extraVars,
	})
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("provider", p.Name()).
		Int("template_id", templateID).
		Int("job_id", resp.Job).
		Msg("AAP job launched")

	job, err := p.client.WaitForJob(ctx, resp.Job, p.config.JobPollInterval)
	if err != nil {
		return nil, fmt.Errorf("waiting for AAP job %d: %w", resp.Job, err)
	}

	log.Info().
		Str("provider", p.Name()).
		Int("job_id", job.ID).
		Str("status", string(job.Status)).
		Float64("elapsed_seconds", job.Elapsed).
		Msg("AAP job completed")

	return job, nil
}

// SyncVPCToFabric triggers an AAP job template that creates a VRF on the
// physical switch fabric for the given NICo VPC.
//
// The playbook is expected to use the nvidia.nvue collection:
//
//	- nvidia.nvue.vrf: create VRF "nico-{{ nico_vpc_id }}"
//	- nvidia.nvue.router: configure BGP for the VRF
//	- nvidia.nvue.evpn: advertise VRF via EVPN (if configured)
func (p *AnsibleFabricProvider) SyncVPCToFabric(ctx context.Context, vpcID, vpcName, tenantID string) error {
	logger := log.With().Str("provider", p.Name()).Str("vpc_id", vpcID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.CreateVPC) {
		logger.Debug().Msg("create-vpc template not configured, skipping fabric sync")
		return nil
	}

	logger.Info().Msg("syncing VPC to physical fabric via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.CreateVPC, map[string]interface{}{
		"nico_vpc_id":   vpcID,
		"nico_vpc_name": vpcName,
		"nico_tenant_id": tenantID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to sync VPC to fabric")
		return fmt.Errorf("syncing VPC %s to fabric: %w", vpcID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("VPC fabric sync failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("VPC synced to physical fabric")
	return nil
}

// RemoveVPCFromFabric triggers an AAP job template that removes a VRF from
// the physical switch fabric.
//
// The playbook is expected to:
//
//	- nvidia.nvue.vrf: delete VRF "nico-{{ nico_vpc_id }}" (state: absent)
func (p *AnsibleFabricProvider) RemoveVPCFromFabric(ctx context.Context, vpcID string) error {
	logger := log.With().Str("provider", p.Name()).Str("vpc_id", vpcID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.DeleteVPC) {
		logger.Debug().Msg("delete-vpc template not configured, skipping fabric cleanup")
		return nil
	}

	logger.Info().Msg("removing VPC from physical fabric via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.DeleteVPC, map[string]interface{}{
		"nico_vpc_id": vpcID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to remove VPC from fabric")
		return fmt.Errorf("removing VPC %s from fabric: %w", vpcID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("VPC fabric removal failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("VPC removed from physical fabric")
	return nil
}

// SyncSubnetToFabric triggers an AAP job template that creates a VxLAN
// VNET on the physical switch fabric for the given NICo subnet.
//
// The playbook is expected to use the nvidia.nvue collection:
//
//	- nvidia.nvue.vxlan: create VNI for subnet
//	- nvidia.nvue.bridge: add VNET to bridge domain
//	- nvidia.nvue.evpn: advertise VNI via BGP EVPN
//	- nvidia.nvue.interface: configure gateway IP on SVI
func (p *AnsibleFabricProvider) SyncSubnetToFabric(ctx context.Context, subnetID, vpcID, prefix, subnetName string) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Str("vpc_id", vpcID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.CreateSubnet) {
		logger.Debug().Msg("create-subnet template not configured, skipping fabric sync")
		return nil
	}

	logger.Info().Str("prefix", prefix).Msg("syncing subnet to physical fabric via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.CreateSubnet, map[string]interface{}{
		"nico_subnet_id":     subnetID,
		"nico_vpc_id":        vpcID,
		"nico_subnet_prefix": prefix,
		"nico_subnet_name":   subnetName,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to sync subnet to fabric")
		return fmt.Errorf("syncing subnet %s to fabric: %w", subnetID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("subnet fabric sync failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("subnet synced to physical fabric")
	return nil
}

// RemoveSubnetFromFabric triggers an AAP job template that removes a VxLAN
// VNET from the physical switch fabric.
//
// The playbook is expected to:
//
//	- nvidia.nvue.vxlan: remove VNI (state: absent)
//	- nvidia.nvue.bridge: remove VNET from bridge domain
func (p *AnsibleFabricProvider) RemoveSubnetFromFabric(ctx context.Context, subnetID, vpcID string) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.DeleteSubnet) {
		logger.Debug().Msg("delete-subnet template not configured, skipping fabric cleanup")
		return nil
	}

	logger.Info().Msg("removing subnet from physical fabric via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.DeleteSubnet, map[string]interface{}{
		"nico_subnet_id": subnetID,
		"nico_vpc_id":    vpcID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to remove subnet from fabric")
		return fmt.Errorf("removing subnet %s from fabric: %w", subnetID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("subnet fabric removal failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("subnet removed from physical fabric")
	return nil
}

// ConfigureInstancePorts triggers an AAP job template that configures
// physical switch ports for a newly created instance (MTU, VLAN membership,
// admin state).
//
// The playbook is expected to use the nvidia.nvue collection:
//
//	- nvidia.nvue.interface: set MTU, admin state "up"
//	- nvidia.nvue.bridge: add port to VNET bridge domain
func (p *AnsibleFabricProvider) ConfigureInstancePorts(ctx context.Context, instanceID, machineID, vpcID, subnetID string) error {
	logger := log.With().Str("provider", p.Name()).Str("instance_id", instanceID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.ConfigureInstance) {
		logger.Debug().Msg("configure-instance template not configured, skipping port config")
		return nil
	}

	logger.Info().Str("machine_id", machineID).Msg("configuring switch ports for instance via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.ConfigureInstance, map[string]interface{}{
		"nico_instance_id": instanceID,
		"nico_machine_id":  machineID,
		"nico_vpc_id":      vpcID,
		"nico_subnet_id":   subnetID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to configure switch ports for instance")
		return fmt.Errorf("configuring ports for instance %s: %w", instanceID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("instance port config failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("switch ports configured for instance")
	return nil
}

// DeconfigureInstancePorts triggers an AAP job template that removes
// physical switch port configuration when an instance is deleted.
func (p *AnsibleFabricProvider) DeconfigureInstancePorts(ctx context.Context, instanceID, machineID string) error {
	logger := log.With().Str("provider", p.Name()).Str("instance_id", instanceID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.DeconfigureInstance) {
		logger.Debug().Msg("deconfigure-instance template not configured, skipping port cleanup")
		return nil
	}

	logger.Info().Str("machine_id", machineID).Msg("deconfiguring switch ports for instance via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.DeconfigureInstance, map[string]interface{}{
		"nico_instance_id": instanceID,
		"nico_machine_id":  machineID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to deconfigure switch ports for instance")
		return fmt.Errorf("deconfiguring ports for instance %s: %w", instanceID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("instance port deconfiguration failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("switch ports deconfigured for instance")
	return nil
}

// SyncIBPartitionToFabric triggers an AAP job template that creates an
// InfiniBand partition (PKEY) on the UFM-managed IB fabric.
//
// The playbook is expected to use ansible.builtin.uri to call UFM REST API:
//
//	- POST /ufmRestV2/plugin/fast_api/resources/pkeys: create PKEY
//	- Configure GUID membership for the partition
func (p *AnsibleFabricProvider) SyncIBPartitionToFabric(ctx context.Context, partitionID, partitionName, pkey, tenantID string, guids []string) error {
	logger := log.With().Str("provider", p.Name()).Str("partition_id", partitionID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.CreateIBPartition) {
		logger.Debug().Msg("create-ib-partition template not configured, skipping IB fabric sync")
		return nil
	}

	logger.Info().
		Str("partition_name", partitionName).
		Str("pkey", pkey).
		Int("guid_count", len(guids)).
		Msg("syncing IB partition to UFM fabric via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.CreateIBPartition, map[string]interface{}{
		"nico_partition_id":   partitionID,
		"nico_partition_name": partitionName,
		"nico_pkey":           pkey,
		"nico_tenant_id":      tenantID,
		"nico_guids":          guids,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to sync IB partition to UFM")
		return fmt.Errorf("syncing IB partition %s to UFM: %w", partitionID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("IB partition fabric sync failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("IB partition synced to UFM fabric")
	return nil
}

// RemoveIBPartitionFromFabric triggers an AAP job template that removes an
// InfiniBand partition (PKEY) from the UFM-managed IB fabric.
//
// The playbook is expected to use ansible.builtin.uri to call UFM REST API:
//
//	- DELETE /ufmRestV2/plugin/fast_api/resources/pkeys/<pkey>/delete
func (p *AnsibleFabricProvider) RemoveIBPartitionFromFabric(ctx context.Context, partitionID, pkey string) error {
	logger := log.With().Str("provider", p.Name()).Str("partition_id", partitionID).Logger()

	if !p.config.Templates.HasTemplate(p.config.Templates.DeleteIBPartition) {
		logger.Debug().Msg("delete-ib-partition template not configured, skipping IB fabric cleanup")
		return nil
	}

	logger.Info().Str("pkey", pkey).Msg("removing IB partition from UFM fabric via AAP")

	job, err := p.launchAndWait(ctx, p.config.Templates.DeleteIBPartition, map[string]interface{}{
		"nico_partition_id": partitionID,
		"nico_pkey":         pkey,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to remove IB partition from UFM")
		return fmt.Errorf("removing IB partition %s from UFM: %w", partitionID, err)
	}

	if !job.Status.IsSuccess() {
		return fmt.Errorf("IB partition fabric removal failed: AAP job %d status %s", job.ID, job.Status)
	}

	logger.Info().Int("job_id", job.ID).Msg("IB partition removed from UFM fabric")
	return nil
}
