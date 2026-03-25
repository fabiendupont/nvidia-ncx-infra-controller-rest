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
)

// NetrisAllocation represents an IPAM allocation in Netris Controller.
type NetrisAllocation struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Prefix   string `json:"prefix"`
	TenantID int    `json:"tenantID"`
}

// NetrisSubnet represents an IPAM subnet in Netris Controller.
type NetrisSubnet struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Prefix   string `json:"prefix"`
	TenantID int    `json:"tenantID"`
	Purpose  string `json:"purpose"`
	VPCID    int    `json:"vpcID"`
}

// CreateAllocation creates a new IPAM allocation in the Netris Controller.
func (c *Client) CreateAllocation(ctx context.Context, alloc NetrisAllocation) (*NetrisAllocation, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, "/api/v2/ipam/allocation", alloc)
	if err != nil {
		return nil, fmt.Errorf("creating allocation: %w", err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return nil, fmt.Errorf("creating allocation: unexpected status %d: %s", statusCode, string(body))
	}

	var result NetrisAllocation
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding allocation response: %w", err)
	}

	return &result, nil
}

// ListAllocations retrieves all IPAM allocations from the Netris Controller.
func (c *Client) ListAllocations(ctx context.Context) ([]NetrisAllocation, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/api/v2/ipam/allocation", nil)
	if err != nil {
		return nil, fmt.Errorf("listing allocations: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("listing allocations: unexpected status %d: %s", statusCode, string(body))
	}

	var result []NetrisAllocation
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding allocations response: %w", err)
	}

	return result, nil
}

// CreateSubnet creates a new IPAM subnet in the Netris Controller.
func (c *Client) CreateSubnet(ctx context.Context, subnet NetrisSubnet) (*NetrisSubnet, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, "/api/v2/ipam/subnet", subnet)
	if err != nil {
		return nil, fmt.Errorf("creating subnet: %w", err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return nil, fmt.Errorf("creating subnet: unexpected status %d: %s", statusCode, string(body))
	}

	var result NetrisSubnet
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding subnet response: %w", err)
	}

	return &result, nil
}

// ListSubnets retrieves all IPAM subnets from the Netris Controller.
func (c *Client) ListSubnets(ctx context.Context) ([]NetrisSubnet, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/api/v2/ipam/subnet", nil)
	if err != nil {
		return nil, fmt.Errorf("listing subnets: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("listing subnets: unexpected status %d: %s", statusCode, string(body))
	}

	var result []NetrisSubnet
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding subnets response: %w", err)
	}

	return result, nil
}

// DeleteSubnet deletes an IPAM subnet by ID from the Netris Controller.
func (c *Client) DeleteSubnet(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v2/ipam/subnet/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting subnet %d: %w", id, err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return fmt.Errorf("deleting subnet %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	return nil
}
