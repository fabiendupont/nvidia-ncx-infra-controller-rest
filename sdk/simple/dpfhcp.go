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
	"context"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard"
)

// DPFHCPProvisionRequest is a request to provision a DPF hosted control plane.
type DPFHCPProvisionRequest struct {
	DPUClusterRef                  DPUClusterReference     `json:"dpuClusterRef"`
	BaseDomain                     string                  `json:"baseDomain"`
	OCPReleaseImage                string                  `json:"ocpReleaseImage"`
	SSHKeySecretRef                string                  `json:"sshKeySecretRef"`
	PullSecretRef                  string                  `json:"pullSecretRef"`
	ControlPlaneAvailabilityPolicy string                  `json:"controlPlaneAvailabilityPolicy,omitempty"`
	VirtualIP                      string                  `json:"virtualIP,omitempty"`
	EtcdStorageClass               string                  `json:"etcdStorageClass,omitempty"`
	FlannelEnabled                 *bool                   `json:"flannelEnabled,omitempty"`
	DPUDeploymentRef               *DPUDeploymentReference `json:"dpuDeploymentRef,omitempty"`
	MachineOSURL                   string                  `json:"machineOSURL,omitempty"`
}

// DPUClusterReference identifies a DPU cluster by name and namespace.
type DPUClusterReference = standard.DPUClusterReference

// DPUDeploymentReference identifies a DPU deployment by name and namespace.
type DPUDeploymentReference = standard.DPUDeploymentReference

// DPFHCPProvisioningRecord tracks the state of a DPF HCP provisioning request.
type DPFHCPProvisioningRecord struct {
	SiteID      string            `json:"siteId"`
	Config      DPFHCPProvisionRequest `json:"config"`
	Status      string            `json:"status"`
	Phase       string            `json:"phase,omitempty"`
	WorkflowID  string            `json:"workflowId,omitempty"`
	CRName      string            `json:"crName,omitempty"`
	CRNamespace string            `json:"crNamespace,omitempty"`
	Created     time.Time         `json:"created"`
	Updated     time.Time         `json:"updated"`
}

// DPFHCPManager manages DPF HCP provisioning operations.
type DPFHCPManager struct {
	client *Client
}

// NewDPFHCPManager creates a new DPFHCPManager.
func NewDPFHCPManager(client *Client) DPFHCPManager {
	return DPFHCPManager{client: client}
}

func dpfhcpRecordFromStandard(api standard.ProvisioningRecord) DPFHCPProvisioningRecord {
	r := DPFHCPProvisioningRecord{}
	if api.SiteId != nil {
		r.SiteID = *api.SiteId
	}
	if api.Status != nil {
		r.Status = *api.Status
	}
	if api.Phase != nil {
		r.Phase = *api.Phase
	}
	if api.WorkflowId != nil {
		r.WorkflowID = *api.WorkflowId
	}
	if api.CrName != nil {
		r.CRName = *api.CrName
	}
	if api.CrNamespace != nil {
		r.CRNamespace = *api.CrNamespace
	}
	if api.Created != nil {
		r.Created = *api.Created
	}
	if api.Updated != nil {
		r.Updated = *api.Updated
	}
	if api.Config != nil {
		c := api.Config
		r.Config = DPFHCPProvisionRequest{
			DPUClusterRef:   c.DpuClusterRef,
			BaseDomain:      c.GetBaseDomain(),
			OCPReleaseImage: c.GetOcpReleaseImage(),
			SSHKeySecretRef: c.GetSshKeySecretRef(),
			PullSecretRef:   c.GetPullSecretRef(),
		}
		if c.ControlPlaneAvailabilityPolicy != nil {
			r.Config.ControlPlaneAvailabilityPolicy = *c.ControlPlaneAvailabilityPolicy
		}
		if c.VirtualIP != nil {
			r.Config.VirtualIP = *c.VirtualIP
		}
		if c.EtcdStorageClass != nil {
			r.Config.EtcdStorageClass = *c.EtcdStorageClass
		}
		r.Config.FlannelEnabled = c.FlannelEnabled
		r.Config.DPUDeploymentRef = c.DpuDeploymentRef
		if c.MachineOSURL != nil {
			r.Config.MachineOSURL = *c.MachineOSURL
		}
	}
	return r
}

func toStandardDPFHCPRequest(req DPFHCPProvisionRequest) standard.DPFHCPRequest {
	apiReq := standard.DPFHCPRequest{
		DpuClusterRef:   req.DPUClusterRef,
		BaseDomain:      req.BaseDomain,
		OcpReleaseImage: req.OCPReleaseImage,
		SshKeySecretRef: req.SSHKeySecretRef,
		PullSecretRef:   req.PullSecretRef,
	}
	if req.ControlPlaneAvailabilityPolicy != "" {
		apiReq.ControlPlaneAvailabilityPolicy = &req.ControlPlaneAvailabilityPolicy
	}
	if req.VirtualIP != "" {
		apiReq.VirtualIP = &req.VirtualIP
	}
	if req.EtcdStorageClass != "" {
		apiReq.EtcdStorageClass = &req.EtcdStorageClass
	}
	apiReq.FlannelEnabled = req.FlannelEnabled
	apiReq.DpuDeploymentRef = req.DPUDeploymentRef
	if req.MachineOSURL != "" {
		apiReq.MachineOSURL = &req.MachineOSURL
	}
	return apiReq
}

// ProvisionDPFHCP provisions a DPF hosted control plane for a site.
func (dm DPFHCPManager) ProvisionDPFHCP(ctx context.Context, siteID string, request DPFHCPProvisionRequest) (*DPFHCPProvisioningRecord, *ApiError) {
	ctx = WithLogger(ctx, dm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, dm.client.Config.Token)

	apiReq := toStandardDPFHCPRequest(request)
	result, resp, err := dm.client.apiClient.DPFHCPAPI.ProvisionDpfHcp(ctx, dm.client.apiMetadata.Organization, siteID).
		DPFHCPRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	r := dpfhcpRecordFromStandard(*result)
	return &r, nil
}

// GetDPFHCPStatus returns the provisioning status of a DPF HCP for a site.
func (dm DPFHCPManager) GetDPFHCPStatus(ctx context.Context, siteID string) (*DPFHCPProvisioningRecord, *ApiError) {
	ctx = WithLogger(ctx, dm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, dm.client.Config.Token)

	result, resp, err := dm.client.apiClient.DPFHCPAPI.GetDpfHcpStatus(ctx, dm.client.apiMetadata.Organization, siteID).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	r := dpfhcpRecordFromStandard(*result)
	return &r, nil
}

// DeleteDPFHCP initiates teardown of a DPF HCP for a site.
func (dm DPFHCPManager) DeleteDPFHCP(ctx context.Context, siteID string) *ApiError {
	ctx = WithLogger(ctx, dm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, dm.client.Config.Token)

	resp, err := dm.client.apiClient.DPFHCPAPI.DeleteDpfHcp(ctx, dm.client.apiMetadata.Organization, siteID).Execute()
	return HandleResponseError(resp, err)
}
