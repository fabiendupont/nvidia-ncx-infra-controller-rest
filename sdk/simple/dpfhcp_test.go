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

func TestDPFHCPRecordFromStandard(t *testing.T) {
	t.Run("full provisioning record conversion", func(t *testing.T) {
		now := time.Now().UTC()
		siteID := "site-001"
		status := "Provisioning"
		phase := "Installing"
		wfID := "wf-dpf-001"
		crName := "dpfhcp-site-001"
		crNamespace := "dpf-system"

		baseDomain := "example.com"
		ocpRelease := "quay.io/openshift-release-dev/ocp-release:4.14.0"
		sshRef := "ssh-secret"
		pullRef := "pull-secret"

		api := standard.ProvisioningRecord{
			SiteId: &siteID,
			Config: &standard.DPFHCPRequest{
				DpuClusterRef:   standard.DPUClusterReference{Name: "dpu-cluster", Namespace: "dpu-ns"},
				BaseDomain:      baseDomain,
				OcpReleaseImage: ocpRelease,
				SshKeySecretRef: sshRef,
				PullSecretRef:   pullRef,
			},
			Status:      &status,
			Phase:       &phase,
			WorkflowId:  &wfID,
			CrName:      &crName,
			CrNamespace: &crNamespace,
			Created:     &now,
			Updated:     &now,
		}

		r := dpfhcpRecordFromStandard(api)

		assert.Equal(t, "site-001", r.SiteID)
		assert.Equal(t, "Provisioning", r.Status)
		assert.Equal(t, "Installing", r.Phase)
		assert.Equal(t, "wf-dpf-001", r.WorkflowID)
		assert.Equal(t, "dpfhcp-site-001", r.CRName)
		assert.Equal(t, "dpf-system", r.CRNamespace)
		assert.Equal(t, "dpu-cluster", r.Config.DPUClusterRef.Name)
		assert.Equal(t, "dpu-ns", r.Config.DPUClusterRef.Namespace)
		assert.Equal(t, "example.com", r.Config.BaseDomain)
		assert.Equal(t, now, r.Created)
	})

	t.Run("minimal record with nil optional fields", func(t *testing.T) {
		siteID := "site-002"
		status := "Pending"

		api := standard.ProvisioningRecord{
			SiteId: &siteID,
			Status: &status,
		}

		r := dpfhcpRecordFromStandard(api)

		assert.Equal(t, "site-002", r.SiteID)
		assert.Equal(t, "Pending", r.Status)
		assert.Empty(t, r.Phase)
		assert.Empty(t, r.WorkflowID)
		assert.True(t, r.Created.IsZero())
	})
}

func TestToStandardDPFHCPRequest(t *testing.T) {
	t.Run("full request conversion", func(t *testing.T) {
		enabled := true
		req := DPFHCPProvisionRequest{
			DPUClusterRef:                  DPUClusterReference{Name: "cluster-1", Namespace: "ns-1"},
			BaseDomain:                     "test.example.com",
			OCPReleaseImage:                "quay.io/ocp:4.14",
			SSHKeySecretRef:                "my-ssh",
			PullSecretRef:                  "my-pull",
			ControlPlaneAvailabilityPolicy: "HighlyAvailable",
			VirtualIP:                      "10.0.0.1",
			EtcdStorageClass:               "gp3",
			FlannelEnabled:                 &enabled,
			MachineOSURL:                   "http://os.example.com/image",
		}

		apiReq := toStandardDPFHCPRequest(req)

		assert.Equal(t, "cluster-1", apiReq.DpuClusterRef.Name)
		assert.Equal(t, "test.example.com", apiReq.BaseDomain)
		assert.Equal(t, "quay.io/ocp:4.14", apiReq.OcpReleaseImage)
		assert.NotNil(t, apiReq.ControlPlaneAvailabilityPolicy)
		assert.Equal(t, "HighlyAvailable", *apiReq.ControlPlaneAvailabilityPolicy)
		assert.NotNil(t, apiReq.FlannelEnabled)
		assert.True(t, *apiReq.FlannelEnabled)
	})

	t.Run("minimal request leaves optional fields nil", func(t *testing.T) {
		req := DPFHCPProvisionRequest{
			DPUClusterRef:   DPUClusterReference{Name: "c", Namespace: "n"},
			BaseDomain:      "a.com",
			OCPReleaseImage: "img",
			SSHKeySecretRef: "ssh",
			PullSecretRef:   "pull",
		}

		apiReq := toStandardDPFHCPRequest(req)

		assert.Nil(t, apiReq.ControlPlaneAvailabilityPolicy)
		assert.Nil(t, apiReq.VirtualIP)
		assert.Nil(t, apiReq.EtcdStorageClass)
		assert.Nil(t, apiReq.FlannelEnabled)
		assert.Nil(t, apiReq.MachineOSURL)
	})
}
