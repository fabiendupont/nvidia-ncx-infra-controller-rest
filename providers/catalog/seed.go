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

package catalog

import (
	"github.com/rs/zerolog/log"
)

// SeedBlueprints returns a set of demonstration blueprints that showcase
// the catalog's capabilities: atomic blueprints wrapping infrastructure,
// and a composed blueprint referencing them.
func SeedBlueprints() []*Blueprint {
	return []*Blueprint{
		{
			Name:        "GPU 4xA100 Slice",
			Version:     "1.0.0",
			Description: "Four A100 80GB GPUs with NVLink interconnect, dedicated subnet, and security group.",
			Parameters: map[string]BlueprintParameter{
				"name": {Name: "name", Type: "string", Required: true, Description: "Name for the GPU slice"},
			},
			Resources: map[string]BlueprintResource{
				"subnet": {
					Type:       "nico/subnet",
					Properties: map[string]interface{}{"cidr": "10.200.0.0/28"},
				},
				"nsg": {
					Type:      "nico/network-security-group",
					DependsOn: []string{"subnet"},
					Properties: map[string]interface{}{
						"rules": []interface{}{
							map[string]interface{}{"protocol": "tcp", "port": 22, "source": "10.0.0.0/8"},
						},
					},
				},
				"gpu-0": {
					Type:      "nico/instance",
					DependsOn: []string{"subnet", "nsg"},
					Properties: map[string]interface{}{
						"instance_type":  "gpu-a100-80g",
						"subnet":         "{{ subnet.id }}",
						"security_group": "{{ nsg.id }}",
					},
				},
				"gpu-1": {
					Type:      "nico/instance",
					DependsOn: []string{"subnet", "nsg"},
					Properties: map[string]interface{}{
						"instance_type":  "gpu-a100-80g",
						"subnet":         "{{ subnet.id }}",
						"security_group": "{{ nsg.id }}",
					},
				},
				"gpu-2": {
					Type:      "nico/instance",
					DependsOn: []string{"subnet", "nsg"},
					Properties: map[string]interface{}{
						"instance_type":  "gpu-a100-80g",
						"subnet":         "{{ subnet.id }}",
						"security_group": "{{ nsg.id }}",
					},
				},
				"gpu-3": {
					Type:      "nico/instance",
					DependsOn: []string{"subnet", "nsg"},
					Properties: map[string]interface{}{
						"instance_type":  "gpu-a100-80g",
						"subnet":         "{{ subnet.id }}",
						"security_group": "{{ nsg.id }}",
					},
				},
				"nvlink": {
					Type:      "nico/nvlink-partition",
					DependsOn: []string{"gpu-0", "gpu-1", "gpu-2", "gpu-3"},
					Properties: map[string]interface{}{
						"instances": []interface{}{
							"{{ gpu-0.id }}", "{{ gpu-1.id }}",
							"{{ gpu-2.id }}", "{{ gpu-3.id }}",
						},
					},
				},
			},
			Labels:     map[string]string{"category": "compute", "gpu": "a100", "tier": "standard"},
			Pricing:    &PricingSpec{Rate: 10.00, Unit: "hour", Currency: "USD"},
			Visibility: VisibilityPublic,
		},
		{
			Name:        "Storage 200GB",
			Version:     "1.0.0",
			Description: "200GB high-performance block storage allocation.",
			Parameters: map[string]BlueprintParameter{
				"name": {Name: "name", Type: "string", Required: true, Description: "Name for the storage allocation"},
			},
			Resources: map[string]BlueprintResource{
				"storage": {
					Type: "nico/allocation",
					Properties: map[string]interface{}{
						"size_gb":      200,
						"storage_tier": "high-iops",
					},
				},
			},
			Labels:     map[string]string{"category": "storage", "tier": "standard"},
			Pricing:    &PricingSpec{Rate: 3.00, Unit: "hour", Currency: "USD"},
			Visibility: VisibilityPublic,
		},
		{
			Name:        "PyTorch Stack",
			Version:     "1.0.0",
			Description: "Pre-configured PyTorch environment with CUDA drivers and NCCL.",
			Parameters: map[string]BlueprintParameter{
				"pytorch_version": {
					Name: "pytorch_version", Type: "string", Required: false,
					Default: "2.3", Enum: []string{"2.1", "2.2", "2.3"},
					Description: "PyTorch version",
				},
			},
			Resources: map[string]BlueprintResource{
				"software": {
					Type: "nico/allocation",
					Properties: map[string]interface{}{
						"type":    "software-stack",
						"image":   "pytorch:{{ pytorch_version }}-cuda12",
						"drivers": "cuda-12.4",
					},
				},
			},
			Labels:     map[string]string{"category": "software", "framework": "pytorch"},
			Pricing:    &PricingSpec{Rate: 2.00, Unit: "hour", Currency: "USD"},
			Visibility: VisibilityPublic,
		},
	}
}

// SeedComposedBlueprint returns a composed blueprint that references the
// given atomic blueprint IDs. Call this after seeding the atomic blueprints
// to get their generated IDs.
func SeedComposedBlueprint(gpuID, storageID, pytorchID string) *Blueprint {
	return &Blueprint{
		Name:        "AI Standard Workstation",
		Version:     "1.0.0",
		Description: "Complete ML workstation: 4xA100 GPUs + 200GB storage + PyTorch. Ready for distributed training.",
		Parameters: map[string]BlueprintParameter{
			"name": {Name: "name", Type: "string", Required: true, Description: "Name for the workstation"},
			"pytorch_version": {
				Name: "pytorch_version", Type: "string", Required: false,
				Default: "2.3", Description: "PyTorch version",
			},
		},
		Resources: map[string]BlueprintResource{
			"gpu": {
				Type: "blueprint/" + gpuID,
			},
			"storage": {
				Type:      "blueprint/" + storageID,
				DependsOn: []string{"gpu"},
			},
			"pytorch": {
				Type:      "blueprint/" + pytorchID,
				DependsOn: []string{"gpu"},
				Properties: map[string]interface{}{
					"pytorch_version": "{{ pytorch_version }}",
				},
			},
		},
		Labels:     map[string]string{"category": "workstation", "tier": "standard", "gpu": "a100"},
		Pricing:    &PricingSpec{Rate: 15.00, Unit: "hour", Currency: "USD"},
		Visibility: VisibilityPublic,
	}
}

// LoadSeedData populates the blueprint store with demonstration blueprints
// if it is empty. Safe to call on every startup — skips if data already exists.
func (p *CatalogProvider) LoadSeedData() {
	existing := p.blueprintStore.GetAll()
	if len(existing) > 0 {
		return
	}

	logger := log.With().Str("provider", "nico-catalog").Logger()
	logger.Info().Msg("loading seed blueprints")

	atomics := SeedBlueprints()
	var ids []string
	for _, b := range atomics {
		if err := p.blueprintStore.Create(b); err != nil {
			logger.Warn().Err(err).Str("blueprint", b.Name).Msg("failed to seed blueprint")
			continue
		}
		ids = append(ids, b.ID)
		logger.Info().Str("id", b.ID).Str("name", b.Name).Msg("seeded blueprint")
	}

	if len(ids) == 3 {
		composed := SeedComposedBlueprint(ids[0], ids[1], ids[2])
		if err := p.blueprintStore.Create(composed); err != nil {
			logger.Warn().Err(err).Msg("failed to seed composed blueprint")
		} else {
			logger.Info().Str("id", composed.ID).Str("name", composed.Name).Msg("seeded composed blueprint")
		}
	}
}
