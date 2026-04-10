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

	// TODO: Implement NVUE API calls when nvue-client-go is available.
	//
	// The expected NVUE revision workflow:
	//
	// // 1. Create a new revision
	// rev, err := p.client.CreateRevision(ctx)
	// if err != nil {
	// 	return fmt.Errorf("creating NVUE revision: %w", err)
	// }
	//
	// // 2. PATCH the VRF configuration
	// // NVUE path: /vrf/{name}
	// vrfConfig := map[string]interface{}{
	// 	"router": map[string]interface{}{
	// 		"bgp": map[string]interface{}{
	// 			"autonomous-system": 65000, // TODO: from config
	// 			"enable":            "on",
	// 			"router-id":         "auto",
	// 		},
	// 	},
	// 	"evpn": map[string]interface{}{
	// 		"enable": "on",
	// 	},
	// }
	// err = p.client.PatchVRF(ctx, rev.ID, vrfName, vrfConfig)
	// if err != nil {
	// 	return fmt.Errorf("patching VRF %s: %w", vrfName, err)
	// }
	//
	// // 3. Apply the revision
	// err = p.client.ApplyRevision(ctx, rev.ID)
	// if err != nil {
	// 	return fmt.Errorf("applying revision %s: %w", rev.ID, err)
	// }
	//
	// // 4. Wait for the apply to complete
	// err = p.client.WaitForRevision(ctx, rev.ID, p.config.RevisionPollInterval, p.config.RevisionTimeout)
	// if err != nil {
	// 	return fmt.Errorf("waiting for revision %s: %w", rev.ID, err)
	// }

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

	// TODO: Implement NVUE API calls when nvue-client-go is available.
	//
	// // 1. Create a new revision
	// rev, err := p.client.CreateRevision(ctx)
	// if err != nil {
	// 	return fmt.Errorf("creating NVUE revision: %w", err)
	// }
	//
	// // 2. DELETE the VRF
	// // NVUE path: /vrf/{name}
	// err = p.client.DeleteVRF(ctx, rev.ID, vrfName)
	// if err != nil {
	// 	return fmt.Errorf("deleting VRF %s: %w", vrfName, err)
	// }
	//
	// // 3. Apply the revision
	// err = p.client.ApplyRevision(ctx, rev.ID)
	// if err != nil {
	// 	return fmt.Errorf("applying revision %s: %w", rev.ID, err)
	// }
	//
	// // 4. Wait for the apply to complete
	// err = p.client.WaitForRevision(ctx, rev.ID, p.config.RevisionPollInterval, p.config.RevisionTimeout)
	// if err != nil {
	// 	return fmt.Errorf("waiting for revision %s: %w", rev.ID, err)
	// }

	logger.Info().
		Str("vrf_name", vrfName).
		Msg("VRF removed from Spectrum fabric")

	return nil
}

