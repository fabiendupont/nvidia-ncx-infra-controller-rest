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

package ansiblefabric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPayloadField_MapStringInterface(t *testing.T) {
	payload := map[string]interface{}{
		"prefix":  "10.0.0.0/24",
		"vpc_id":  "vpc-abc-123",
		"name":    "my-subnet",
	}

	prefix, err := extractPayloadField(payload, "prefix", "cidr")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/24", prefix)

	vpcID, err := extractPayloadField(payload, "vpc_id")
	require.NoError(t, err)
	assert.Equal(t, "vpc-abc-123", vpcID)
}

func TestExtractPayloadField_FallbackName(t *testing.T) {
	payload := map[string]interface{}{
		"cidr": "192.168.1.0/24",
	}

	// "prefix" not found, falls back to "cidr"
	prefix, err := extractPayloadField(payload, "prefix", "cidr")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0/24", prefix)
}

func TestExtractPayloadField_MapStringString(t *testing.T) {
	payload := map[string]string{
		"prefix": "172.16.0.0/12",
	}

	prefix, err := extractPayloadField(payload, "prefix")
	require.NoError(t, err)
	assert.Equal(t, "172.16.0.0/12", prefix)
}

func TestExtractPayloadField_NilPayload(t *testing.T) {
	_, err := extractPayloadField(nil, "prefix")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil payload")
}

func TestExtractPayloadField_EmptyMap(t *testing.T) {
	payload := map[string]interface{}{}

	_, err := extractPayloadField(payload, "prefix", "cidr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no field")
}

func TestExtractPayloadField_WrongType(t *testing.T) {
	payload := 42

	_, err := extractPayloadField(payload, "prefix")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no field")
}

func TestExtractPayloadField_EmptyString(t *testing.T) {
	payload := map[string]interface{}{
		"prefix": "",
		"cidr":   "10.0.0.0/8",
	}

	// Empty "prefix" should be skipped, falls back to "cidr"
	result, err := extractPayloadField(payload, "prefix", "cidr")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8", result)
}
