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

package ansible

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/providers/ansible-fabric/client"
)

// mockHookFirer records hook calls for testing.
type mockHookFirer struct {
	syncCalls  []hookCall
	asyncCalls []hookCall
	syncErr    error // If set, FireSync returns this error
}

type hookCall struct {
	feature string
	event   string
	payload interface{}
}

func (m *mockHookFirer) FireSync(_ context.Context, feature, event string, payload interface{}) error {
	m.syncCalls = append(m.syncCalls, hookCall{feature, event, payload})
	return m.syncErr
}

func (m *mockHookFirer) FireAsync(_ context.Context, feature, event string, payload interface{}) {
	m.asyncCalls = append(m.asyncCalls, hookCall{feature, event, payload})
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*client.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := client.New(client.Config{URL: srv.URL, Token: "test"})
	require.NoError(t, err)
	return c, srv
}

func successHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/launch/"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{ID: 1, Status: "pending", Job: 100})

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/jobs/"):
			json.NewEncoder(w).Encode(client.Job{ID: 100, Status: client.JobStatusSuccessful, Elapsed: 3.2})

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/workflow_jobs/"):
			json.NewEncoder(w).Encode(client.WorkflowJob{ID: 100, Status: client.JobStatusSuccessful, Elapsed: 12.5})

		default:
			http.NotFound(w, r)
		}
	})
}

func failureHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/launch/"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{ID: 1, Status: "pending", Job: 200})

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/jobs/"):
			json.NewEncoder(w).Encode(client.Job{ID: 200, Status: client.JobStatusFailed, Failed: true})

		default:
			http.NotFound(w, r)
		}
	})
}

func TestRunJob_Success(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	result, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID:  42,
		ExtraVars:   map[string]interface{}{"nico_vpc_id": "vpc-123"},
		Description: "test VPC sync",
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 100, result.JobID)
	assert.Equal(t, "successful", result.Status)
	assert.Equal(t, 3.2, result.Elapsed)
}

func TestRunJob_Failure(t *testing.T) {
	c, srv := newTestClient(t, failureHandler())
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	result, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID:  42,
		Description: "should fail",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
	assert.False(t, result.Success)
	assert.Equal(t, 200, result.JobID)
}

func TestRunJob_NilClient(t *testing.T) {
	ma := NewManageAnsible(nil, 0, 0)

	_, err := ma.RunJob(context.Background(), RunJobInput{TemplateID: 42})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRunJob_InvalidTemplateID(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	_, err := ma.RunJob(context.Background(), RunJobInput{TemplateID: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template ID")
}

func TestRunJob_CheckMode(t *testing.T) {
	var capturedVars map[string]interface{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/launch/"):
			var req client.LaunchRequest
			json.NewDecoder(r.Body).Decode(&req)
			capturedVars = req.ExtraVars

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{ID: 1, Status: "pending", Job: 100})

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/jobs/"):
			json.NewEncoder(w).Encode(client.Job{ID: 100, Status: client.JobStatusSuccessful})

		default:
			http.NotFound(w, r)
		}
	})

	c, srv := newTestClient(t, handler)
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	_, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID: 42,
		ExtraVars:  map[string]interface{}{"nico_vpc_id": "vpc-123"},
		CheckMode:  true,
	})

	require.NoError(t, err)
	assert.Equal(t, true, capturedVars["ansible_check_mode"])
	assert.Equal(t, "vpc-123", capturedVars["nico_vpc_id"])
}

func TestRunJob_WithHooks(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	hooks := &mockHookFirer{}
	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)
	ma.SetHooks(hooks)

	_, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID:  42,
		Description: "hooked job",
	})
	require.NoError(t, err)

	// Pre-run sync hook should have been called
	require.Len(t, hooks.syncCalls, 1)
	assert.Equal(t, "ansible", hooks.syncCalls[0].feature)
	assert.Equal(t, "pre-run-job", hooks.syncCalls[0].event)

	// Post-run async hook should have been called
	require.Len(t, hooks.asyncCalls, 1)
	assert.Equal(t, "ansible", hooks.asyncCalls[0].feature)
	assert.Equal(t, "post-run-job", hooks.asyncCalls[0].event)
}

func TestRunJob_PreHookAborts(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	hooks := &mockHookFirer{syncErr: assert.AnError}
	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)
	ma.SetHooks(hooks)

	_, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID: 42,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre-run hook failed")
}

func TestRunJob_Timeout(t *testing.T) {
	// Handler that never returns a terminal status
	var pollCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/launch/"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{ID: 1, Status: "pending", Job: 100})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/jobs/"):
			pollCount.Add(1)
			json.NewEncoder(w).Encode(client.Job{ID: 100, Status: client.JobStatusRunning})
		default:
			http.NotFound(w, r)
		}
	})

	c, srv := newTestClient(t, handler)
	defer srv.Close()

	ma := NewManageAnsible(c, 500*time.Millisecond, 100*time.Millisecond)

	_, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID: 42,
		Timeout:    500 * time.Millisecond,
	})
	require.Error(t, err)
	assert.True(t, pollCount.Load() > 0, "should have polled at least once")
}

func TestRunWorkflowJob_Success(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	result, err := ma.RunWorkflowJob(context.Background(), RunWorkflowJobInput{
		TemplateID:  99,
		ExtraVars:   map[string]interface{}{"nico_vpc_id": "vpc-456"},
		Description: "test workflow",
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "successful", result.Status)
}

func TestRunWorkflowJob_InvalidTemplate(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	_, err := ma.RunWorkflowJob(context.Background(), RunWorkflowJobInput{TemplateID: -1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workflow template ID")
}

func TestNewManageAnsible_Defaults(t *testing.T) {
	c, srv := newTestClient(t, successHandler())
	defer srv.Close()

	ma := NewManageAnsible(c, 0, 0)
	assert.Equal(t, 10*time.Minute, ma.defaultTimeout)
	assert.Equal(t, 5*time.Second, ma.defaultPollInterval)
}

func TestRunJob_WithLimit(t *testing.T) {
	var capturedLimit string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/launch/"):
			var req client.LaunchRequest
			json.NewDecoder(r.Body).Decode(&req)
			capturedLimit = req.Limit

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{ID: 1, Status: "pending", Job: 100})

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/jobs/"):
			json.NewEncoder(w).Encode(client.Job{ID: 100, Status: client.JobStatusSuccessful})

		default:
			http.NotFound(w, r)
		}
	})

	c, srv := newTestClient(t, handler)
	defer srv.Close()

	ma := NewManageAnsible(c, 5*time.Second, 100*time.Millisecond)

	_, err := ma.RunJob(context.Background(), RunJobInput{
		TemplateID: 42,
		Limit:      "rack01-leaf*",
	})

	require.NoError(t, err)
	assert.Equal(t, "rack01-leaf*", capturedLimit)
}
