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

package networking

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

func TestProviderName(t *testing.T) {
	p := New()
	assert.Equal(t, "nico-networking", p.Name())
}

func TestProviderVersion(t *testing.T) {
	p := New()
	assert.Equal(t, "1.0.6", p.Version())
}

func TestProviderFeatures(t *testing.T) {
	p := New()
	assert.Equal(t, []string{"networking"}, p.Features())
}

func TestProviderDependencies(t *testing.T) {
	p := New()
	assert.Nil(t, p.Dependencies())
}

func TestProviderInit(t *testing.T) {
	p := New()
	ctx := provider.ProviderContext{
		Config:   &config.Config{},
		Registry: provider.NewRegistry(),
	}
	err := p.Init(ctx)
	require.NoError(t, err)
}

func TestProviderShutdown(t *testing.T) {
	p := New()
	err := p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestProviderServiceNilBeforeInit(t *testing.T) {
	p := New()
	assert.Nil(t, p.Service())
}

func TestProviderServiceAfterInit(t *testing.T) {
	p := New()
	ctx := provider.ProviderContext{
		Config:   &config.Config{},
		Registry: provider.NewRegistry(),
	}
	require.NoError(t, p.Init(ctx))
	assert.NotNil(t, p.Service())
}
