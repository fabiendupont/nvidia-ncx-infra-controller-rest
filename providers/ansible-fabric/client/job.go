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

// JobStatus represents the status of an AAP job.
type JobStatus string

const (
	JobStatusNew        JobStatus = "new"
	JobStatusPending    JobStatus = "pending"
	JobStatusWaiting    JobStatus = "waiting"
	JobStatusRunning    JobStatus = "running"
	JobStatusSuccessful JobStatus = "successful"
	JobStatusFailed     JobStatus = "failed"
	JobStatusError      JobStatus = "error"
	JobStatusCanceled   JobStatus = "canceled"
)

// IsTerminal returns true if the job has reached a final state.
func (s JobStatus) IsTerminal() bool {
	switch s {
	case JobStatusSuccessful, JobStatusFailed, JobStatusError, JobStatusCanceled:
		return true
	}
	return false
}

// IsSuccess returns true if the job completed successfully.
func (s JobStatus) IsSuccess() bool {
	return s == JobStatusSuccessful
}

// LaunchRequest contains the parameters for launching a job template.
type LaunchRequest struct {
	// ExtraVars are passed to the playbook as extra variables.
	ExtraVars map[string]interface{} `json:"extra_vars,omitempty"`

	// Limit restricts the playbook run to specific hosts (Ansible --limit).
	Limit string `json:"limit,omitempty"`

	// Inventory overrides the default inventory for this job run.
	Inventory int `json:"inventory,omitempty"`
}

// LaunchResponse is the response from launching a job template.
type LaunchResponse struct {
	ID     int       `json:"id"`
	Status JobStatus `json:"status"`
	Job    int       `json:"job"`
}

// Job represents the status of a running or completed AAP job.
type Job struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Status       JobStatus `json:"status"`
	Failed       bool      `json:"failed"`
	Started      string    `json:"started"`
	Finished     string    `json:"finished"`
	Elapsed      float64   `json:"elapsed"`
	ResultStdout string    `json:"result_stdout"`
}

// LaunchJobTemplate starts an AAP job template by ID, passing extra variables.
// Returns the launched job's metadata.
func (c *Client) LaunchJobTemplate(ctx context.Context, templateID int, req LaunchRequest) (*LaunchResponse, error) {
	path := fmt.Sprintf("/api/v2/job_templates/%d/launch/", templateID)
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, fmt.Errorf("launching job template %d: %w", templateID, err)
	}

	if statusCode != http.StatusCreated && statusCode != http.StatusOK {
		return nil, fmt.Errorf("launching job template %d: unexpected status %d: %s", templateID, statusCode, string(body))
	}

	var result LaunchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding launch response: %w", err)
	}

	return &result, nil
}

// GetJob retrieves the current status of a job by ID.
func (c *Client) GetJob(ctx context.Context, jobID int) (*Job, error) {
	path := fmt.Sprintf("/api/v2/jobs/%d/", jobID)
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting job %d: %w", jobID, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("getting job %d: unexpected status %d: %s", jobID, statusCode, string(body))
	}

	var result Job
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding job response: %w", err)
	}

	return &result, nil
}

// WaitForJob polls the job status until it reaches a terminal state or the
// context is cancelled. Returns the final job state.
func (c *Client) WaitForJob(ctx context.Context, jobID int, pollInterval time.Duration) (*Job, error) {
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
			job, err := c.GetJob(ctx, jobID)
			if err != nil {
				return nil, err
			}
			if job.Status.IsTerminal() {
				return job, nil
			}
		}
	}
}

// CancelJob attempts to cancel a running job.
func (c *Client) CancelJob(ctx context.Context, jobID int) error {
	path := fmt.Sprintf("/api/v2/jobs/%d/cancel/", jobID)
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("cancelling job %d: %w", jobID, err)
	}

	if statusCode != http.StatusAccepted && statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return fmt.Errorf("cancelling job %d: unexpected status %d: %s", jobID, statusCode, string(body))
	}

	return nil
}
