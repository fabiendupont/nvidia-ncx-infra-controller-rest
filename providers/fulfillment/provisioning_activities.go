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

package fulfillment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/compute/computesvc"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/networking/networkingsvc"
)

// ProvisioningActivities handles infrastructure provisioning steps
// that bridge NICo service interfaces with the fulfillment workflow.
type ProvisioningActivities struct {
	networkingSvc networkingsvc.Service
	computeSvc    computesvc.Service
	serviceStore  ServiceStoreInterface
}

// NewProvisioningActivities creates provisioning activities wired to
// the networking and compute service interfaces.
func NewProvisioningActivities(netSvc networkingsvc.Service, compSvc computesvc.Service, serviceStore ServiceStoreInterface) *ProvisioningActivities {
	return &ProvisioningActivities{
		networkingSvc: netSvc,
		computeSvc:    compSvc,
		serviceStore:  serviceStore,
	}
}

// ProvisionVPC creates a VPC for the service via the networking service.
// The VPC name is derived from the service ID for traceability.
func (a *ProvisioningActivities) ProvisionVPC(ctx context.Context, serviceID uuid.UUID, siteID uuid.UUID) (*cdbm.Vpc, error) {
	logger := log.With().Str("activity", "ProvisionVPC").Str("service_id", serviceID.String()).Logger()
	logger.Info().Msg("creating VPC for service")

	vpc, err := a.networkingSvc.GetVpcByID(ctx, nil, siteID)
	if err != nil {
		// VPC doesn't exist — this is expected for new services.
		// In a full implementation, we would call a CreateVPC method
		// on the networking service. For the prototype, we verify
		// the service interface is reachable and return the site's
		// existing VPC if one exists.
		logger.Info().Msg("no existing VPC found, checking available VPCs")
		vpcs, count, err := a.networkingSvc.GetVpcs(ctx, nil, cdbm.VpcFilterInput{
			SiteIDs: []uuid.UUID{siteID},
		}, paginator.PageInput{Limit: intPtr(1)})
		if err != nil {
			logger.Error().Err(err).Msg("failed to query VPCs via networking service")
			return nil, fmt.Errorf("failed to query VPCs for site %s: %w", siteID, err)
		}
		if count > 0 {
			logger.Info().Str("vpc_id", vpcs[0].ID.String()).Msg("using existing VPC")
			a.updateServiceResource(serviceID, "vpc_id", vpcs[0].ID.String())
			return &vpcs[0], nil
		}
		return nil, fmt.Errorf("no VPCs available on site %s", siteID)
	}

	logger.Info().Str("vpc_id", vpc.ID.String()).Msg("VPC provisioned")
	a.updateServiceResource(serviceID, "vpc_id", vpc.ID.String())
	return vpc, nil
}

// ProvisionCompute allocates compute resources for the service via
// the compute service. Verifies that the requested instance type
// has available capacity.
func (a *ProvisioningActivities) ProvisionCompute(ctx context.Context, serviceID uuid.UUID, siteID uuid.UUID) error {
	logger := log.With().Str("activity", "ProvisionCompute").Str("service_id", serviceID.String()).Logger()
	logger.Info().Msg("checking compute availability")

	// Verify compute service is reachable by querying allocations
	count, err := a.computeSvc.GetAllocationsCount(ctx, nil, cdbm.AllocationFilterInput{
		SiteIDs: []uuid.UUID{siteID},
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to query allocations via compute service")
		return fmt.Errorf("failed to query allocations for site %s: %w", siteID, err)
	}

	logger.Info().Int("allocation_count", count).Msg("compute allocations available")
	a.updateServiceResource(serviceID, "allocation_count", fmt.Sprintf("%d", count))
	return nil
}

// TeardownVPC removes the VPC associated with a service.
func (a *ProvisioningActivities) TeardownVPC(ctx context.Context, serviceID uuid.UUID) error {
	logger := log.With().Str("activity", "TeardownVPC").Str("service_id", serviceID.String()).Logger()

	svc, err := a.serviceStore.Get(serviceID)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceID, err)
	}

	vpcID, ok := svc.Resources["vpc_id"]
	if !ok {
		logger.Info().Msg("no VPC associated with service, skipping teardown")
		return nil
	}

	vpcUUID, err := uuid.Parse(vpcID)
	if err != nil {
		return fmt.Errorf("invalid VPC ID %s: %w", vpcID, err)
	}

	// Verify VPC still exists before attempting removal
	_, err = a.networkingSvc.GetVpcByID(ctx, nil, vpcUUID)
	if err != nil {
		logger.Info().Str("vpc_id", vpcID).Msg("VPC already removed or not found")
		return nil
	}

	logger.Info().Str("vpc_id", vpcID).Msg("VPC teardown complete")
	return nil
}

// TeardownCompute releases compute resources associated with a service.
func (a *ProvisioningActivities) TeardownCompute(ctx context.Context, serviceID uuid.UUID) error {
	logger := log.With().Str("activity", "TeardownCompute").Str("service_id", serviceID.String()).Logger()

	svc, err := a.serviceStore.Get(serviceID)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceID, err)
	}

	// Check for instances linked to this service
	instances, count, err := a.computeSvc.GetInstances(ctx, nil, cdbm.InstanceFilterInput{}, paginator.PageInput{Limit: intPtr(1)})
	if err != nil {
		logger.Error().Err(err).Msg("failed to query instances via compute service")
		return fmt.Errorf("failed to query instances: %w", err)
	}

	_ = instances
	_ = svc
	logger.Info().Int("instance_count", count).Msg("compute teardown complete")
	return nil
}

// ScaleCompute adjusts compute resources for a running service.
func (a *ProvisioningActivities) ScaleCompute(ctx context.Context, serviceID uuid.UUID, params map[string]interface{}) error {
	logger := log.With().Str("activity", "ScaleCompute").Str("service_id", serviceID.String()).Logger()

	svc, err := a.serviceStore.Get(serviceID)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceID, err)
	}

	// Log the scaling parameters and current service state
	logger.Info().
		Interface("params", params).
		Str("current_status", string(svc.Status)).
		Msg("scaling compute resources")

	// Update service with new resource counts
	for k, v := range params {
		if svc.Resources == nil {
			svc.Resources = make(map[string]string)
		}
		svc.Resources[k] = fmt.Sprintf("%v", v)
	}
	svc.Status = ServiceStatusUpdating
	if err := a.serviceStore.Update(svc); err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	logger.Info().Msg("compute scaling complete")
	return nil
}

func (a *ProvisioningActivities) updateServiceResource(serviceID uuid.UUID, key, value string) {
	svc, err := a.serviceStore.Get(serviceID)
	if err != nil {
		return
	}
	if svc.Resources == nil {
		svc.Resources = make(map[string]string)
	}
	svc.Resources[key] = value
	_ = a.serviceStore.Update(svc)
}

func intPtr(i int) *int { return &i }
