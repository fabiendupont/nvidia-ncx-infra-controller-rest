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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- validateSubnetConfig ----------

func TestValidateSubnetConfig_Success(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	payload := map[string]interface{}{
		"prefix": "10.0.1.0/24",
		"vpc_id": "vpc-123",
	}

	err := p.validateSubnetConfig(context.Background(), payload)
	require.NoError(t, err)

	// Should have created a revision, patched, then deleted the revision.
	reqs := state.getRequests()
	require.True(t, len(reqs) >= 3)
	assert.Equal(t, "POST", reqs[0].Method)   // create revision
	assert.Equal(t, "PATCH", reqs[1].Method)  // patch validation probe
	assert.Equal(t, "DELETE", reqs[2].Method) // cleanup revision
}

func TestValidateSubnetConfig_NVUERejects(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueFailingHandler(t, state))
	defer srv.Close()

	payload := map[string]interface{}{
		"prefix": "10.0.1.0/24",
		"vpc_id": "vpc-123",
	}

	err := p.validateSubnetConfig(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVUE dry-run")
	assert.Contains(t, err.Error(), "rejected")
}

func TestValidateSubnetConfig_MissingPrefix(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	payload := map[string]interface{}{
		"vpc_id": "vpc-123",
	}

	// Should skip validation gracefully, not error.
	err := p.validateSubnetConfig(context.Background(), payload)
	require.NoError(t, err)
	assert.Empty(t, state.getRequests())
}

func TestValidateSubnetConfig_MissingVPCID(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	payload := map[string]interface{}{
		"prefix": "10.0.1.0/24",
	}

	err := p.validateSubnetConfig(context.Background(), payload)
	require.NoError(t, err)
	assert.Empty(t, state.getRequests())
}

func TestValidateSubnetConfig_NilPayload(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	err := p.validateSubnetConfig(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, state.getRequests())
}

func TestValidateSubnetConfig_AlternateFieldNames(t *testing.T) {
	state := &nvueMockState{}
	p, srv := newTestProvider(t, nvueSuccessHandler(t, state))
	defer srv.Close()

	// Uses "cidr" instead of "prefix" — should work via extractPayloadField.
	payload := map[string]interface{}{
		"cidr":   "10.0.2.0/24",
		"vpc_id": "vpc-456",
	}

	err := p.validateSubnetConfig(context.Background(), payload)
	require.NoError(t, err)

	reqs := state.getRequests()
	require.True(t, len(reqs) >= 3)
}

// ---------- extractPayloadField ----------

func TestExtractPayloadField_MapStringInterface(t *testing.T) {
	payload := map[string]interface{}{
		"prefix": "10.0.1.0/24",
		"vpc_id": "vpc-123",
	}

	v, err := extractPayloadField(payload, "prefix")
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.0/24", v)
}

func TestExtractPayloadField_MapStringString(t *testing.T) {
	payload := map[string]string{
		"prefix": "10.0.1.0/24",
		"vpc_id": "vpc-123",
	}

	v, err := extractPayloadField(payload, "prefix")
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.0/24", v)
}

func TestExtractPayloadField_FallbackName(t *testing.T) {
	payload := map[string]interface{}{
		"cidr": "10.0.2.0/24",
	}

	v, err := extractPayloadField(payload, "prefix", "cidr")
	require.NoError(t, err)
	assert.Equal(t, "10.0.2.0/24", v)
}

func TestExtractPayloadField_NotFound(t *testing.T) {
	payload := map[string]interface{}{
		"other": "value",
	}

	_, err := extractPayloadField(payload, "prefix", "cidr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no field")
}

func TestExtractPayloadField_NilPayload(t *testing.T) {
	_, err := extractPayloadField(nil, "prefix")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil payload")
}

func TestExtractPayloadField_EmptyString(t *testing.T) {
	payload := map[string]interface{}{
		"prefix": "",
	}

	_, err := extractPayloadField(payload, "prefix")
	require.Error(t, err)
}
