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

func TestFulfillmentServiceFromStandard(t *testing.T) {
	t.Run("full service conversion", func(t *testing.T) {
		now := time.Now().UTC()
		id := "svc-001"
		orderID := "order-001"
		bpID := "bp-001"
		bpName := "gpu-cluster"
		tenantID := "tenant-123"
		name := "my-cluster"
		status := "Active"

		api := standard.FulfillmentService{
			Id:            &id,
			OrderId:       &orderID,
			BlueprintId:   &bpID,
			BlueprintName: &bpName,
			TenantId:      &tenantID,
			Name:          &name,
			Status:        &status,
			Resources:     map[string]string{"vpc": "vpc-001", "instance": "inst-001"},
			Created:       &now,
			Updated:       &now,
		}

		svc := fulfillmentServiceFromStandard(api)

		assert.Equal(t, "svc-001", svc.ID)
		assert.Equal(t, "order-001", svc.OrderID)
		assert.Equal(t, "bp-001", svc.BlueprintID)
		assert.Equal(t, "gpu-cluster", svc.BlueprintName)
		assert.Equal(t, "tenant-123", svc.TenantID)
		assert.Equal(t, "my-cluster", svc.Name)
		assert.Equal(t, "Active", svc.Status)
		assert.Equal(t, "vpc-001", svc.Resources["vpc"])
		assert.Equal(t, "inst-001", svc.Resources["instance"])
		assert.Equal(t, now, svc.Created)
	})

	t.Run("minimal service with nil optional fields", func(t *testing.T) {
		id := "svc-002"
		status := "Provisioning"
		api := standard.FulfillmentService{
			Id:     &id,
			Status: &status,
		}

		svc := fulfillmentServiceFromStandard(api)

		assert.Equal(t, "svc-002", svc.ID)
		assert.Equal(t, "Provisioning", svc.Status)
		assert.Empty(t, svc.OrderID)
		assert.Empty(t, svc.Name)
		assert.Nil(t, svc.Resources)
	})
}
