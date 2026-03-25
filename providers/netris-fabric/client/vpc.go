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

// NetrisVPC represents a VPC resource in Netris Controller.
type NetrisVPC struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TenantID    int    `json:"tenantID"`
}

// CreateVPC creates a new VPC in the Netris Controller.
func (c *Client) CreateVPC(ctx context.Context, vpc NetrisVPC) (*NetrisVPC, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, "/api/v2/vpc", vpc)
	if err != nil {
		return nil, fmt.Errorf("creating VPC: %w", err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return nil, fmt.Errorf("creating VPC: unexpected status %d: %s", statusCode, string(body))
	}

	var result NetrisVPC
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding VPC response: %w", err)
	}

	return &result, nil
}

// GetVPC retrieves a VPC by ID from the Netris Controller.
func (c *Client) GetVPC(ctx context.Context, id int) (*NetrisVPC, error) {
	path := fmt.Sprintf("/api/v2/vpc/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting VPC %d: %w", id, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("getting VPC %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	var result NetrisVPC
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding VPC response: %w", err)
	}

	return &result, nil
}

// ListVPCs retrieves all VPCs from the Netris Controller.
func (c *Client) ListVPCs(ctx context.Context) ([]NetrisVPC, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/api/v2/vpc", nil)
	if err != nil {
		return nil, fmt.Errorf("listing VPCs: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("listing VPCs: unexpected status %d: %s", statusCode, string(body))
	}

	var result []NetrisVPC
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding VPCs response: %w", err)
	}

	return result, nil
}

// DeleteVPC deletes a VPC by ID from the Netris Controller.
func (c *Client) DeleteVPC(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v2/vpc/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting VPC %d: %w", id, err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return fmt.Errorf("deleting VPC %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	return nil
}
