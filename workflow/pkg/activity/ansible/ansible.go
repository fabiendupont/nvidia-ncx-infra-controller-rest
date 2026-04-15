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

// Package ansible provides a generic Temporal activity for triggering
// Ansible Automation Platform (AAP) job templates from any NICo workflow.
//
// This is a shared activity — not provider-specific. Any provider that
// implements WorkflowProvider can register this activity and call it from
// its workflows. The activity handles:
//
//   - Launching AAP job templates and workflow job templates
//   - Polling for job completion with configurable timeout
//   - Passing NICo context (VPC ID, subnet prefix, etc.) as extra_vars
//   - Returning structured results for downstream workflow decisions
//
// Usage from a workflow:
//
//	var ansibleMgr ansible.ManageAnsible
//	var result ansible.JobResult
//	err := workflow.ExecuteActivity(ctx, ansibleMgr.RunJob, ansible.RunJobInput{
//	    TemplateID: 42,
//	    ExtraVars:  map[string]interface{}{"nico_vpc_id": vpcID},
//	}).Get(ctx, &result)
package ansible

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/providers/ansible-fabric/client"
)

// hookFirer is the interface for firing lifecycle hooks. Matches the
// pattern used by ManageVpc, ManageInstance, etc.
type hookFirer interface {
	FireSync(ctx context.Context, feature, event string, payload interface{}) error
	FireAsync(ctx context.Context, feature, event string, payload interface{})
}

// ManageAnsible is the Temporal activity for running AAP job templates.
// It follows the same value-receiver pattern as ManageVpc, ManageInstance,
// etc. Register it with w.RegisterActivity(&ansibleManager).
type ManageAnsible struct {
	aapClient           *client.Client
	defaultTimeout      time.Duration
	defaultPollInterval time.Duration
	hooks               hookFirer
}

// NewManageAnsible creates a new ManageAnsible activity with a pre-configured
// AAP client. The client handles authentication and connectivity.
func NewManageAnsible(aapClient *client.Client, defaultTimeout, defaultPollInterval time.Duration) ManageAnsible {
	if defaultTimeout == 0 {
		defaultTimeout = 10 * time.Minute
	}
	if defaultPollInterval == 0 {
		defaultPollInterval = 5 * time.Second
	}
	return ManageAnsible{
		aapClient:           aapClient,
		defaultTimeout:      defaultTimeout,
		defaultPollInterval: defaultPollInterval,
	}
}

// SetHooks enables lifecycle hook integration. Optional — activities work
// without hooks.
func (ma *ManageAnsible) SetHooks(hooks hookFirer) {
	ma.hooks = hooks
}

// RunJobInput contains the parameters for launching an AAP job template.
type RunJobInput struct {
	// TemplateID is the AAP job template ID to launch.
	TemplateID int

	// ExtraVars are passed to the playbook as Ansible extra variables.
	// Convention: prefix NICo-originated variables with "nico_" to avoid
	// collisions with playbook variables.
	ExtraVars map[string]interface{}

	// Limit restricts the playbook run to specific hosts (Ansible --limit).
	// Empty string means no limit (all hosts in the inventory).
	Limit string

	// Timeout overrides the default job timeout for this run. Zero means
	// use the default (10 minutes).
	Timeout time.Duration

	// CheckMode runs the playbook in dry-run mode (Ansible --check).
	// Useful for pre-create validation hooks.
	CheckMode bool

	// Description is a human-readable label for logging and audit trail.
	Description string
}

// JobResult contains the outcome of an AAP job execution.
type JobResult struct {
	// JobID is the AAP job ID.
	JobID int

	// Status is the terminal job status (successful, failed, error, canceled).
	Status string

	// Elapsed is the job execution time in seconds.
	Elapsed float64

	// Success is true if the job completed successfully.
	Success bool
}

