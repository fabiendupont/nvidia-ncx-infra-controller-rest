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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_MissingURL(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URL is required")
}

func TestNew_Success(t *testing.T) {
	c, err := New(Config{URL: "https://aap.example.com", Token: "test"})
	require.NoError(t, err)
	assert.Equal(t, "https://aap.example.com", c.baseURL)
	assert.Equal(t, "test", c.token)
}

func TestNew_TrailingSlash(t *testing.T) {
	c, err := New(Config{URL: "https://aap.example.com/", Token: "test"})
	require.NoError(t, err)
	assert.Equal(t, "https://aap.example.com", c.baseURL)
}

func TestPing_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/ping/", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL, Token: "test-token"})
	require.NoError(t, err)

	err = c.Ping(context.Background())
	assert.NoError(t, err)
}

func TestPing_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", user)
		assert.Equal(t, "secret", pass)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL, Username: "admin", Password: "secret"})
	require.NoError(t, err)

	err = c.Ping(context.Background())
	assert.NoError(t, err)
}

func TestPing_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL, Token: "bad-token"})
	require.NoError(t, err)

	err = c.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestLaunchJobTemplate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/job_templates/42/launch/", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req LaunchRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "vpc-123", req.ExtraVars["nico_vpc_id"])

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(LaunchResponse{ID: 1, Status: JobStatusPending, Job: 100})
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL, Token: "test"})
	require.NoError(t, err)

	resp, err := c.LaunchJobTemplate(context.Background(), 42, LaunchRequest{
		ExtraVars: map[string]interface{}{"nico_vpc_id": "vpc-123"},
	})
	require.NoError(t, err)
	assert.Equal(t, 100, resp.Job)
}

func TestGetJob_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/jobs/100/", r.URL.Path)
		json.NewEncoder(w).Encode(Job{
			ID:     100,
			Status: JobStatusSuccessful,
		})
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL, Token: "test"})
	require.NoError(t, err)

	job, err := c.GetJob(context.Background(), 100)
	require.NoError(t, err)
	assert.Equal(t, JobStatusSuccessful, job.Status)
}

func TestJobStatus_IsTerminal(t *testing.T) {
	assert.True(t, JobStatusSuccessful.IsTerminal())
	assert.True(t, JobStatusFailed.IsTerminal())
	assert.True(t, JobStatusError.IsTerminal())
	assert.True(t, JobStatusCanceled.IsTerminal())

	assert.False(t, JobStatusNew.IsTerminal())
	assert.False(t, JobStatusPending.IsTerminal())
	assert.False(t, JobStatusWaiting.IsTerminal())
	assert.False(t, JobStatusRunning.IsTerminal())
}

func TestJobStatus_IsSuccess(t *testing.T) {
	assert.True(t, JobStatusSuccessful.IsSuccess())
	assert.False(t, JobStatusFailed.IsSuccess())
	assert.False(t, JobStatusRunning.IsSuccess())
}
