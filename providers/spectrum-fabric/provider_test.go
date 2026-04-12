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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

func TestProviderName(t *testing.T) {
	p := New(ProviderConfig{})
	assert.Equal(t, "spectrum-fabric", p.Name())
}

func TestProviderVersion(t *testing.T) {
	p := New(ProviderConfig{})
	assert.Equal(t, "0.1.0", p.Version())
}

func TestProviderFeatures(t *testing.T) {
	p := New(ProviderConfig{})
	assert.Equal(t, []string{"spectrum-fabric"}, p.Features())
}

func TestProviderDependencies(t *testing.T) {
	p := New(ProviderConfig{})
	assert.Equal(t, []string{"nico-networking"}, p.Dependencies())
}

func TestProviderShutdown(t *testing.T) {
	p := New(ProviderConfig{})
	err := p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestProviderInit_MissingConfig(t *testing.T) {
	p := New(ProviderConfig{})
	ctx := provider.ProviderContext{
		Registry: provider.NewRegistry(),
	}

	err := p.Init(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVUE_URL")
}

func TestProviderInit_Success(t *testing.T) {
	// Stand up a mock NVUE server for the connectivity check.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"hostname": "spine01"})
	}))
	defer srv.Close()

	p := New(ProviderConfig{
		NVUEURL:      srv.URL,
		NVUEUsername:  "admin",
		NVUEPassword:  "admin",
	})

	ctx := provider.ProviderContext{
		Registry: provider.NewRegistry(),
	}

	err := p.Init(ctx)
	require.NoError(t, err)
	assert.NotNil(t, p.client)
}

func TestProviderInit_ConnectivityFailure(t *testing.T) {
	// Server that always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"down"}`))
	}))
	defer srv.Close()

	p := New(ProviderConfig{
		NVUEURL:      srv.URL,
		NVUEUsername:  "admin",
		NVUEPassword:  "admin",
	})

	ctx := provider.ProviderContext{
		Registry: provider.NewRegistry(),
	}

	err := p.Init(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVUE connectivity check failed")
}

func TestProviderInit_WithFeatures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"hostname": "spine01"})
	}))
	defer srv.Close()

	p := New(ProviderConfig{
		NVUEURL:      srv.URL,
		NVUEUsername:  "admin",
		NVUEPassword:  "admin",
		Features: FeatureConfig{
			SyncVPC:    true,
			SyncSubnet: true,
		},
	})

	ctx := provider.ProviderContext{
		Registry: provider.NewRegistry(),
	}

	// Init with all features enabled should succeed and register hooks.
	err := p.Init(ctx)
	require.NoError(t, err)
}
