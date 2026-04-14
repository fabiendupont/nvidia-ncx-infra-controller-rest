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

func TestFaultEventFromStandard(t *testing.T) {
	t.Run("full fault event conversion", func(t *testing.T) {
		now := time.Now().UTC()
		id := "fault-001"
		orgID := "org-1"
		tenantID := "tenant-1"
		siteID := "site-1"
		machineID := "machine-1"
		instanceID := "instance-1"
		source := "ufm"
		severity := "critical"
		component := "gpu"
		classification := "gpu-xid-error"
		message := "GPU XID error 79 detected"
		state := "open"
		wfID := "wf-123"
		attempts := int32(2)
		escalation := int32(1)

		api := standard.FaultEvent{
			Id:                    &id,
			OrgId:                 &orgID,
			TenantId:              &tenantID,
			SiteId:                &siteID,
			MachineId:             &machineID,
			InstanceId:            &instanceID,
			Source:                &source,
			Severity:              &severity,
			Component:             &component,
			Classification:        &classification,
			Message:               &message,
			State:                 &state,
			DetectedAt:            &now,
			RemediationWorkflowId: &wfID,
			RemediationAttempts:   &attempts,
			EscalationLevel:       &escalation,
			Metadata:              map[string]interface{}{"xid": float64(79)},
			CreatedAt:             &now,
			UpdatedAt:             &now,
		}

		e := faultEventFromStandard(api)

		assert.Equal(t, "fault-001", e.ID)
		assert.Equal(t, "org-1", e.OrgID)
		assert.Equal(t, &tenantID, e.TenantID)
		assert.Equal(t, "site-1", e.SiteID)
		assert.Equal(t, &machineID, e.MachineID)
		assert.Equal(t, &instanceID, e.InstanceID)
		assert.Equal(t, "ufm", e.Source)
		assert.Equal(t, "critical", e.Severity)
		assert.Equal(t, "gpu", e.Component)
		assert.Equal(t, &classification, e.Classification)
		assert.Equal(t, "GPU XID error 79 detected", e.Message)
		assert.Equal(t, "open", e.State)
		assert.Equal(t, now, e.DetectedAt)
		assert.Equal(t, &wfID, e.RemediationWorkflowID)
		assert.Equal(t, 2, e.RemediationAttempts)
		assert.Equal(t, 1, e.EscalationLevel)
		assert.Equal(t, float64(79), e.Metadata["xid"])
	})

	t.Run("minimal fault event with nil optional fields", func(t *testing.T) {
		id := "fault-002"
		state := "open"
		source := "bmc"
		api := standard.FaultEvent{
			Id:     &id,
			State:  &state,
			Source: &source,
		}

		e := faultEventFromStandard(api)

		assert.Equal(t, "fault-002", e.ID)
		assert.Equal(t, "open", e.State)
		assert.Equal(t, "bmc", e.Source)
		assert.Nil(t, e.TenantID)
		assert.Nil(t, e.MachineID)
		assert.Nil(t, e.Classification)
		assert.Zero(t, e.RemediationAttempts)
	})
}
