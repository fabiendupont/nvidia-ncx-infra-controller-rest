/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ufmfabric

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// FeatureConfig controls which NICo lifecycle events trigger UFM
// configuration changes. Disabling a feature causes the provider
// to skip the corresponding hooks.
type FeatureConfig struct {
	// SyncIBPartition enables PKEY creation/deletion on IB partition lifecycle events.
	SyncIBPartition bool `json:"sync_ib_partition"`
}

// ProviderConfig holds the configuration for the ufm-fabric provider.
type ProviderConfig struct {
	// UFMURL is the base URL of the UFM Enterprise REST API
	// (e.g., "https://ufm.lab:443").
	UFMURL string

	// UFMUsername is the username for HTTP basic auth against UFM.
	UFMUsername string

	// UFMPassword is the password for HTTP basic auth against UFM.
	UFMPassword string

	// TLSSkipVerify disables TLS certificate verification when connecting
	// to UFM. Useful for lab environments with self-signed certs.
	TLSSkipVerify bool

	// Features controls which lifecycle events trigger UFM sync.
	Features FeatureConfig

	// JobPollInterval is how often to poll UFM for async job completion.
	// Defaults to 2 seconds if zero.
	JobPollInterval time.Duration

	// JobTimeout is the maximum time to wait for a UFM job to complete.
	// Defaults to 2 minutes if zero.
	JobTimeout time.Duration
}

// ConfigFromEnv reads the ufm-fabric provider configuration from
// environment variables.
func ConfigFromEnv() ProviderConfig {
	cfg := ProviderConfig{
		UFMURL:        os.Getenv("UFM_URL"),
		UFMUsername:    os.Getenv("UFM_USERNAME"),
		UFMPassword:   os.Getenv("UFM_PASSWORD"),
		TLSSkipVerify: envBool("UFM_TLS_SKIP_VERIFY"),
		Features: FeatureConfig{
			SyncIBPartition: envBoolDefault("UFM_SYNC_IB_PARTITION", true),
		},
	}

	if d := os.Getenv("UFM_JOB_POLL_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			cfg.JobPollInterval = parsed
		}
	}

	if d := os.Getenv("UFM_JOB_TIMEOUT"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			cfg.JobTimeout = parsed
		}
	}

	return cfg
}

// Validate checks that the minimum required configuration is present.
func (c *ProviderConfig) Validate() error {
	if c.UFMURL == "" {
		return fmt.Errorf("UFM_URL is required")
	}
	if c.UFMUsername == "" {
		return fmt.Errorf("UFM_USERNAME is required")
	}
	if c.UFMPassword == "" {
		return fmt.Errorf("UFM_PASSWORD is required")
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