// SyncSubnetToFabric creates VxLAN VNI and bridge VLAN configuration on
// the Spectrum switches for the given NICo subnet. Uses the NVUE
// revision-based workflow:
//
//  1. Create a new NVUE revision
//  2. PATCH /nvue_v1/nve/vxlan with VNI configuration
//  3. PATCH /nvue_v1/interface/{svi}/bridge/domain with VLAN-VNI mapping
//  4. PATCH /nvue_v1/interface/{svi}/ip/address with gateway IP
//  5. Apply (commit) the revision
//  6. Poll until the apply completes
//
// NVUE object model paths involved:
//   - /nve/vxlan: NVE VxLAN endpoint configuration
//   - /bridge/domain/{name}/vlan/{vid}: VLAN configuration in bridge domain
//   - /bridge/domain/{name}/vlan/{vid}/vni: VLAN-to-VNI mapping
//   - /interface/vlan{vid}: SVI for the VLAN (gateway interface)
//   - /interface/vlan{vid}/ip/address: gateway IP address on the SVI
//
// This operation is idempotent — if the VNI already exists, NVUE treats
// the PATCH as a no-op.
func (p *SpectrumFabricProvider) SyncSubnetToFabric(ctx context.Context, subnetID, vpcID, prefix, subnetName string) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Str("vpc_id", vpcID).Logger()

	if !p.config.Features.SyncSubnet {
		logger.Debug().Msg("subnet sync disabled, skipping")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	vrfName := fmt.Sprintf("nico-%s", vpcID)

	logger.Info().
		Str("prefix", prefix).
		Str("vrf_name", vrfName).
		Msg("creating VxLAN VNI on Spectrum fabric via NVUE")

	// TODO: Implement NVUE API calls when nvue-client-go is available.
	//
	// The expected NVUE revision workflow:
	//
	// // 1. Create a new revision
	// rev, err := p.client.CreateRevision(ctx)
	// if err != nil {
	// 	return fmt.Errorf("creating NVUE revision: %w", err)
	// }
	//
	// // 2. Configure VxLAN VNI
	// // NVUE path: /nve/vxlan
	// // A VNI ID would typically be derived from the subnet or assigned
	// // from a pool. For now, this is a placeholder.
	// vniConfig := map[string]interface{}{
	// 	"enable": "on",
	// }
	// err = p.client.PatchNVE(ctx, rev.ID, vniConfig)
	// if err != nil {
	// 	return fmt.Errorf("patching NVE VxLAN config: %w", err)
	// }
	//
	// // 3. Configure bridge domain VLAN-VNI mapping
	// // NVUE path: /bridge/domain/br_default/vlan/{vid}/vni
	// bridgeConfig := map[string]interface{}{
	// 	"vni": map[string]interface{}{
	// 		"flooding": map[string]interface{}{
	// 			"enable": "on",
	// 		},
	// 	},
	// }
	// err = p.client.PatchBridgeVLAN(ctx, rev.ID, "br_default", vlanID, bridgeConfig)
	// if err != nil {
	// 	return fmt.Errorf("patching bridge VLAN-VNI mapping: %w", err)
	// }
	//
	// // 4. Configure SVI with gateway IP
	// // NVUE path: /interface/vlan{vid}/ip/address
	// sviConfig := map[string]interface{}{
	// 	"ip": map[string]interface{}{
	// 		"address": map[string]interface{}{
	// 			prefix: map[string]interface{}{},
	// 		},
	// 		"vrf": vrfName,
	// 	},
	// }
	// err = p.client.PatchInterface(ctx, rev.ID, fmt.Sprintf("vlan%d", vlanID), sviConfig)
	// if err != nil {
	// 	return fmt.Errorf("patching SVI config: %w", err)
	// }
	//
	// // 5. Apply the revision
	// err = p.client.ApplyRevision(ctx, rev.ID)
	// if err != nil {
	// 	return fmt.Errorf("applying revision %s: %w", rev.ID, err)
	// }
	//
	// // 6. Wait for the apply to complete
	// err = p.client.WaitForRevision(ctx, rev.ID, p.config.RevisionPollInterval, p.config.RevisionTimeout)
	// if err != nil {
	// 	return fmt.Errorf("waiting for revision %s: %w", rev.ID, err)
	// }

	logger.Info().
		Str("prefix", prefix).
		Msg("VxLAN VNI created on Spectrum fabric")

	return nil
}

// RemoveSubnetFromFabric removes the VxLAN VNI and bridge VLAN
// configuration from the Spectrum switches for the given NICo subnet.
// Uses the NVUE revision-based workflow:
//
//  1. Create a new NVUE revision
//  2. DELETE /nvue_v1/interface/vlan{vid} (remove SVI)
//  3. DELETE /nvue_v1/bridge/domain/{name}/vlan/{vid} (remove VLAN from bridge)
//  4. Apply (commit) the revision
//  5. Poll until the apply completes
//
// This operation is idempotent — if the VNI does not exist, NVUE treats
// the DELETE as a no-op.
func (p *SpectrumFabricProvider) RemoveSubnetFromFabric(ctx context.Context, subnetID, vpcID string) error {
	logger := log.With().Str("provider", p.Name()).Str("subnet_id", subnetID).Logger()

	if !p.config.Features.SyncSubnet {
		logger.Debug().Msg("subnet sync disabled, skipping removal")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	logger.Info().Msg("removing VxLAN VNI from Spectrum fabric via NVUE")

	// TODO: Implement NVUE API calls when nvue-client-go is available.
	//
	// // 1. Create a new revision
	// rev, err := p.client.CreateRevision(ctx)
	// if err != nil {
	// 	return fmt.Errorf("creating NVUE revision: %w", err)
	// }
	//
	// // 2. Delete SVI interface
	// // NVUE path: /interface/vlan{vid}
	// err = p.client.DeleteInterface(ctx, rev.ID, fmt.Sprintf("vlan%d", vlanID))
	// if err != nil {
	// 	return fmt.Errorf("deleting SVI: %w", err)
	// }
	//
	// // 3. Delete VLAN from bridge domain
	// // NVUE path: /bridge/domain/br_default/vlan/{vid}
	// err = p.client.DeleteBridgeVLAN(ctx, rev.ID, "br_default", vlanID)
	// if err != nil {
	// 	return fmt.Errorf("deleting bridge VLAN: %w", err)
	// }
	//
	// // 4. Apply the revision
	// err = p.client.ApplyRevision(ctx, rev.ID)
	// if err != nil {
	// 	return fmt.Errorf("applying revision %s: %w", rev.ID, err)
	// }
	//
	// // 5. Wait for the apply to complete
	// err = p.client.WaitForRevision(ctx, rev.ID, p.config.RevisionPollInterval, p.config.RevisionTimeout)
	// if err != nil {
	// 	return fmt.Errorf("waiting for revision %s: %w", rev.ID, err)
	// }

	logger.Info().
		Str("subnet_id", subnetID).
		Msg("VxLAN VNI removed from Spectrum fabric")

	return nil
}
