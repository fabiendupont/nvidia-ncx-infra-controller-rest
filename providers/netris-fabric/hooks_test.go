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

package netrisfabric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPrefix_MapStringInterface_Prefix(t *testing.T) {
	payload := map[string]interface{}{
		"prefix": "10.0.0.0/24",
		"name":   "my-subnet",
	}

	prefix, err := extractPrefix(payload)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/24", prefix)
}

func TestExtractPrefix_MapStringInterface_CIDR(t *testing.T) {
	payload := map[string]interface{}{
		"cidr": "192.168.1.0/24",
	}

	prefix, err := extractPrefix(payload)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0/24", prefix)
}

func TestExtractPrefix_MapStringString_Prefix(t *testing.T) {
	payload := map[string]string{
		"prefix": "172.16.0.0/12",
	}

	prefix, err := extractPrefix(payload)
	require.NoError(t, err)
	assert.Equal(t, "172.16.0.0/12", prefix)
}

func TestExtractPrefix_NilPayload(t *testing.T) {
	_, err := extractPrefix(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil payload")
}

func TestExtractPrefix_EmptyMap(t *testing.T) {
	payload := map[string]interface{}{}

	_, err := extractPrefix(payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no prefix or cidr field found")
}

func TestExtractPrefix_WrongType(t *testing.T) {
	// A type that extractPrefix does not handle.
	payload := 42

	_, err := extractPrefix(payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no prefix or cidr field found")
}
