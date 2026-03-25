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

	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"go.temporal.io/sdk/temporal"
)

const (
	// defaultCRNamespace is the namespace used for DPFHCPProvisioner CRs.
	defaultCRNamespace = "dpf-operator-system"

	// crNamePrefix is used to derive the CR name from a site ID.
	crNamePrefix = "dpfhcp-"
)

// deriveCRName returns the CR name for a given site ID.
func deriveCRName(siteID string) string {
	return crNamePrefix + siteID
}

// requireK8sClient returns a non-retryable error if the K8s client is not configured.
func (a *DPFHCPActivities) requireK8sClient() error {
	if a.client == nil {
		return temporal.NewNonRetryableApplicationError(
			"K8s client is not configured for DPF HCP provider",
			"K8S_CLIENT_NOT_CONFIGURED",
			nil,
		)
	}
	return nil
}

// ValidateSiteState validates that the site exists and is in a valid state
// for DPF HCP operations.
func (a *DPFHCPActivities) ValidateSiteState(ctx context.Context, siteID string) error {
	logger := log.With().Str("Activity", "ValidateSiteState").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("validating site state")

	if siteID == "" {
		return temporal.NewNonRetryableApplicationError(
			"site ID must not be empty",
			"INVALID_SITE_ID",
			nil,
		)
	}

	record, err := a.store.GetBySiteID(siteID)
	if err != nil {
		logger.Debug().Msg("no existing provisioning record found for site")
	}
	if record != nil {
		logger.Info().Str("Status", string(record.Status)).Msg("existing provisioning record found")
	}

	logger.Info().Msg("site state validated")
	return nil
}

// CreateProvisioningRecord creates a new provisioning record for the given site
// and DPF HCP configuration. Idempotent: if a record already exists, it updates
// the configuration.
func (a *DPFHCPActivities) CreateProvisioningRecord(ctx context.Context, siteID string, config DPFHCPRequest) error {
	logger := log.With().Str("Activity", "CreateProvisioningRecord").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("creating provisioning record")

	existing, _ := a.store.GetBySiteID(siteID)
	if existing != nil {
		logger.Info().Msg("provisioning record already exists, updating configuration")
		existing.Config = config
		existing.Updated = time.Now()
		if err := a.store.Update(existing); err != nil {
			logger.Warn().Err(err).Msg("failed to update existing provisioning record")
			return fmt.Errorf("failed to update provisioning record for site %s: %w", siteID, err)
		}
		logger.Info().Msg("provisioning record updated")
		return nil
	}

	now := time.Now()
	record := &ProvisioningRecord{
		SiteID:      siteID,
		Config:      config,
		Status:      StatusPending,
		CRName:      deriveCRName(siteID),
		CRNamespace: defaultCRNamespace,
		Created:     now,
		Updated:     now,
	}

	if err := a.store.Create(record); err != nil {
		logger.Warn().Err(err).Msg("failed to create provisioning record")
		return fmt.Errorf("failed to create provisioning record for site %s: %w", siteID, err)
	}

	logger.Info().Msg("provisioning record created")
	return nil
}

// CreateDPFHCPProvisionerCR creates the DPF HCP Provisioner custom resource
// in the target Kubernetes cluster. Idempotent: if the CR already exists, it
// is a no-op.
func (a *DPFHCPActivities) CreateDPFHCPProvisionerCR(ctx context.Context, siteID string) error {
	logger := log.With().Str("Activity", "CreateDPFHCPProvisionerCR").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("creating DPF HCP Provisioner CR")

	if err := a.requireK8sClient(); err != nil {
		return err
	}

	record, err := a.store.GetBySiteID(siteID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve provisioning record")
		return fmt.Errorf("failed to retrieve provisioning record for site %s: %w", siteID, err)
	}

	// Check if CR already exists (idempotent)
	_, getErr := a.client.GetProvisioner(ctx, record.CRName, record.CRNamespace)
	if getErr == nil {
		logger.Info().Msg("DPF HCP Provisioner CR already exists")
		return nil
	}

	if err := a.client.CreateProvisioner(ctx, record.CRName, record.CRNamespace, record.Config); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			logger.Info().Msg("DPF HCP Provisioner CR already exists")
			return nil
		}
		logger.Warn().Err(err).Msg("failed to create DPF HCP Provisioner CR")
		return fmt.Errorf("failed to create DPF HCP Provisioner CR for site %s: %w", siteID, err)
	}

	// Update record status to provisioning
	record.Status = StatusProvisioning
	record.Updated = time.Now()
	if updateErr := a.store.Update(record); updateErr != nil {
		logger.Warn().Err(updateErr).Msg("failed to update provisioning record status")
	}

	logger.Info().Msg("DPF HCP Provisioner CR created")
	return nil
}

