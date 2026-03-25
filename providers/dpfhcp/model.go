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

package dpfhcp

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// DefaultNamespace is the default Kubernetes namespace for DPF HCP Provisioner CRs.
const DefaultNamespace = "dpf-system"

// crName derives the CR name from a site ID.
func crName(siteID string) string {
	return "dpfhcp-" + siteID
}

// DPUClusterReference identifies a DPU cluster by name and namespace.
type DPUClusterReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// DPUDeploymentReference identifies a DPU deployment by name and namespace.
type DPUDeploymentReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// DPFHCPRequest is the JSON body for provisioning a DPF hosted control plane.
type DPFHCPRequest struct {
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

// ProvisioningStatus represents the lifecycle state of a DPF HCP provisioning record.
type ProvisioningStatus string

const (
	StatusPending      ProvisioningStatus = "Pending"
	StatusProvisioning ProvisioningStatus = "Provisioning"
	StatusReady        ProvisioningStatus = "Ready"
	StatusFailed       ProvisioningStatus = "Failed"
	StatusDeleting     ProvisioningStatus = "Deleting"
)

// ProvisioningRecord tracks the state of a DPF HCP provisioning request for a site.
type ProvisioningRecord struct {
	SiteID      string             `json:"siteId"`
	Config      DPFHCPRequest      `json:"config"`
	Status      ProvisioningStatus `json:"status"`
	Phase       string             `json:"phase,omitempty"`
	Conditions  []StatusCondition  `json:"conditions,omitempty"`
	WorkflowID  string             `json:"workflowId,omitempty"`
	CRName      string             `json:"crName,omitempty"`
	CRNamespace string             `json:"crNamespace,omitempty"`
	Created     time.Time          `json:"created"`
	Updated     time.Time          `json:"updated"`
}

// CreateProvisionerCR creates a DPFHCPProvisioner CR using the site ID to derive
// the CR name and the default namespace.
func (c *DPFHCPClient) CreateProvisionerCR(ctx context.Context, siteID string, config DPFHCPRequest) error {
	name := crName(siteID)
	return c.CreateProvisioner(ctx, name, DefaultNamespace, config)
}

// DeleteProvisionerCR deletes the DPFHCPProvisioner CR for the given site.
// Returns nil if the CR does not exist (idempotent).
func (c *DPFHCPClient) DeleteProvisionerCR(ctx context.Context, siteID string) error {
	name := crName(siteID)
	err := c.DeleteProvisioner(ctx, name, DefaultNamespace)
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

// WaitForPhase watches the DPFHCPProvisioner CR for the given site until it
// reaches the target phase.
func (c *DPFHCPClient) WaitForPhase(ctx context.Context, siteID string, targetPhase string) error {
	name := crName(siteID)
	_, err := c.WatchProvisionerPhase(ctx, name, DefaultNamespace, targetPhase)
	return err
}

// WaitForDeletion polls the Kubernetes API until the DPFHCPProvisioner CR
// for the given site no longer exists.
func (c *DPFHCPClient) WaitForDeletion(ctx context.Context, siteID string) error {
	name := crName(siteID)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := c.GetProvisioner(ctx, name, DefaultNamespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			// Transient errors; retry
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// WaitForDeletionError is returned when deletion does not complete in time.
type WaitForDeletionError struct {
	SiteID string
}

func (e *WaitForDeletionError) Error() string {
	return fmt.Sprintf("timed out waiting for DPFHCPProvisioner CR deletion for site %s", e.SiteID)
}
