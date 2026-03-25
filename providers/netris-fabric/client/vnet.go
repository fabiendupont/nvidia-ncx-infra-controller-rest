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

// NetrisVNetSite represents a site assignment within a VNet.
type NetrisVNetSite struct {
	SiteID   int      `json:"siteID"`
	Gateways []string `json:"gateways"`
}

// NetrisVNet represents a VNet resource in Netris Controller.
type NetrisVNet struct {
	ID       int              `json:"id"`
	Name     string           `json:"name"`
	TenantID int              `json:"tenantID"`
	VPCID    int              `json:"vpcID"`
	State    string           `json:"state"`
	Sites    []NetrisVNetSite `json:"sites"`
}

// CreateVNet creates a new VNet in the Netris Controller.
func (c *Client) CreateVNet(ctx context.Context, vnet NetrisVNet) (*NetrisVNet, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodPost, "/api/v2/vnet", vnet)
	if err != nil {
		return nil, fmt.Errorf("creating VNet: %w", err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return nil, fmt.Errorf("creating VNet: unexpected status %d: %s", statusCode, string(body))
	}

	var result NetrisVNet
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding VNet response: %w", err)
	}

	return &result, nil
}

// GetVNet retrieves a VNet by ID from the Netris Controller.
func (c *Client) GetVNet(ctx context.Context, id int) (*NetrisVNet, error) {
	path := fmt.Sprintf("/api/v2/vnet/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting VNet %d: %w", id, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("getting VNet %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	var result NetrisVNet
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding VNet response: %w", err)
	}

	return &result, nil
}

// ListVNets retrieves all VNets from the Netris Controller.
func (c *Client) ListVNets(ctx context.Context) ([]NetrisVNet, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/api/v2/vnet", nil)
	if err != nil {
		return nil, fmt.Errorf("listing VNets: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("listing VNets: unexpected status %d: %s", statusCode, string(body))
	}

	var result []NetrisVNet
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding VNets response: %w", err)
	}

	return result, nil
}

// DeleteVNet deletes a VNet by ID from the Netris Controller.
func (c *Client) DeleteVNet(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v2/vnet/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting VNet %d: %w", id, err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return fmt.Errorf("deleting VNet %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	return nil
}
