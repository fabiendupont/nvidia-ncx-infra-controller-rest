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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// DPFHCPProvisioner CR types (local definitions matching the operator's v1alpha1 API)
// Group: dpf.nvidia.com, Version: v1alpha1, Resource: dpfhcpprovisioners
const (
	CRGroup    = "dpf.nvidia.com"
	CRVersion  = "v1alpha1"
	CRResource = "dpfhcpprovisioners"
)

var gvr = schema.GroupVersionResource{
	Group:    CRGroup,
	Version:  CRVersion,
	Resource: CRResource,
}

// StatusCondition represents a condition on the DPFHCPProvisioner CR.
type StatusCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
}

// ProvisionerStatus represents the observed state from the CR.
type ProvisionerStatus struct {
	Phase      string
	Conditions []StatusCondition
}

// DPFHCPClient manages DPFHCPProvisioner custom resources on the management cluster.
type DPFHCPClient struct {
	dynamicClient dynamic.Interface
}

// NewDPFHCPClient creates a client using in-cluster config.
func NewDPFHCPClient() (*DPFHCPClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("building in-cluster config: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &DPFHCPClient{dynamicClient: dynClient}, nil
}

// NewDPFHCPClientFromKubeconfig creates a client from a kubeconfig path.
func NewDPFHCPClientFromKubeconfig(kubeconfigPath string) (*DPFHCPClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("building config from kubeconfig %q: %w", kubeconfigPath, err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &DPFHCPClient{dynamicClient: dynClient}, nil
}

// CreateProvisioner creates a DPFHCPProvisioner CR from the given config.
func (c *DPFHCPClient) CreateProvisioner(ctx context.Context, name, namespace string, config DPFHCPRequest) error {
	spec := map[string]interface{}{
		"dpuClusterRef": map[string]interface{}{
			"name":      config.DPUClusterRef.Name,
			"namespace": config.DPUClusterRef.Namespace,
		},
		"baseDomain":      config.BaseDomain,
		"ocpReleaseImage": config.OCPReleaseImage,
		"sshKeySecretRef": config.SSHKeySecretRef,
		"pullSecretRef":   config.PullSecretRef,
	}

	if config.ControlPlaneAvailabilityPolicy != "" {
		spec["controlPlaneAvailabilityPolicy"] = config.ControlPlaneAvailabilityPolicy
	}
	if config.VirtualIP != "" {
		spec["virtualIP"] = config.VirtualIP
	}
	if config.EtcdStorageClass != "" {
		spec["etcdStorageClass"] = config.EtcdStorageClass
	}
	if config.FlannelEnabled != nil {
		spec["flannelEnabled"] = *config.FlannelEnabled
	}
	if config.DPUDeploymentRef != nil {
		spec["dpuDeploymentRef"] = map[string]interface{}{
			"name":      config.DPUDeploymentRef.Name,
			"namespace": config.DPUDeploymentRef.Namespace,
		}
	}
	if config.MachineOSURL != "" {
		spec["machineOSURL"] = config.MachineOSURL
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": CRGroup + "/" + CRVersion,
			"kind":       "DPFHCPProvisioner",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}

	_, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	return err
}

// GetProvisioner gets a DPFHCPProvisioner CR and returns its status.
func (c *DPFHCPClient) GetProvisioner(ctx context.Context, name, namespace string) (*ProvisionerStatus, error) {
	obj, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return extractStatus(obj)
}

// DeleteProvisioner deletes a DPFHCPProvisioner CR.
func (c *DPFHCPClient) DeleteProvisioner(ctx context.Context, name, namespace string) error {
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// WatchProvisionerPhase watches a DPFHCPProvisioner CR until it reaches the target phase
// or fails. Returns the final phase or an error.
func (c *DPFHCPClient) WatchProvisionerPhase(ctx context.Context, name, namespace string, targetPhase string) (string, error) {
	watcher, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		return "", fmt.Errorf("watching DPFHCPProvisioner %s/%s: %w", namespace, name, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return "", fmt.Errorf("watch channel closed for DPFHCPProvisioner %s/%s", namespace, name)
			}

			if event.Type == watch.Deleted {
				return "", fmt.Errorf("DPFHCPProvisioner %s/%s was deleted", namespace, name)
			}

			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}

			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			status, err := extractStatus(obj)
			if err != nil {
				continue
			}

			if status.Phase == targetPhase {
				return status.Phase, nil
			}

			if status.Phase == "Failed" {
				return status.Phase, fmt.Errorf("DPFHCPProvisioner %s/%s reached Failed phase", namespace, name)
			}
		}
	}
}

// extractStatus reads phase and conditions from an unstructured DPFHCPProvisioner object.
func extractStatus(obj *unstructured.Unstructured) (*ProvisionerStatus, error) {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")

	result := &ProvisionerStatus{
		Phase: phase,
	}

	condSlice, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return result, nil
	}

	for _, c := range condSlice {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		cond := StatusCondition{}
		if v, ok := condMap["type"].(string); ok {
			cond.Type = v
		}
		if v, ok := condMap["status"].(string); ok {
			cond.Status = v
		}
		if v, ok := condMap["reason"].(string); ok {
			cond.Reason = v
		}
		if v, ok := condMap["message"].(string); ok {
			cond.Message = v
		}
		if v, ok := condMap["lastTransitionTime"].(string); ok {
			cond.LastTransitionTime = v
		}

		result.Conditions = append(result.Conditions, cond)
	}

	return result, nil
}
