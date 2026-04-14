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

func TestOrderFromStandard(t *testing.T) {
	t.Run("full order conversion", func(t *testing.T) {
		now := time.Now().UTC()
		id := "order-001"
		bpID := "bp-001"
		bpName := "gpu-cluster"
		tenantID := "tenant-123"
		status := "Provisioning"
		statusMsg := "Creating resources"
		wfID := "wf-abc"
		svcID := "svc-001"

		api := standard.Order{
			Id:            &id,
			BlueprintId:   &bpID,
			BlueprintName: &bpName,
			TenantId:      &tenantID,
			Parameters:    map[string]interface{}{"gpu_count": float64(8)},
			Status:        &status,
			StatusMessage: &statusMsg,
			WorkflowId:    &wfID,
			ServiceId:     &svcID,
			Created:       &now,
			Updated:       &now,
		}

		o := orderFromStandard(api)

		assert.Equal(t, "order-001", o.ID)
		assert.Equal(t, "bp-001", o.BlueprintID)
		assert.Equal(t, "gpu-cluster", o.BlueprintName)
		assert.Equal(t, "tenant-123", o.TenantID)
		assert.Equal(t, float64(8), o.Parameters["gpu_count"])
		assert.Equal(t, "Provisioning", o.Status)
		assert.Equal(t, "Creating resources", o.StatusMessage)
		assert.Equal(t, "wf-abc", o.WorkflowID)
		assert.Equal(t, &svcID, o.ServiceID)
		assert.Equal(t, now, o.Created)
	})

	t.Run("minimal order with nil optional fields", func(t *testing.T) {
		id := "order-002"
		status := "Pending"
		api := standard.Order{
			Id:     &id,
			Status: &status,
		}

		o := orderFromStandard(api)

		assert.Equal(t, "order-002", o.ID)
		assert.Equal(t, "Pending", o.Status)
		assert.Empty(t, o.BlueprintID)
		assert.Empty(t, o.TenantID)
		assert.Empty(t, o.StatusMessage)
		assert.Nil(t, o.ServiceID)
	})
}
