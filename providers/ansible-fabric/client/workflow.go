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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WorkflowJob represents the status of a running or completed AAP workflow job.
type WorkflowJob struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Status   JobStatus `json:"status"`
	Failed   bool      `json:"failed"`
	Started  string    `json:"started"`
	Finished string    `json:"finished"`
	Elapsed  float64   `json:"elapsed"`
}

// LaunchWorkflowJobTemplate starts an AAP workflow job template by ID.
// Workflow job templates chain multiple job templates with approval gates,
// branching, and convergence. Used for multi-step operations like
// "validate → apply → verify" with automatic rollback on failure.
func (c *Client) LaunchWorkflowJobTemplate(ctx context.Context, templateID int, req LaunchRequest) (*LaunchResponse, error) {
	path := fmt.Sprintf("/api/v2/workflow_job_templates/%d/launch/", templateID)
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, fmt.Errorf("launching workflow job template %d: %w", templateID, err)
	}

	if statusCode != http.StatusCreated && statusCode != http.StatusOK {
		return nil, fmt.Errorf("launching workflow job template %d: unexpected status %d: %s", templateID, statusCode, string(body))
	}

	var result LaunchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding workflow launch response: %w", err)
	}

	return &result, nil
}

// GetWorkflowJob retrieves the current status of a workflow job by ID.
func (c *Client) GetWorkflowJob(ctx context.Context, jobID int) (*WorkflowJob, error) {
	path := fmt.Sprintf("/api/v2/workflow_jobs/%d/", jobID)
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting workflow job %d: %w", jobID, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("getting workflow job %d: unexpected status %d: %s", jobID, statusCode, string(body))
	}

	var result WorkflowJob
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding workflow job response: %w", err)
	}

	return &result, nil
}

// WaitForWorkflowJob polls the workflow job status until it reaches a terminal
// state or the context is cancelled. Returns the final workflow job state.
func (c *Client) WaitForWorkflowJob(ctx context.Context, jobID int, pollInterval time.Duration) (*WorkflowJob, error) {
	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			job, err := c.GetWorkflowJob(ctx, jobID)
			if err != nil {
				return nil, err
			}
			if job.Status.IsTerminal() {
				return job, nil
			}
		}
	}
}
