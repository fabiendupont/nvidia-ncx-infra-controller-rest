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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_Success(t *testing.T) {
	cfg := ProviderConfig{
		NVUEURL:      "https://spine01:8765",
		NVUEUsername:  "admin",
		NVUEPassword:  "password",
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestValidate_MissingURL(t *testing.T) {
	cfg := ProviderConfig{
		NVUEUsername: "admin",
		NVUEPassword: "password",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVUE_URL")
}

func TestValidate_MissingUsername(t *testing.T) {
	cfg := ProviderConfig{
		NVUEURL:      "https://spine01:8765",
		NVUEPassword: "password",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVUE_USERNAME")
}

func TestValidate_MissingPassword(t *testing.T) {
	cfg := ProviderConfig{
		NVUEURL:     "https://spine01:8765",
		NVUEUsername: "admin",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVUE_PASSWORD")
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("NVUE_URL", "https://spine01:8765")
	t.Setenv("NVUE_USERNAME", "admin")
	t.Setenv("NVUE_PASSWORD", "secret")
	t.Setenv("NVUE_TLS_SKIP_VERIFY", "true")
	t.Setenv("NVUE_SYNC_VPC", "false")
	t.Setenv("NVUE_SYNC_SUBNET", "true")
	t.Setenv("NVUE_REVISION_POLL_INTERVAL", "5s")
	t.Setenv("NVUE_REVISION_TIMEOUT", "3m")

	cfg := ConfigFromEnv()

	assert.Equal(t, "https://spine01:8765", cfg.NVUEURL)
	assert.Equal(t, "admin", cfg.NVUEUsername)
	assert.Equal(t, "secret", cfg.NVUEPassword)
	assert.True(t, cfg.TLSSkipVerify)
	assert.False(t, cfg.Features.SyncVPC)
	assert.True(t, cfg.Features.SyncSubnet)
	assert.Equal(t, 5*time.Second, cfg.RevisionPollInterval)
	assert.Equal(t, 3*time.Minute, cfg.RevisionTimeout)
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	// Clear all env vars to test defaults.
	t.Setenv("NVUE_URL", "")
	t.Setenv("NVUE_USERNAME", "")
	t.Setenv("NVUE_PASSWORD", "")
	t.Setenv("NVUE_TLS_SKIP_VERIFY", "")
	t.Setenv("NVUE_SYNC_VPC", "")
	t.Setenv("NVUE_SYNC_SUBNET", "")
	t.Setenv("NVUE_REVISION_POLL_INTERVAL", "")
	t.Setenv("NVUE_REVISION_TIMEOUT", "")

	cfg := ConfigFromEnv()

	assert.Empty(t, cfg.NVUEURL)
	assert.False(t, cfg.TLSSkipVerify)
	assert.True(t, cfg.Features.SyncVPC, "SyncVPC should default to true")
	assert.True(t, cfg.Features.SyncSubnet, "SyncSubnet should default to true")
	assert.Equal(t, time.Duration(0), cfg.RevisionPollInterval)
	assert.Equal(t, time.Duration(0), cfg.RevisionTimeout)
}

func TestConfigFromEnv_InvalidDuration(t *testing.T) {
	t.Setenv("NVUE_URL", "https://spine01:8765")
	t.Setenv("NVUE_USERNAME", "admin")
	t.Setenv("NVUE_PASSWORD", "secret")
	t.Setenv("NVUE_REVISION_POLL_INTERVAL", "not-a-duration")
	t.Setenv("NVUE_REVISION_TIMEOUT", "also-bad")

	cfg := ConfigFromEnv()

	// Invalid durations should be silently ignored (zero value).
	assert.Equal(t, time.Duration(0), cfg.RevisionPollInterval)
	assert.Equal(t, time.Duration(0), cfg.RevisionTimeout)
}

func TestEnvBool(t *testing.T) {
	t.Setenv("TEST_BOOL_TRUE", "true")
	t.Setenv("TEST_BOOL_FALSE", "false")
	t.Setenv("TEST_BOOL_ONE", "1")
	t.Setenv("TEST_BOOL_INVALID", "notabool")

	assert.True(t, envBool("TEST_BOOL_TRUE"))
	assert.False(t, envBool("TEST_BOOL_FALSE"))
	assert.True(t, envBool("TEST_BOOL_ONE"))
	assert.False(t, envBool("TEST_BOOL_INVALID"))
	assert.False(t, envBool("TEST_BOOL_NONEXISTENT"))
}

func TestEnvBoolDefault(t *testing.T) {
	t.Setenv("TEST_BOOL_SET", "false")

	assert.False(t, envBoolDefault("TEST_BOOL_SET", true))
	assert.True(t, envBoolDefault("TEST_BOOL_UNSET_12345", true))
	assert.False(t, envBoolDefault("TEST_BOOL_UNSET_12345", false))
}