// WaitForPhase watches the DPF HCP Provisioner CR until it reaches the target
// phase. Returns an error if the phase is not reached within the activity
// timeout.
func (a *DPFHCPActivities) WaitForPhase(ctx context.Context, siteID string, targetPhase string) error {
	logger := log.With().Str("Activity", "WaitForPhase").
		Str("SiteID", siteID).
		Str("TargetPhase", targetPhase).Logger()

	logger.Info().Msg("waiting for phase")

	if err := a.requireK8sClient(); err != nil {
		return err
	}

	record, err := a.store.GetBySiteID(siteID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve provisioning record")
		return fmt.Errorf("failed to retrieve provisioning record for site %s: %w", siteID, err)
	}

	phase, err := a.client.WatchProvisionerPhase(ctx, record.CRName, record.CRNamespace, targetPhase)
	if err != nil {
		logger.Warn().Err(err).Msg("failed waiting for phase")
		return fmt.Errorf("failed waiting for phase %s on site %s: %w", targetPhase, siteID, err)
	}

	// Update record with observed phase
	record.Phase = phase
	record.Updated = time.Now()
	if updateErr := a.store.Update(record); updateErr != nil {
		logger.Warn().Err(updateErr).Msg("failed to update provisioning record phase")
	}

	logger.Info().Msg("target phase reached")
	return nil
}

// UpdateSiteDPFStatus updates the status field on the site's provisioning
// record.
func (a *DPFHCPActivities) UpdateSiteDPFStatus(ctx context.Context, siteID string, status ProvisioningStatus) error {
	logger := log.With().Str("Activity", "UpdateSiteDPFStatus").
		Str("SiteID", siteID).
		Str("Status", string(status)).Logger()

	logger.Info().Msg("updating site DPF status")

	record, err := a.store.GetBySiteID(siteID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve provisioning record")
		return fmt.Errorf("failed to retrieve provisioning record for site %s: %w", siteID, err)
	}

	record.Status = status
	record.Updated = time.Now()
	if err := a.store.Update(record); err != nil {
		logger.Warn().Err(err).Msg("failed to update provisioning record status")
		return fmt.Errorf("failed to update provisioning record status for site %s: %w", siteID, err)
	}

	logger.Info().Msg("site DPF status updated")
	return nil
}

// DeleteDPFHCPProvisionerCR deletes the DPF HCP Provisioner custom resource
// from the target Kubernetes cluster. Idempotent: if the CR does not exist,
// it is a no-op.
func (a *DPFHCPActivities) DeleteDPFHCPProvisionerCR(ctx context.Context, siteID string) error {
	logger := log.With().Str("Activity", "DeleteDPFHCPProvisionerCR").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("deleting DPF HCP Provisioner CR")

	if err := a.requireK8sClient(); err != nil {
		return err
	}

	record, err := a.store.GetBySiteID(siteID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve provisioning record")
		return fmt.Errorf("failed to retrieve provisioning record for site %s: %w", siteID, err)
	}

	if err := a.client.DeleteProvisioner(ctx, record.CRName, record.CRNamespace); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info().Msg("DPF HCP Provisioner CR already deleted")
			return nil
		}
		logger.Warn().Err(err).Msg("failed to delete DPF HCP Provisioner CR")
		return fmt.Errorf("failed to delete DPF HCP Provisioner CR for site %s: %w", siteID, err)
	}

	// Update record status to deleting
	record.Status = StatusDeleting
	record.Updated = time.Now()
	if updateErr := a.store.Update(record); updateErr != nil {
		logger.Warn().Err(updateErr).Msg("failed to update provisioning record status")
	}

	logger.Info().Msg("DPF HCP Provisioner CR deleted")
	return nil
}

// WaitForCRDeletion polls the Kubernetes API until the DPF HCP Provisioner CR
// for the given site is fully deleted. Returns an error if the CR is not
// deleted within the activity timeout.
func (a *DPFHCPActivities) WaitForCRDeletion(ctx context.Context, siteID string) error {
	logger := log.With().Str("Activity", "WaitForCRDeletion").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("waiting for CR deletion")

	if err := a.requireK8sClient(); err != nil {
		return err
	}

	record, err := a.store.GetBySiteID(siteID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve provisioning record")
		return fmt.Errorf("failed to retrieve provisioning record for site %s: %w", siteID, err)
	}

	// Poll until the CR no longer exists
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, getErr := a.client.GetProvisioner(ctx, record.CRName, record.CRNamespace)
		if getErr != nil {
			if k8serrors.IsNotFound(getErr) {
				logger.Info().Msg("CR deletion confirmed")
				return nil
			}
			logger.Debug().Err(getErr).Msg("error checking CR existence, will retry")
		}

		// Brief pause before next poll
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// DeleteProvisioningRecord removes the provisioning record for the given site.
// Idempotent: if the record does not exist, it is a no-op.
func (a *DPFHCPActivities) DeleteProvisioningRecord(ctx context.Context, siteID string) error {
	logger := log.With().Str("Activity", "DeleteProvisioningRecord").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("deleting provisioning record")

	err := a.store.Delete(siteID)
	if err != nil {
		// Check if already deleted (idempotent)
		if _, getErr := a.store.GetBySiteID(siteID); getErr != nil {
			logger.Info().Msg("provisioning record already deleted")
			return nil
		}
		logger.Warn().Err(err).Msg("failed to delete provisioning record")
		return fmt.Errorf("failed to delete provisioning record for site %s: %w", siteID, err)
	}

	logger.Info().Msg("provisioning record deleted")
	return nil
}
