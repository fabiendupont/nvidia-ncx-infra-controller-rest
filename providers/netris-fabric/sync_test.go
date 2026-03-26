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

package netrisfabric

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/providers/netris-fabric/client"
)

func TestIdMap_SetAndGetNetrisID(t *testing.T) {
	m := newIDMap()

	m.set("vpc-abc", 42)

	id, ok := m.getNetrisID("vpc-abc")
	assert.True(t, ok)
	assert.Equal(t, 42, id)
}

func TestIdMap_GetNetrisID_NotFound(t *testing.T) {
	m := newIDMap()

	id, ok := m.getNetrisID("does-not-exist")
	assert.False(t, ok)
	assert.Equal(t, 0, id)
}

func TestIdMap_Delete(t *testing.T) {
	m := newIDMap()

	m.set("vpc-abc", 42)
	m.delete("vpc-abc")

	_, ok := m.getNetrisID("vpc-abc")
	assert.False(t, ok)
}

func TestIdMap_Delete_NonExistent(t *testing.T) {
	m := newIDMap()

	// Should not panic when deleting a key that was never set.
	m.delete("never-set")

	_, ok := m.getNetrisID("never-set")
	assert.False(t, ok)
}

func TestIdMap_SetOverwrite(t *testing.T) {
	m := newIDMap()

	m.set("vpc-abc", 10)
	m.set("vpc-abc", 20)

	id, ok := m.getNetrisID("vpc-abc")
	assert.True(t, ok)
	assert.Equal(t, 20, id)
}

// newTestProvider creates a NetrisFabricProvider backed by an httptest server.
// The handler function receives all HTTP requests sent by the client.
func newTestProvider(t *testing.T, handler http.HandlerFunc) (*NetrisFabricProvider, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(handler)

	c, err := client.New(client.Config{
		URL:      srv.URL,
		Username: "test",
		Password: "test",
	})
	require.NoError(t, err)

	p := &NetrisFabricProvider{
		netrisURL:  srv.URL,
		netrisUser: "test",
		netrisPass: "test",
		client:     c,
		vpcIDs:     newIDMap(),
		subnetIDs:  newIDMap(),
	}

	return p, srv
}

func TestCheckIPAMConflict_NoConflict(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/ipam/allocation":
			json.NewEncoder(w).Encode([]client.NetrisAllocation{})
		case "/api/v2/ipam/subnet":
			json.NewEncoder(w).Encode([]client.NetrisSubnet{})
		default:
			http.NotFound(w, r)
		}
	})

	p, srv := newTestProvider(t, handler)
	defer srv.Close()

	err := p.CheckIPAMConflict(context.Background(), "10.100.0.0/24")
	assert.NoError(t, err)
}

func TestCheckIPAMConflict_ConflictWithAllocation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/ipam/allocation":
			json.NewEncoder(w).Encode([]client.NetrisAllocation{
				{ID: 1, Name: "mgmt-block", Prefix: "10.100.0.0/16"},
			})
		case "/api/v2/ipam/subnet":
			json.NewEncoder(w).Encode([]client.NetrisSubnet{})
		default:
			http.NotFound(w, r)
		}
	})

	p, srv := newTestProvider(t, handler)
	defer srv.Close()

	err := p.CheckIPAMConflict(context.Background(), "10.100.1.0/24")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mgmt-block")
	assert.Contains(t, err.Error(), "allocation")
}

func TestCheckIPAMConflict_ConflictWithSubnet(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/ipam/allocation":
			json.NewEncoder(w).Encode([]client.NetrisAllocation{})
		case "/api/v2/ipam/subnet":
			json.NewEncoder(w).Encode([]client.NetrisSubnet{
				{ID: 2, Name: "infra-subnet", Prefix: "10.100.1.0/24"},
			})
		default:
			http.NotFound(w, r)
		}
	})

	p, srv := newTestProvider(t, handler)
	defer srv.Close()

	err := p.CheckIPAMConflict(context.Background(), "10.100.0.0/16")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "infra-subnet")
	assert.Contains(t, err.Error(), "subnet")
}

func TestCheckIPAMConflict_InvalidPrefix(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
	})

	p, srv := newTestProvider(t, handler)
	defer srv.Close()

	err := p.CheckIPAMConflict(context.Background(), "not-a-cidr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CIDR")
}
