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

package spectrumfabric

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabiendupont/nvidia-nvue-client-go/pkg/nvue"
)

// nvueMockState tracks requests received by the mock NVUE server so tests
// can assert on the API calls made by sync functions.
type nvueMockState struct {
	mu       sync.Mutex
	requests []mockRequest
}

type mockRequest struct {
	Method string
	Path   string
	Body   string
}

func (s *nvueMockState) record(method, path, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, mockRequest{Method: method, Path: path, Body: body})
}

func (s *nvueMockState) getRequests() []mockRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]mockRequest, len(s.requests))
	copy(cp, s.requests)
	return cp
}

// newTestProvider creates a SpectrumFabricProvider backed by an httptest
// server that simulates the NVUE REST API revision workflow.
func newTestProvider(t *testing.T, handler http.HandlerFunc) (*SpectrumFabricProvider, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(handler)

	client := nvue.NewClientFromURL(srv.URL, "test", "test",
		nvue.WithPollInterval(10*time.Millisecond),
		nvue.WithApplyTimeout(2*time.Second),
	)

	p := &SpectrumFabricProvider{
		config: ProviderConfig{
			NVUEURL:      srv.URL,
			NVUEUsername: "test",
			NVUEPassword: "test",
			Features: FeatureConfig{
				SyncVPC:    true,
				SyncSubnet: true,
			},
		},
		client: client,
	}

	return p, srv
}

// nvueSuccessHandler returns a handler that simulates a successful NVUE
// revision workflow: create revision → patch → apply → applied.
func nvueSuccessHandler(t *testing.T, state *nvueMockState) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			b, _ := readBody(r)
			body = string(b)
		}
		state.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")

		switch {
		// Create revision
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/revision"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]nvue.Revision{
				"rev-test-001": {State: "pending"},
			})

		// Apply revision (PATCH /revision/{id})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/revision/"):
			json.NewEncoder(w).Encode(nvue.Revision{State: "apply"})

		// Poll revision state (GET /revision/{id})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/revision/"):
			json.NewEncoder(w).Encode(nvue.Revision{State: "applied"})

		// PATCH any config path (VRF, interface, bridge, nve)
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)

		// DELETE any config path
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)

		// GET /system (connectivity check)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/system"):
			json.NewEncoder(w).Encode(map[string]any{"hostname": "spine01"})

		default:
			http.NotFound(w, r)
		}
	}
}

// nvueFailingHandler returns a handler where NVUE PATCH calls fail with
// a 400 error, simulating a configuration rejection.
func nvueFailingHandler(t *testing.T, state *nvueMockState) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			b, _ := readBody(r)
			body = string(b)
		}
		state.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")

		switch {
		// Create revision succeeds
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/revision"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]nvue.Revision{
				"rev-test-001": {State: "pending"},
			})

		// PATCH fails
		case r.Method == http.MethodPatch && !strings.Contains(r.URL.Path, "/revision/"):
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"invalid configuration"}`))

		// DELETE (for cleanup)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}
}

// nvueDeleteFailHandler returns a handler where revision creation succeeds
// but DELETE calls fail with a 400 error.
func nvueDeleteFailHandler(t *testing.T, state *nvueMockState) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			b, _ := readBody(r)
			body = string(b)
		}
		state.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/revision"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]nvue.Revision{
				"rev-test-001": {State: "pending"},
			})

		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"delete failed"}`))

		default:
			http.NotFound(w, r)
		}
	}
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	b := make([]byte, 0, 1024)
	buf := make([]byte, 256)
	for {
		n, err := r.Body.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil {
			break
		}
	}
	return b, nil
}

// ---------- SyncVPCToFabric ----------

func TestSyncVPCToFabric_Success(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	require.NoError(t, err)

	// Verify the revision workflow was followed.
	reqs := state.getRequests()
	require.True(t, len(reqs) >= 3, "expected at least 3 requests (create rev, patch, apply)")

	// First request: create revision.
	assert.Equal(t, http.MethodPost, reqs[0].Method)
	assert.True(t, strings.HasSuffix(reqs[0].Path, "/revision"))

	// Second request: patch VRF config.
	assert.Equal(t, http.MethodPatch, reqs[1].Method)
	assert.Contains(t, reqs[1].Path, "/vrf/nico-vpc-123")

	// Verify the VRF config payload includes BGP and EVPN.
	assert.Contains(t, reqs[1].Body, `"enable":"on"`)
}

func TestSyncVPCToFabric_Disabled(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	p.config.Features.SyncVPC = false

	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	require.NoError(t, err)

	// No requests should have been made.
	assert.Empty(t, state.getRequests())
}

