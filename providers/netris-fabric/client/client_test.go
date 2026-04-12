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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// netrisMockHandler simulates the Netris Controller REST API for testing.
func netrisMockHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Auth
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			json.NewEncoder(w).Encode(loginResponse{Token: "test-token"})

		// VPC CRUD
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/vpc":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(NetrisVPC{ID: 1, Name: "test-vpc"})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/vpc":
			json.NewEncoder(w).Encode([]NetrisVPC{
				{ID: 1, Name: "vpc-1"},
				{ID: 2, Name: "vpc-2"},
			})

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/vpc/"):
			json.NewEncoder(w).Encode(NetrisVPC{ID: 1, Name: "vpc-1"})

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v2/vpc/"):
			w.WriteHeader(http.StatusNoContent)

		// VNet CRUD
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/vnet":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(NetrisVNet{ID: 10, Name: "test-vnet"})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/vnet":
			json.NewEncoder(w).Encode([]NetrisVNet{{ID: 10, Name: "vnet-1"}})

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/vnet/"):
			json.NewEncoder(w).Encode(NetrisVNet{ID: 10, Name: "vnet-1"})

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v2/vnet/"):
			w.WriteHeader(http.StatusNoContent)

		// IPAM Allocation
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/ipam/allocation":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(NetrisAllocation{ID: 100, Name: "alloc-1", Prefix: "10.0.0.0/16"})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ipam/allocation":
			json.NewEncoder(w).Encode([]NetrisAllocation{{ID: 100, Prefix: "10.0.0.0/16"}})

		// IPAM Subnet
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/ipam/subnet":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(NetrisSubnet{ID: 200, Name: "subnet-1", Prefix: "10.0.1.0/24"})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ipam/subnet":
			json.NewEncoder(w).Encode([]NetrisSubnet{{ID: 200, Prefix: "10.0.1.0/24"}})

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v2/ipam/subnet/"):
			w.WriteHeader(http.StatusNoContent)

		// Port
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/port":
			json.NewEncoder(w).Encode([]NetrisPort{{ID: 1, Name: "swp1"}})

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/port/"):
			json.NewEncoder(w).Encode(NetrisPort{ID: 1, Name: "swp1", AdminState: "up"})

		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v2/port/"):
			json.NewEncoder(w).Encode(NetrisPort{ID: 1, Name: "swp1", MTU: 9000})

		default:
			http.NotFound(w, r)
		}
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(handler)
	c, err := New(Config{URL: srv.URL, Username: "admin", Password: "admin"})
	require.NoError(t, err)

	return c, srv
}

// ---------- Client ----------

func TestNew_MissingURL(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URL is required")
}

func TestNew_Success(t *testing.T) {
	c, err := New(Config{URL: "http://localhost"})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestLogin_Success(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	err := c.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-token", c.authToken)
}

func TestLogin_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL, Username: "bad", Password: "creds"})
	require.NoError(t, err)

	err = c.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// ---------- VPC ----------

func TestCreateVPC(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	vpc, err := c.CreateVPC(context.Background(), NetrisVPC{Name: "test-vpc"})
	require.NoError(t, err)
	assert.Equal(t, 1, vpc.ID)
	assert.Equal(t, "test-vpc", vpc.Name)
}

func TestGetVPC(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	vpc, err := c.GetVPC(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, vpc.ID)
}

func TestListVPCs(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	vpcs, err := c.ListVPCs(context.Background())
	require.NoError(t, err)
	assert.Len(t, vpcs, 2)
}

func TestDeleteVPC(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	err := c.DeleteVPC(context.Background(), 1)
	require.NoError(t, err)
}

// ---------- VNet ----------

func TestCreateVNet(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	vnet, err := c.CreateVNet(context.Background(), NetrisVNet{Name: "test-vnet"})
	require.NoError(t, err)
	assert.Equal(t, 10, vnet.ID)
}

func TestGetVNet(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	vnet, err := c.GetVNet(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 10, vnet.ID)
}

func TestListVNets(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	vnets, err := c.ListVNets(context.Background())
	require.NoError(t, err)
	assert.Len(t, vnets, 1)
}

func TestDeleteVNet(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	err := c.DeleteVNet(context.Background(), 10)
	require.NoError(t, err)
}

// ---------- IPAM ----------

func TestCreateAllocation(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	alloc, err := c.CreateAllocation(context.Background(), NetrisAllocation{Name: "alloc-1", Prefix: "10.0.0.0/16"})
	require.NoError(t, err)
	assert.Equal(t, 100, alloc.ID)
}

func TestListAllocations(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	allocs, err := c.ListAllocations(context.Background())
	require.NoError(t, err)
	assert.Len(t, allocs, 1)
}

func TestCreateSubnet(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	subnet, err := c.CreateSubnet(context.Background(), NetrisSubnet{Name: "subnet-1", Prefix: "10.0.1.0/24"})
	require.NoError(t, err)
	assert.Equal(t, 200, subnet.ID)
}

func TestListSubnets(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	subnets, err := c.ListSubnets(context.Background())
	require.NoError(t, err)
	assert.Len(t, subnets, 1)
}

func TestDeleteSubnet(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	err := c.DeleteSubnet(context.Background(), 200)
	require.NoError(t, err)
}

// ---------- Port ----------

func TestGetPort(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	port, err := c.GetPort(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "swp1", port.Name)
	assert.Equal(t, "up", port.AdminState)
}

func TestListPorts(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	ports, err := c.ListPorts(context.Background())
	require.NoError(t, err)
	assert.Len(t, ports, 1)
}

func TestUpdatePort(t *testing.T) {
	c, srv := newTestClient(t, netrisMockHandler(t))
	defer srv.Close()

	port, err := c.UpdatePort(context.Background(), 1, NetrisPort{MTU: 9000})
	require.NoError(t, err)
	assert.Equal(t, 9000, port.MTU)
}

// ---------- Error cases ----------

func TestCreateVPC_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL})
	require.NoError(t, err)

	_, err = c.CreateVPC(context.Background(), NetrisVPC{Name: "fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestDeleteVNet_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	c, err := New(Config{URL: srv.URL})
	require.NoError(t, err)

	err = c.DeleteVNet(context.Background(), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}
