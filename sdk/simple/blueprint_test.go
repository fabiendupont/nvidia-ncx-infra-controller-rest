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

package simple

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard"
)

func TestBlueprintFromStandard(t *testing.T) {
	t.Run("full blueprint conversion", func(t *testing.T) {
		now := time.Now().UTC()
		desc := "A test blueprint"
		basedOn := "parent-bp"
		tenantID := "tenant-123"

		api := standard.Blueprint{
			Id:          "bp-001",
			Name:        "gpu-cluster",
			Version:     "1.0.0",
			Description: &desc,
			Parameters: map[string]standard.BlueprintParameter{
				"gpu_count": {Name: StringPtr("gpu_count")},
			},
			Resources: map[string]standard.BlueprintResource{
				"compute": {Type: StringPtr("nico/instance")},
			},
			Labels:     map[string]string{"env": "prod"},
			TenantId:   &tenantID,
			Visibility: "organization",
			BasedOn:    &basedOn,
			IsActive:   true,
			Created:    &now,
			Updated:    &now,
		}

		b := blueprintFromStandard(api)

		assert.Equal(t, "bp-001", b.ID)
		assert.Equal(t, "gpu-cluster", b.Name)
		assert.Equal(t, "1.0.0", b.Version)
		assert.Equal(t, "A test blueprint", b.Description)
		assert.Contains(t, b.Parameters, "gpu_count")
		assert.Contains(t, b.Resources, "compute")
		assert.Equal(t, "prod", b.Labels["env"])
		assert.Equal(t, &tenantID, b.TenantID)
		assert.Equal(t, "organization", b.Visibility)
		assert.Equal(t, "parent-bp", b.BasedOn)
		assert.True(t, b.IsActive)
		assert.Equal(t, now, b.Created)
		assert.Equal(t, now, b.Updated)
	})

	t.Run("minimal blueprint with nil optional fields", func(t *testing.T) {
		api := standard.Blueprint{
			Id:         "bp-002",
			Name:       "simple",
			Version:    "0.1.0",
			Visibility: "public",
			IsActive:   false,
		}

		b := blueprintFromStandard(api)

		assert.Equal(t, "bp-002", b.ID)
		assert.Equal(t, "simple", b.Name)
		assert.Empty(t, b.Description)
		assert.Nil(t, b.TenantID)
		assert.Empty(t, b.BasedOn)
		assert.True(t, b.Created.IsZero())
	})
}

func TestToStandardCreateBlueprintRequest(t *testing.T) {
	t.Run("full create request", func(t *testing.T) {
		req := BlueprintCreateRequest{
			Name:        "my-bp",
			Version:     "2.0.0",
			Description: "desc",
			Visibility:  "private",
			BasedOn:     "parent",
			Labels:      map[string]string{"tier": "gold"},
		}

		apiReq := toStandardCreateBlueprintRequest(req)

		assert.Equal(t, "my-bp", apiReq.Name)
		assert.Equal(t, "2.0.0", apiReq.Version)
		assert.NotNil(t, apiReq.Description)
		assert.Equal(t, "desc", *apiReq.Description)
		assert.NotNil(t, apiReq.Visibility)
		assert.Equal(t, "private", *apiReq.Visibility)
		assert.NotNil(t, apiReq.BasedOn)
		assert.Equal(t, "parent", *apiReq.BasedOn)
		assert.Equal(t, "gold", apiReq.Labels["tier"])
	})

	t.Run("minimal create request leaves optional fields nil", func(t *testing.T) {
		req := BlueprintCreateRequest{
			Name:    "bare",
			Version: "1.0.0",
		}

		apiReq := toStandardCreateBlueprintRequest(req)

		assert.Equal(t, "bare", apiReq.Name)
		assert.Nil(t, apiReq.Description)
		assert.Nil(t, apiReq.Visibility)
		assert.Nil(t, apiReq.BasedOn)
		assert.Nil(t, apiReq.Labels)
	})
}

func TestToStandardUpdateBlueprintRequest(t *testing.T) {
	t.Run("partial update sets only provided fields", func(t *testing.T) {
		req := BlueprintUpdateRequest{
			Name: "new-name",
		}

		apiReq := toStandardUpdateBlueprintRequest(req)

		assert.NotNil(t, apiReq.Name)
		assert.Equal(t, "new-name", *apiReq.Name)
		assert.Nil(t, apiReq.Version)
		assert.Nil(t, apiReq.Description)
		assert.Nil(t, apiReq.Visibility)
	})
}
