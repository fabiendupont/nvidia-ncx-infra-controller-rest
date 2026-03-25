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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config holds the connection parameters for an Ansible Automation Platform
// Controller instance.
type Config struct {
	// URL is the base URL of the AAP Controller (e.g., "https://aap.example.com").
	URL string

	// Token is a personal access token or OAuth2 token for authentication.
	// Preferred over username/password for programmatic access.
	Token string

	// Username is the AAP Controller username. Used only if Token is empty.
	Username string

	// Password is the AAP Controller password. Used only if Token is empty.
	Password string

	// InsecureSkipVerify disables TLS certificate verification. Only for
	// development/testing environments.
	InsecureSkipVerify bool
}

// Client communicates with the Ansible Automation Platform Controller REST API
// (formerly AWX/Tower). It launches job templates and monitors job status.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	username   string
	password   string
}

// New creates a new AAP Controller API client.
func New(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("AAP Controller URL is required")
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:  strings.TrimRight(cfg.URL, "/"),
		token:    cfg.Token,
		username: cfg.Username,
		password: cfg.Password,
	}

	return c, nil
}

// doRequest builds and sends an authenticated HTTP request, returning the
// response body and status code.
func (c *Client) doRequest(ctx context.Context, method, path string, payload interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// Ping verifies connectivity to the AAP Controller by hitting the API root.
func (c *Client) Ping(ctx context.Context) error {
	_, statusCode, err := c.doRequest(ctx, http.MethodGet, "/api/v2/ping/", nil)
	if err != nil {
		return fmt.Errorf("pinging AAP Controller: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("AAP Controller ping returned status %d", statusCode)
	}
	return nil
}
