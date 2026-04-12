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

	"github.com/fabiendupont/nvidia-nvue-client-go/pkg/nvue"
)

// SyncVPCToFabric creates a VRF on the Spectrum switches that corresponds
// to the NICo VPC. The VRF is configured via the NVUE REST API using the
// revision-based workflow:
//
//  1. Create a new NVUE revision
//  2. PATCH /nvue_v1/vrf/nico-{vpcID} with VRF configuration
//  3. Apply (commit) the revision
//  4. Poll until the apply completes
//
// The NVUE VRF object model path is: /vrf/{name}
// VRF attributes include: router BGP configuration, EVPN advertisement,
// and L3VNI assignment for VXLAN routing.
//
// This operation is idempotent — if the VRF already exists, NVUE treats
// the PATCH as a no-op.
func (p *SpectrumFabricProvider) SyncVPCToFabric(ctx context.Context, vpcID, vpcName, tenantID string) error {
	logger := log.With().Str("provider", p.Name()).Str("vpc_id", vpcID).Logger()

	if !p.config.Features.SyncVPC {
		logger.Debug().Msg("VPC sync disabled, skipping")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	vrfName := fmt.Sprintf("nico-%s", vpcID)

	logger.Info().
		Str("vrf_name", vrfName).
		Str("vpc_name", vpcName).
		Msg("creating VRF on Spectrum fabric via NVUE")

	vrfConfig := map[string]any{
		"router": map[string]any{
			"bgp": map[string]any{
				"enable":    "on",
				"router-id": "auto",
				"address-family": map[string]any{
					"ipv4-unicast": map[string]any{
						"enable":        "on",
						"redistribute":  map[string]any{"connected": map[string]any{"enable": "on"}},
						"route-export":  map[string]any{"to-evpn": map[string]any{"enable": "on"}},
					},
					"l2vpn-evpn": map[string]any{
						"enable": "on",
					},
				},
			},
		},
		"evpn": map[string]any{
			"enable": "on",
		},
	}

	_, err := p.client.ConfigureAndApply(ctx, []nvue.PatchOp{
		{Path: fmt.Sprintf("/vrf/%s", vrfName), Payload: vrfConfig},
	})
	if err != nil {
		return fmt.Errorf("creating VRF %s on Spectrum fabric: %w", vrfName, err)
	}

	logger.Info().
		Str("vrf_name", vrfName).
		Msg("VRF created on Spectrum fabric")

	return nil
}

// RemoveVPCFromFabric removes the VRF from the Spectrum switches that
// corresponds to the NICo VPC. Uses the NVUE revision-based workflow:
//
//  1. Create a new NVUE revision
//  2. DELETE /nvue_v1/vrf/nico-{vpcID}
//  3. Apply (commit) the revision
//  4. Poll until the apply completes
//
// This operation is idempotent — if the VRF does not exist, NVUE treats
// the DELETE as a no-op.
func (p *SpectrumFabricProvider) RemoveVPCFromFabric(ctx context.Context, vpcID string) error {
	logger := log.With().Str("provider", p.Name()).Str("vpc_id", vpcID).Logger()

	if !p.config.Features.SyncVPC {
		logger.Debug().Msg("VPC sync disabled, skipping removal")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	vrfName := fmt.Sprintf("nico-%s", vpcID)

	logger.Info().
		Str("vrf_name", vrfName).
		Msg("removing VRF from Spectrum fabric via NVUE")

	revID, err := p.client.CreateRevisionID(ctx)
	if err != nil {
		return fmt.Errorf("creating NVUE revision: %w", err)
	}

	if err := p.client.DeleteVRF(ctx, vrfName, revID); err != nil {
		return fmt.Errorf("deleting VRF %s: %w", vrfName, err)
	}

	if _, err := p.client.ApplyAndWait(ctx, revID); err != nil {
		return fmt.Errorf("applying VRF deletion for %s: %w", vrfName, err)
	}

	logger.Info().
		Str("vrf_name", vrfName).
		Msg("VRF removed from Spectrum fabric")

	return nil
}

// SyncSubnetToFabric creates VxLAN VNI and bridge VLAN configuration on
// the Spectrum switches for the given NICo subnet. Uses the NVUE
// revision-based workflow with multiple patches in a single revision:
//
//  1. Create a new NVUE revision
//  2. PATCH /nvue_v1/bridge/domain/br_default with VLAN-VNI mapping
//  3. PATCH /nvue_v1/interface/vlan{vid} with SVI and gateway IP
//  4. PATCH /nvue_v1/nve/vxlan with VNI flooding config
//  5. Apply (commit) the revision
//  6. Poll until the apply completes
//
// NVUE object model paths involved:
//   - /bridge/domain/{name}/vlan/{vid}: VLAN configuration in bridge domain
//   - /bridge/domain/{name}/vlan/{vid}/vni: VLAN-to-VNI mapping
//   - /interface/vlan{vid}: SVI for the VLAN (gateway interface)
//   - /interface/vlan{vid}/ip/address: gateway IP address on the SVI
//   - /nve/vxlan: NVE VxLAN endpoint configuration
//
// This operation is idempotent — if the VNI already exists, NVUE treats
// the PATCH as a no-op.
func (p *SpectrumFabricProvider) SyncSubnetToFabric(ctx context.Context, subnetID, vpcID, prefix, subnetName string, vlanID, vni int) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Str("vpc_id", vpcID).Logger()

	if !p.config.Features.SyncSubnet {
		logger.Debug().Msg("subnet sync disabled, skipping")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	vrfName := fmt.Sprintf("nico-%s", vpcID)
	sviName := fmt.Sprintf("vlan%d", vlanID)

	logger.Info().
		Str("prefix", prefix).
		Str("vrf_name", vrfName).
		Int("vlan_id", vlanID).
		Int("vni", vni).
		Msg("creating VxLAN VNI on Spectrum fabric via NVUE")

	// Configure bridge domain VLAN with VNI mapping.
	bridgeConfig := map[string]any{
		"vlan": map[string]any{
			fmt.Sprintf("%d", vlanID): map[string]any{
				"vni": map[string]any{
					fmt.Sprintf("%d", vni): map[string]any{
						"flooding": map[string]any{
							"enable":            "on",
							"head-end-replication": map[string]any{},
						},
					},
				},
			},
		},
	}

	// Configure SVI with gateway IP in the tenant VRF.
	sviConfig := map[string]any{
		"type": "svi",
		"ip": map[string]any{
			"address": map[string]any{
				prefix: map[string]any{},
			},
			"vrf": vrfName,
		},
	}

	// Configure NVE VxLAN endpoint.
	nveConfig := map[string]any{
		"enable": "on",
	}

	_, err := p.client.ConfigureAndApply(ctx, []nvue.PatchOp{
		{Path: "/bridge/domain/br_default", Payload: bridgeConfig},
		{Path: fmt.Sprintf("/interface/%s", sviName), Payload: sviConfig},
		{Path: "/nve/vxlan", Payload: nveConfig},
	})
	if err != nil {
		return fmt.Errorf("creating VxLAN VNI %d for subnet %s: %w", vni, subnetID, err)
	}

	logger.Info().
		Str("prefix", prefix).
		Int("vni", vni).
		Msg("VxLAN VNI created on Spectrum fabric")

	return nil
}

// RemoveSubnetFromFabric removes the VxLAN VNI and bridge VLAN
// configuration from the Spectrum switches for the given NICo subnet.
// Uses the NVUE revision-based workflow:
//
//  1. Create a new NVUE revision
//  2. DELETE /nvue_v1/interface/vlan{vid} (remove SVI)
//  3. DELETE /nvue_v1/bridge/domain/br_default/vlan/{vid} (remove VLAN from bridge)
//  4. Apply (commit) the revision
//  5. Poll until the apply completes
//
// This operation is idempotent — if the VNI does not exist, NVUE treats
// the DELETE as a no-op.
func (p *SpectrumFabricProvider) RemoveSubnetFromFabric(ctx context.Context, subnetID, vpcID string, vlanID int) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Logger()

	if !p.config.Features.SyncSubnet {
		logger.Debug().Msg("subnet sync disabled, skipping removal")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	sviName := fmt.Sprintf("vlan%d", vlanID)

	logger.Info().
		Int("vlan_id", vlanID).
		Msg("removing VxLAN VNI from Spectrum fabric via NVUE")

	revID, err := p.client.CreateRevisionID(ctx)
	if err != nil {
		return fmt.Errorf("creating NVUE revision: %w", err)
	}

	// Remove SVI interface.
	if err := p.client.DeleteInterface(ctx, sviName, revID); err != nil {
		return fmt.Errorf("deleting SVI %s: %w", sviName, err)
	}

	// Remove VLAN from bridge domain.
	bridgeVLANPath := fmt.Sprintf("/bridge/domain/br_default/vlan/%d", vlanID)
	if err := p.client.Delete(ctx, bridgeVLANPath, revID); err != nil {
		return fmt.Errorf("deleting bridge VLAN %d: %w", vlanID, err)
	}

	if _, err := p.client.ApplyAndWait(ctx, revID); err != nil {
		return fmt.Errorf("applying VxLAN VNI removal for subnet %s: %w", subnetID, err)
	}

	logger.Info().
		Str("subnet_id", subnetID).
		Int("vlan_id", vlanID).
		Msg("VxLAN VNI removed from Spectrum fabric")

	return nil
}
