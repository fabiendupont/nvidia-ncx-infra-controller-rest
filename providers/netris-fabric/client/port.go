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

// NetrisPort represents a port resource in Netris Controller.
type NetrisPort struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	AdminState string `json:"adminState"`
	OperState  string `json:"operState"`
	MTU        int    `json:"mtu"`
	LACP       bool   `json:"lacp"`
}

// GetPort retrieves a port by ID from the Netris Controller.
func (c *Client) GetPort(ctx context.Context, id int) (*NetrisPort, error) {
	path := fmt.Sprintf("/api/v2/port/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting port %d: %w", id, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("getting port %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	var result NetrisPort
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding port response: %w", err)
	}

	return &result, nil
}

// UpdatePort updates a port by ID in the Netris Controller.
func (c *Client) UpdatePort(ctx context.Context, id int, port NetrisPort) (*NetrisPort, error) {
	path := fmt.Sprintf("/api/v2/port/%d", id)
	body, statusCode, err := c.doRequest(ctx, http.MethodPut, path, port)
	if err != nil {
		return nil, fmt.Errorf("updating port %d: %w", id, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("updating port %d: unexpected status %d: %s", id, statusCode, string(body))
	}

	var result NetrisPort
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding port response: %w", err)
	}

	return &result, nil
}

// ListPorts retrieves all ports from the Netris Controller.
func (c *Client) ListPorts(ctx context.Context) ([]NetrisPort, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/api/v2/port", nil)
	if err != nil {
		return nil, fmt.Errorf("listing ports: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("listing ports: unexpected status %d: %s", statusCode, string(body))
	}

	var result []NetrisPort
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding ports response: %w", err)
	}

	return result, nil
}
