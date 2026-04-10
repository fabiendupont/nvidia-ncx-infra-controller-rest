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
	"fmt"
	"os"
	"strconv"
	"time"
)

// FeatureConfig controls which NICo lifecycle events trigger NVUE
// configuration changes on the Spectrum switches. Disabling a feature
// causes the provider to skip the corresponding hooks.
type FeatureConfig struct {
	// SyncVPC enables VRF creation/deletion on VPC lifecycle events.
	SyncVPC bool `json:"sync_vpc"`

	// SyncSubnet enables VxLAN VNI creation/deletion on subnet lifecycle events.
	SyncSubnet bool `json:"sync_subnet"`
}

// ProviderConfig holds the full configuration for the spectrum-fabric provider.
type ProviderConfig struct {
	// NVUEURL is the base URL of the NVUE REST API on the Spectrum switch
	// (e.g., "https://spine01.lab:8765").
	NVUEURL string

	// NVUEUsername is the username for HTTP basic auth against the NVUE API.
	NVUEUsername string

	// NVUEPassword is the password for HTTP basic auth against the NVUE API.
	NVUEPassword string

	// TLSSkipVerify disables TLS certificate verification when connecting
	// to the NVUE API. Useful for lab environments with self-signed certs.
	TLSSkipVerify bool

	// Features controls which lifecycle events trigger fabric sync.
	Features FeatureConfig

	// RevisionPollInterval is how often to poll NVUE for revision apply
	// completion. Defaults to 2 seconds if zero.
	RevisionPollInterval time.Duration

	// RevisionTimeout is the maximum time to wait for an NVUE revision
	// to be applied. Defaults to 2 minutes if zero.
	RevisionTimeout time.Duration
}

// ConfigFromEnv reads the spectrum-fabric provider configuration from
// environment variables.
func ConfigFromEnv() ProviderConfig {
	cfg := ProviderConfig{
		NVUEURL:       os.Getenv("NVUE_URL"),
		NVUEUsername:  os.Getenv("NVUE_USERNAME"),
		NVUEPassword:  os.Getenv("NVUE_PASSWORD"),
		TLSSkipVerify: envBool("NVUE_TLS_SKIP_VERIFY"),
		Features: FeatureConfig{
			SyncVPC:    envBoolDefault("NVUE_SYNC_VPC", true),
			SyncSubnet: envBoolDefault("NVUE_SYNC_SUBNET", true),
		},
	}

	if d := os.Getenv("NVUE_REVISION_POLL_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			cfg.RevisionPollInterval = parsed
		}
	}

	if d := os.Getenv("NVUE_REVISION_TIMEOUT"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			cfg.RevisionTimeout = parsed
		}
	}

	return cfg
}

// Validate checks that the minimum required configuration is present.
func (c *ProviderConfig) Validate() error {
	if c.NVUEURL == "" {
		return fmt.Errorf("NVUE_URL is required")
	}
	if c.NVUEUsername == "" {
		return fmt.Errorf("NVUE_USERNAME is required")
	}
	if c.NVUEPassword == "" {
		return fmt.Errorf("NVUE_PASSWORD is required")
	}
	return nil
}

func envBool(key string) bool {
	v := os.Getenv(key)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

func envBoolDefault(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}
