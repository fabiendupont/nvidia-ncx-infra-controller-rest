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
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/providers/netris-fabric/client"
)

// idMap tracks the bidirectional mapping between NICo string IDs
// (UUIDs) and Netris integer IDs. In production this would be
// persisted in the database; the in-memory map is sufficient for
// the proof of concept.
type idMap struct {
	mu       sync.RWMutex
	toNetris map[string]int // NICo ID → Netris ID
	toNICo   map[int]string // Netris ID → NICo ID
}

func newIDMap() *idMap {
	return &idMap{
		toNetris: make(map[string]int),
		toNICo:   make(map[int]string),
	}
}

func (m *idMap) set(nicoID string, netrisID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toNetris[nicoID] = netrisID
	m.toNICo[netrisID] = nicoID
}

func (m *idMap) getNetrisID(nicoID string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.toNetris[nicoID]
	return id, ok
}

func (m *idMap) delete(nicoID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if netrisID, ok := m.toNetris[nicoID]; ok {
		delete(m.toNICo, netrisID)
	}
	delete(m.toNetris, nicoID)
}

// SyncVPCToFabric creates a Netris VPC (VRF) that matches the NICo VPC.
// If the VPC already exists in the mapping, this is a no-op (idempotent).
func (p *NetrisFabricProvider) SyncVPCToFabric(ctx context.Context, vpcID string) error {
	logger := log.With().Str("provider", p.Name()).Str("vpc_id", vpcID).Logger()

	if p.client == nil {
		return fmt.Errorf("netris client not initialized")
	}

	// Check if already synced
	if _, exists := p.vpcIDs.getNetrisID(vpcID); exists {
		logger.Debug().Msg("VPC already synced to Netris, skipping")
		return nil
	}

	logger.Info().Msg("creating VPC on Netris fabric")

	netrisVPC, err := p.client.CreateVPC(ctx, client.NetrisVPC{
		Name:        fmt.Sprintf("nico-%s", vpcID),
		Description: fmt.Sprintf("NICo VPC %s", vpcID),
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to create VPC on Netris")
		return fmt.Errorf("creating Netris VPC for %s: %w", vpcID, err)
	}

	p.vpcIDs.set(vpcID, netrisVPC.ID)
	logger.Info().Int("netris_id", netrisVPC.ID).Msg("VPC synced to Netris fabric")
	return nil
}

// SyncSubnetToFabric creates a Netris VNET that matches the NICo Subnet.
// The VNET is associated with the parent VPC's Netris VPC ID.
func (p *NetrisFabricProvider) SyncSubnetToFabric(ctx context.Context, subnetID string, vpcID string, prefix string) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Str("vpc_id", vpcID).Logger()

	if p.client == nil {
		return fmt.Errorf("netris client not initialized")
	}

	// Check if already synced
	if _, exists := p.subnetIDs.getNetrisID(subnetID); exists {
		logger.Debug().Msg("subnet already synced to Netris, skipping")
		return nil
	}

	// Look up parent VPC's Netris ID
	netrisVPCID, ok := p.vpcIDs.getNetrisID(vpcID)
	if !ok {
		return fmt.Errorf("parent VPC %s not found in Netris mapping; sync the VPC first", vpcID)
	}

	logger.Info().Str("prefix", prefix).Int("netris_vpc_id", netrisVPCID).Msg("creating VNET on Netris fabric")

	netrisVNet, err := p.client.CreateVNet(ctx, client.NetrisVNet{
		Name:  fmt.Sprintf("nico-%s", subnetID),
		VPCID: netrisVPCID,
		State: "active",
		Sites: []client.NetrisVNetSite{
			{
				Gateways: []string{prefix},
			},
		},
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to create VNET on Netris")
		return fmt.Errorf("creating Netris VNET for subnet %s: %w", subnetID, err)
	}

	p.subnetIDs.set(subnetID, netrisVNet.ID)
	logger.Info().Int("netris_id", netrisVNet.ID).Msg("subnet synced to Netris fabric")
	return nil
}

// ConfigurePortForMachine configures the physical switch port for a
// machine's NIC. Sets the port into the correct VLAN with appropriate
// MTU settings.
func (p *NetrisFabricProvider) ConfigurePortForMachine(ctx context.Context, machineID string, portID int, mtu int) error {
	logger := log.With().Str("provider", p.Name()).Str("machine_id", machineID).Int("port_id", portID).Logger()

	if p.client == nil {
		return fmt.Errorf("netris client not initialized")
	}

	logger.Info().Int("mtu", mtu).Msg("configuring switch port for machine")

	_, err := p.client.UpdatePort(ctx, portID, client.NetrisPort{
		MTU:        mtu,
		AdminState: "up",
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to configure switch port")
		return fmt.Errorf("configuring port %d for machine %s: %w", portID, machineID, err)
	}

	logger.Info().Msg("switch port configured for machine")
	return nil
}

// RemoveVPCFromFabric deletes the Netris VPC that corresponds to the
// NICo VPC. No-op if the VPC was never synced.
func (p *NetrisFabricProvider) RemoveVPCFromFabric(ctx context.Context, vpcID string) error {
	logger := log.With().Str("provider", p.Name()).Str("vpc_id", vpcID).Logger()

	if p.client == nil {
		return fmt.Errorf("netris client not initialized")
	}

	netrisID, ok := p.vpcIDs.getNetrisID(vpcID)
	if !ok {
		logger.Debug().Msg("VPC not found in Netris mapping, nothing to remove")
		return nil
	}

	logger.Info().Int("netris_id", netrisID).Msg("removing VPC from Netris fabric")

	if err := p.client.DeleteVPC(ctx, netrisID); err != nil {
		logger.Error().Err(err).Msg("failed to delete VPC from Netris")
		return fmt.Errorf("deleting Netris VPC %d for %s: %w", netrisID, vpcID, err)
	}

	p.vpcIDs.delete(vpcID)
	logger.Info().Msg("VPC removed from Netris fabric")
	return nil
}

// RemoveSubnetFromFabric deletes the Netris VNET that corresponds to
// the NICo Subnet. No-op if the subnet was never synced.
func (p *NetrisFabricProvider) RemoveSubnetFromFabric(ctx context.Context, subnetID string) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Logger()

	if p.client == nil {
		return fmt.Errorf("netris client not initialized")
	}

	netrisID, ok := p.subnetIDs.getNetrisID(subnetID)
	if !ok {
		logger.Debug().Msg("subnet not found in Netris mapping, nothing to remove")
		return nil
	}

	logger.Info().Int("netris_id", netrisID).Msg("removing VNET from Netris fabric")

	if err := p.client.DeleteVNet(ctx, netrisID); err != nil {
		logger.Error().Err(err).Msg("failed to delete VNET from Netris")
		return fmt.Errorf("deleting Netris VNET %d for subnet %s: %w", netrisID, subnetID, err)
	}

	p.subnetIDs.delete(subnetID)
	logger.Info().Msg("subnet removed from Netris fabric")
	return nil
}

// CheckIPAMConflict queries Netris IPAM and checks whether the given
// CIDR prefix overlaps with any existing Netris allocation or subnet.
// Returns an error describing the conflict, or nil if the prefix is clear.
func (p *NetrisFabricProvider) CheckIPAMConflict(ctx context.Context, prefix string) error {
	if p.client == nil {
		return fmt.Errorf("netris client not initialized")
	}

	_, requestedNet, err := net.ParseCIDR(prefix)
	if err != nil {
		return fmt.Errorf("invalid CIDR prefix %q: %w", prefix, err)
	}

	// Check allocations
	allocations, err := p.client.ListAllocations(ctx)
	if err != nil {
		return fmt.Errorf("listing Netris allocations: %w", err)
	}

	for _, alloc := range allocations {
		_, allocNet, err := net.ParseCIDR(alloc.Prefix)
		if err != nil {
			continue
		}
		if requestedNet.Contains(allocNet.IP) || allocNet.Contains(requestedNet.IP) {
			return fmt.Errorf("requested prefix %s conflicts with Netris allocation %q (%s)", prefix, alloc.Name, alloc.Prefix)
		}
	}

	// Check subnets
	subnets, err := p.client.ListSubnets(ctx)
	if err != nil {
		return fmt.Errorf("listing Netris subnets: %w", err)
	}

	for _, subnet := range subnets {
		_, subnetNet, err := net.ParseCIDR(subnet.Prefix)
		if err != nil {
			continue
		}
		if requestedNet.Contains(subnetNet.IP) || subnetNet.Contains(requestedNet.IP) {
			return fmt.Errorf("requested prefix %s conflicts with Netris subnet %q (%s)", prefix, subnet.Name, subnet.Prefix)
		}
	}

	return nil
}
