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

package ansiblefabric

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/providers/ansible-fabric/client"
)

// newTestProvider creates an AnsibleFabricProvider backed by an httptest
// server. The handler function receives all HTTP requests sent to the
// mock AAP Controller.
func newTestProvider(t *testing.T, handler http.HandlerFunc, templates TemplateConfig) (*AnsibleFabricProvider, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(handler)

	c, err := client.New(client.Config{
		URL:   srv.URL,
		Token: "test-token",
	})
	require.NoError(t, err)

	p := &AnsibleFabricProvider{
		config: ProviderConfig{
			AAPURL:          srv.URL,
			AAPToken:        "test-token",
			Templates:       templates,
			JobPollInterval: 100 * time.Millisecond,
			JobTimeout:      5 * time.Second,
		},
		client: c,
	}

	return p, srv
}

// aapMockHandler returns an http.HandlerFunc that simulates the AAP
// Controller API. It accepts job template launches and returns
// immediately-successful jobs.
func aapMockHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Job template launch
		case r.Method == http.MethodPost && contains(r.URL.Path, "/launch/"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{
				ID:     1,
				Status: client.JobStatusPending,
				Job:    100,
			})

		// Job status poll — return successful immediately
		case r.Method == http.MethodGet && contains(r.URL.Path, "/api/v2/jobs/"):
			json.NewEncoder(w).Encode(client.Job{
				ID:      100,
				Name:    "test-job",
				Status:  client.JobStatusSuccessful,
				Elapsed: 2.5,
			})

		// Ping
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ping/":
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.NotFound(w, r)
		}
	})
}

// aapFailingMockHandler returns a handler where jobs always fail.
func aapFailingMockHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && contains(r.URL.Path, "/launch/"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(client.LaunchResponse{
				ID:     1,
				Status: client.JobStatusPending,
				Job:    200,
			})

		case r.Method == http.MethodGet && contains(r.URL.Path, "/api/v2/jobs/"):
			json.NewEncoder(w).Encode(client.Job{
				ID:     200,
				Name:   "failing-job",
				Status: client.JobStatusFailed,
				Failed: true,
			})

		default:
			http.NotFound(w, r)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSyncVPCToFabric_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{CreateVPC: 10})
	defer srv.Close()

	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	assert.NoError(t, err)
}

func TestSyncVPCToFabric_NoTemplate(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{})
	defer srv.Close()

	// Should be a no-op when template is not configured
	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	assert.NoError(t, err)
}

func TestSyncVPCToFabric_JobFails(t *testing.T) {
	p, srv := newTestProvider(t, aapFailingMockHandler(t), TemplateConfig{CreateVPC: 10})
	defer srv.Close()

	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestRemoveVPCFromFabric_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{DeleteVPC: 11})
	defer srv.Close()

	err := p.RemoveVPCFromFabric(context.Background(), "vpc-123")
	assert.NoError(t, err)
}

func TestSyncSubnetToFabric_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{CreateSubnet: 20})
	defer srv.Close()

	err := p.SyncSubnetToFabric(context.Background(), "sub-456", "vpc-123", "10.0.1.0/24", "my-subnet")
	assert.NoError(t, err)
}

func TestRemoveSubnetFromFabric_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{DeleteSubnet: 21})
	defer srv.Close()

	err := p.RemoveSubnetFromFabric(context.Background(), "sub-456", "vpc-123")
	assert.NoError(t, err)
}

func TestConfigureInstancePorts_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{ConfigureInstance: 30})
	defer srv.Close()

	err := p.ConfigureInstancePorts(context.Background(), "inst-789", "mach-001", "vpc-123", "sub-456")
	assert.NoError(t, err)
}

func TestDeconfigureInstancePorts_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{DeconfigureInstance: 31})
	defer srv.Close()

	err := p.DeconfigureInstancePorts(context.Background(), "inst-789", "mach-001")
	assert.NoError(t, err)
}

func TestSyncIBPartitionToFabric_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{CreateIBPartition: 40})
	defer srv.Close()

	err := p.SyncIBPartitionToFabric(context.Background(), "part-001", "tenant-a-partition", "0x0042", "tenant-a", []string{"guid-1", "guid-2"})
	assert.NoError(t, err)
}

func TestRemoveIBPartitionFromFabric_Success(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{DeleteIBPartition: 41})
	defer srv.Close()

	err := p.RemoveIBPartitionFromFabric(context.Background(), "part-001", "0x0042")
	assert.NoError(t, err)
}

func TestSyncIBPartitionToFabric_NoTemplate(t *testing.T) {
	p, srv := newTestProvider(t, aapMockHandler(t), TemplateConfig{})
	defer srv.Close()

	err := p.SyncIBPartitionToFabric(context.Background(), "part-001", "partition", "0x0042", "tenant-a", nil)
	assert.NoError(t, err)
}

func TestSyncVPCToFabric_NilClient(t *testing.T) {
	p := &AnsibleFabricProvider{
		config: ProviderConfig{
			Templates: TemplateConfig{CreateVPC: 10},
		},
		client: nil,
	}

	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