func TestSyncVPCToFabric_NVUEError(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueFailingHandler(t, state))
	defer srv.Close()

	err := p.SyncVPCToFabric(context.Background(), "vpc-123", "my-vpc", "tenant-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating VRF nico-vpc-123")
}

// ---------- RemoveVPCFromFabric ----------

func TestRemoveVPCFromFabric_Success(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	err := p.RemoveVPCFromFabric(context.Background(), "vpc-456")
	require.NoError(t, err)

	reqs := state.getRequests()
	require.True(t, len(reqs) >= 3)

	// Create revision.
	assert.Equal(t, http.MethodPost, reqs[0].Method)

	// Delete VRF.
	assert.Equal(t, http.MethodDelete, reqs[1].Method)
	assert.Contains(t, reqs[1].Path, "/vrf/nico-vpc-456")
}

func TestRemoveVPCFromFabric_Disabled(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	p.config.Features.SyncVPC = false

	err := p.RemoveVPCFromFabric(context.Background(), "vpc-456")
	require.NoError(t, err)
	assert.Empty(t, state.getRequests())
}

func TestRemoveVPCFromFabric_DeleteError(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueDeleteFailHandler(t, state))
	defer srv.Close()

	err := p.RemoveVPCFromFabric(context.Background(), "vpc-456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting VRF nico-vpc-456")
}

// ---------- SyncSubnetToFabric ----------

func TestSyncSubnetToFabric_Success(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	err := p.SyncSubnetToFabric(context.Background(), "sub-001", "vpc-123", "10.0.1.1/24", "my-subnet", 100, 10100)
	require.NoError(t, err)

	reqs := state.getRequests()
	// create rev + 3 patches (bridge, interface, nve) + apply + poll = 6
	require.True(t, len(reqs) >= 5, "expected at least 5 requests")

	// Verify patches target the right paths.
	var patchPaths []string
	for _, r := range reqs {
		if r.Method == http.MethodPatch && !strings.Contains(r.Path, "/revision/") {
			patchPaths = append(patchPaths, r.Path)
		}
	}
	assert.Len(t, patchPaths, 3)
	assert.Contains(t, patchPaths[0], "/bridge/domain/br_default")
	assert.Contains(t, patchPaths[1], "/interface/vlan100")
	assert.Contains(t, patchPaths[2], "/nve/vxlan")

	// Verify bridge config includes VNI mapping.
	for _, r := range reqs {
		if r.Method == http.MethodPatch && strings.Contains(r.Path, "/bridge/") {
			assert.Contains(t, r.Body, `"100"`)
			assert.Contains(t, r.Body, `"10100"`)
		}
	}
}

func TestSyncSubnetToFabric_Disabled(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	p.config.Features.SyncSubnet = false

	err := p.SyncSubnetToFabric(context.Background(), "sub-001", "vpc-123", "10.0.1.1/24", "my-subnet", 100, 10100)
	require.NoError(t, err)
	assert.Empty(t, state.getRequests())
}

func TestSyncSubnetToFabric_NVUEError(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueFailingHandler(t, state))
	defer srv.Close()

	err := p.SyncSubnetToFabric(context.Background(), "sub-001", "vpc-123", "10.0.1.1/24", "my-subnet", 100, 10100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating VxLAN VNI 10100")
}

// ---------- RemoveSubnetFromFabric ----------

func TestRemoveSubnetFromFabric_Success(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	err := p.RemoveSubnetFromFabric(context.Background(), "sub-001", "vpc-123", 100)
	require.NoError(t, err)

	reqs := state.getRequests()
	require.True(t, len(reqs) >= 4)

	// Verify deletes target the right paths.
	var deletePaths []string
	for _, r := range reqs {
		if r.Method == http.MethodDelete {
			deletePaths = append(deletePaths, r.Path)
		}
	}
	assert.Len(t, deletePaths, 2)
	assert.Contains(t, deletePaths[0], "/interface/vlan100")
	assert.Contains(t, deletePaths[1], "/bridge/domain/br_default/vlan/100")
}

func TestRemoveSubnetFromFabric_Disabled(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	p.config.Features.SyncSubnet = false

	err := p.RemoveSubnetFromFabric(context.Background(), "sub-001", "vpc-123", 100)
	require.NoError(t, err)
	assert.Empty(t, state.getRequests())
}

func TestRemoveSubnetFromFabric_DeleteError(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueDeleteFailHandler(t, state))
	defer srv.Close()

	err := p.RemoveSubnetFromFabric(context.Background(), "sub-001", "vpc-123", 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting SVI vlan100")
}
