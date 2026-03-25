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

func TestValidate_MissingURL(t *testing.T) {
	cfg := ProviderConfig{
		AAPToken: "test-token",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AAP_URL")
}

func TestValidate_MissingAuth(t *testing.T) {
	cfg := ProviderConfig{
		AAPURL: "https://aap.example.com",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AAP_TOKEN")
}

func TestValidate_TokenAuth(t *testing.T) {
	cfg := ProviderConfig{
		AAPURL:   "https://aap.example.com",
		AAPToken: "test-token",
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestValidate_BasicAuth(t *testing.T) {
	cfg := ProviderConfig{
		AAPURL:      "https://aap.example.com",
		AAPUsername: "admin",
		AAPPassword: "password",
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestHasTemplate(t *testing.T) {
	tc := TemplateConfig{
		CreateVPC: 42,
		DeleteVPC: 0,
	}
	assert.True(t, tc.HasTemplate(tc.CreateVPC))
	assert.False(t, tc.HasTemplate(tc.DeleteVPC))
}

func TestEnvInt_Empty(t *testing.T) {
	assert.Equal(t, 0, envInt("NONEXISTENT_ENV_VAR_FOR_TEST"))
}
