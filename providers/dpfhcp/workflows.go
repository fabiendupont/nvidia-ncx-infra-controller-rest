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
	"time"

	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
	tsdkWorker "go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// TaskQueue returns the Temporal task queue name for the DPF HCP provider.
func (p *DPFHCPProvider) TaskQueue() string { return "dpfhcp-tasks" }

// RegisterWorkflows registers all DPF HCP Temporal workflows on the given worker.
func (p *DPFHCPProvider) RegisterWorkflows(w tsdkWorker.Worker) {
	w.RegisterWorkflow(DPFHCPProvisioningWorkflow)
	w.RegisterWorkflow(DPFHCPTeardownWorkflow)
}

// RegisterActivities registers all DPF HCP Temporal activities on the given worker.
func (p *DPFHCPProvider) RegisterActivities(w tsdkWorker.Worker) {
	activities := &DPFHCPActivities{store: p.store, client: p.k8sClient}
	w.RegisterActivity(activities)
}

// DPFHCPProvisioningWorkflow orchestrates DPU cluster provisioning.
// Steps: ValidateSite -> CreateRecord -> CreateCR -> WaitProvisioning -> WaitReady -> UpdateStatus
func DPFHCPProvisioningWorkflow(ctx workflow.Context, siteID string, config DPFHCPRequest) error {
	logger := log.With().Str("Workflow", "DPFHCPProvisioning").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}

	apiOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retrypolicy,
	}

	waitOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	apiCtx := workflow.WithActivityOptions(ctx, apiOptions)
	waitCtx := workflow.WithActivityOptions(ctx, waitOptions)

	var activities DPFHCPActivities

	// Step 1: Validate site state
	logger.Info().Msg("validating site state")
	err := workflow.ExecuteActivity(apiCtx, activities.ValidateSiteState, siteID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to validate site state")
		return err
	}

	// Step 2: Create provisioning record
	logger.Info().Msg("creating provisioning record")
	err = workflow.ExecuteActivity(apiCtx, activities.CreateProvisioningRecord, siteID, config).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create provisioning record")
		return err
	}

	// Step 3: Create DPF HCP Provisioner CR
	logger.Info().Msg("creating DPF HCP Provisioner CR")
	err = workflow.ExecuteActivity(apiCtx, activities.CreateDPFHCPProvisionerCR, siteID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create DPF HCP Provisioner CR")
		return err
	}

	// Step 4: Wait for provisioning phase
	logger.Info().Msg("waiting for provisioning phase")
	err = workflow.ExecuteActivity(waitCtx, activities.WaitForPhase, siteID, "Provisioning").Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed waiting for provisioning phase")
		return err
	}

	// Step 5: Wait for ready phase
	logger.Info().Msg("waiting for ready phase")
	err = workflow.ExecuteActivity(waitCtx, activities.WaitForPhase, siteID, "Ready").Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed waiting for ready phase")
		return err
	}

	// Step 6: Update site DPF status
	logger.Info().Msg("updating site DPF status to ready")
	err = workflow.ExecuteActivity(apiCtx, activities.UpdateSiteDPFStatus, siteID, StatusReady).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to update site DPF status")
		return err
	}

	logger.Info().Msg("completing workflow")
	return nil
}

// DPFHCPTeardownWorkflow tears down DPU cluster infrastructure.
// Steps: ValidateSite -> DeleteCR -> WaitDeletion -> DeleteRecord
func DPFHCPTeardownWorkflow(ctx workflow.Context, siteID string) error {
	logger := log.With().Str("Workflow", "DPFHCPTeardown").
		Str("SiteID", siteID).Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}

	apiOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retrypolicy,
	}

	waitOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	apiCtx := workflow.WithActivityOptions(ctx, apiOptions)
	waitCtx := workflow.WithActivityOptions(ctx, waitOptions)

	var activities DPFHCPActivities

	// Step 1: Validate site state
	logger.Info().Msg("validating site state")
	err := workflow.ExecuteActivity(apiCtx, activities.ValidateSiteState, siteID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to validate site state")
		return err
	}

	// Step 2: Delete DPF HCP Provisioner CR
	logger.Info().Msg("deleting DPF HCP Provisioner CR")
	err = workflow.ExecuteActivity(apiCtx, activities.DeleteDPFHCPProvisionerCR, siteID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to delete DPF HCP Provisioner CR")
		return err
	}

	// Step 3: Wait for CR deletion
	logger.Info().Msg("waiting for CR deletion")
	err = workflow.ExecuteActivity(waitCtx, activities.WaitForCRDeletion, siteID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed waiting for CR deletion")
		return err
	}

	// Step 4: Delete provisioning record
	logger.Info().Msg("deleting provisioning record")
	err = workflow.ExecuteActivity(apiCtx, activities.DeleteProvisioningRecord, siteID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to delete provisioning record")
		return err
	}

	logger.Info().Msg("completing workflow")
	return nil
}