// RunJob launches an AAP job template and waits for completion. This is
// the primary activity method — call it from any NICo Temporal workflow.
//
// The activity is idempotent via Temporal's workflow ID deduplication.
// If the workflow retries, Temporal ensures the same activity execution
// is reused rather than launching a duplicate job.
func (ma ManageAnsible) RunJob(ctx context.Context, input RunJobInput) (JobResult, error) {
	logger := log.With().
		Str("activity", "RunJob").
		Int("template_id", input.TemplateID).
		Str("description", input.Description).
		Logger()

	if ma.aapClient == nil {
		return JobResult{}, fmt.Errorf("AAP client not initialized")
	}

	if input.TemplateID <= 0 {
		return JobResult{}, fmt.Errorf("invalid template ID: %d", input.TemplateID)
	}

	// Inject check mode into extra_vars if requested
	extraVars := input.ExtraVars
	if input.CheckMode {
		if extraVars == nil {
			extraVars = make(map[string]interface{})
		}
		extraVars["ansible_check_mode"] = true
	}

	// Fire pre-run hook
	if ma.hooks != nil {
		hookPayload := map[string]interface{}{
			"template_id": input.TemplateID,
			"extra_vars":  extraVars,
			"description": input.Description,
			"check_mode":  input.CheckMode,
		}
		if err := ma.hooks.FireSync(ctx, "ansible", "pre-run-job", hookPayload); err != nil {
			return JobResult{}, fmt.Errorf("pre-run hook failed: %w", err)
		}
	}

	logger.Info().
		Interface("extra_vars", extraVars).
		Bool("check_mode", input.CheckMode).
		Msg("launching AAP job template")

	// Launch the job
	resp, err := ma.aapClient.LaunchJobTemplate(ctx, input.TemplateID, client.LaunchRequest{
		ExtraVars: extraVars,
		Limit:     input.Limit,
	})
	if err != nil {
		return JobResult{}, fmt.Errorf("launching template %d: %w", input.TemplateID, err)
	}

	logger.Info().
		Int("job_id", resp.Job).
		Msg("AAP job launched, waiting for completion")

	// Wait for completion with timeout
	timeout := input.Timeout
	if timeout == 0 {
		timeout = ma.defaultTimeout
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	job, err := ma.aapClient.WaitForJob(waitCtx, resp.Job, ma.defaultPollInterval)
	if err != nil {
		return JobResult{JobID: resp.Job}, fmt.Errorf("waiting for job %d: %w", resp.Job, err)
	}

	result := JobResult{
		JobID:   job.ID,
		Status:  string(job.Status),
		Elapsed: job.Elapsed,
		Success: job.Status.IsSuccess(),
	}

	logger.Info().
		Int("job_id", job.ID).
		Str("status", result.Status).
		Float64("elapsed_seconds", job.Elapsed).
		Bool("success", result.Success).
		Msg("AAP job completed")

	// Fire post-run hook (async, non-blocking)
	if ma.hooks != nil {
		ma.hooks.FireAsync(ctx, "ansible", "post-run-job", map[string]interface{}{
			"job_id":      job.ID,
			"template_id": input.TemplateID,
			"status":      result.Status,
			"success":     result.Success,
			"elapsed":     result.Elapsed,
			"description": input.Description,
		})
	}

	if !result.Success {
		return result, fmt.Errorf("AAP job %d finished with status %s", job.ID, result.Status)
	}

	return result, nil
}

// RunWorkflowJobInput contains the parameters for launching an AAP
// workflow job template (multi-step: validate → apply → verify).
type RunWorkflowJobInput struct {
	// TemplateID is the AAP workflow job template ID.
	TemplateID int

	// ExtraVars are passed to all nodes in the workflow.
	ExtraVars map[string]interface{}

	// Timeout overrides the default timeout. Workflow jobs typically
	// take longer, so consider setting this higher (e.g., 30 minutes).
	Timeout time.Duration

	// Description is a human-readable label for logging.
	Description string
}

// RunWorkflowJob launches an AAP workflow job template and waits for
// completion. Use this for multi-step operations that chain multiple
// playbooks with approval gates and automatic rollback.
func (ma ManageAnsible) RunWorkflowJob(ctx context.Context, input RunWorkflowJobInput) (JobResult, error) {
	logger := log.With().
		Str("activity", "RunWorkflowJob").
		Int("template_id", input.TemplateID).
		Str("description", input.Description).
		Logger()

	if ma.aapClient == nil {
		return JobResult{}, fmt.Errorf("AAP client not initialized")
	}

	if input.TemplateID <= 0 {
		return JobResult{}, fmt.Errorf("invalid workflow template ID: %d", input.TemplateID)
	}

	logger.Info().
		Interface("extra_vars", input.ExtraVars).
		Msg("launching AAP workflow job template")

	resp, err := ma.aapClient.LaunchWorkflowJobTemplate(ctx, input.TemplateID, client.LaunchRequest{
		ExtraVars: input.ExtraVars,
	})
	if err != nil {
		return JobResult{}, fmt.Errorf("launching workflow template %d: %w", input.TemplateID, err)
	}

	logger.Info().
		Int("job_id", resp.Job).
		Msg("AAP workflow job launched, waiting for completion")

	timeout := input.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	job, err := ma.aapClient.WaitForWorkflowJob(waitCtx, resp.Job, ma.defaultPollInterval)
	if err != nil {
		return JobResult{JobID: resp.Job}, fmt.Errorf("waiting for workflow job %d: %w", resp.Job, err)
	}

	result := JobResult{
		JobID:   job.ID,
		Status:  string(job.Status),
		Elapsed: job.Elapsed,
		Success: job.Status.IsSuccess(),
	}

	logger.Info().
		Int("job_id", job.ID).
		Str("status", result.Status).
		Float64("elapsed_seconds", job.Elapsed).
		Bool("success", result.Success).
		Msg("AAP workflow job completed")

	if !result.Success {
		return result, fmt.Errorf("AAP workflow job %d finished with status %s", job.ID, result.Status)
	}

	return result, nil
}
