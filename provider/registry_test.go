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

package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProvider is a minimal Provider implementation for testing.
type testProvider struct {
	name         string
	version      string
	features     []string
	dependencies []string
	initOrder    *[]string
}

func (p *testProvider) Name() string           { return p.name }
func (p *testProvider) Version() string        { return p.version }
func (p *testProvider) Features() []string     { return p.features }
func (p *testProvider) Dependencies() []string { return p.dependencies }
func (p *testProvider) Init(_ ProviderContext) error {
	if p.initOrder != nil {
		*p.initOrder = append(*p.initOrder, p.name)
	}
	return nil
}
func (p *testProvider) Shutdown(_ context.Context) error { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &testProvider{name: "test-provider", version: "1.0.0", features: []string{"compute"}}

	err := r.Register(p)
	require.NoError(t, err)

	got, ok := r.Get("test-provider")
	assert.True(t, ok)
	assert.Equal(t, "test-provider", got.Name())

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	r := NewRegistry()
	p := &testProvider{name: "test-provider", version: "1.0.0", features: []string{"compute"}}

	err := r.Register(p)
	require.NoError(t, err)

	err = r.Register(p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_FeatureConflict(t *testing.T) {
	r := NewRegistry()
	p1 := &testProvider{name: "provider-a", version: "1.0.0", features: []string{"compute"}}
	p2 := &testProvider{name: "provider-b", version: "1.0.0", features: []string{"compute"}}

	err := r.Register(p1)
	require.NoError(t, err)

	err = r.Register(p2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already provided by")
}

func TestRegistry_FeatureProvider(t *testing.T) {
	r := NewRegistry()
	p := &testProvider{name: "test-provider", version: "1.0.0", features: []string{"compute", "networking"}}

	err := r.Register(p)
	require.NoError(t, err)

	got, ok := r.FeatureProvider("compute")
	assert.True(t, ok)
	assert.Equal(t, "test-provider", got.Name())

	got, ok = r.FeatureProvider("networking")
	assert.True(t, ok)
	assert.Equal(t, "test-provider", got.Name())

	_, ok = r.FeatureProvider("storage")
	assert.False(t, ok)
}

func TestRegistry_DependencyResolution(t *testing.T) {
	var initOrder []string
	r := NewRegistry()

	// A depends on B — B should init first
	pA := &testProvider{name: "provider-a", version: "1.0.0", features: []string{"compute"}, dependencies: []string{"provider-b"}, initOrder: &initOrder}
	pB := &testProvider{name: "provider-b", version: "1.0.0", features: []string{"networking"}, initOrder: &initOrder}

	require.NoError(t, r.Register(pA))
	require.NoError(t, r.Register(pB))

	err := r.ResolveDependencies()
	require.NoError(t, err)

	err = r.InitAll(ProviderContext{})
	require.NoError(t, err)

	require.Len(t, initOrder, 2)
	assert.Equal(t, "provider-b", initOrder[0])
	assert.Equal(t, "provider-a", initOrder[1])
}

func TestRegistry_CircularDependency(t *testing.T) {
	r := NewRegistry()

	pA := &testProvider{name: "provider-a", version: "1.0.0", features: []string{"compute"}, dependencies: []string{"provider-b"}}
	pB := &testProvider{name: "provider-b", version: "1.0.0", features: []string{"networking"}, dependencies: []string{"provider-a"}}

	require.NoError(t, r.Register(pA))
	require.NoError(t, r.Register(pB))

	err := r.ResolveDependencies()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestRegistry_MissingDependency(t *testing.T) {
	r := NewRegistry()

	pA := &testProvider{name: "provider-a", version: "1.0.0", features: []string{"compute"}, dependencies: []string{"provider-missing"}}

	require.NoError(t, r.Register(pA))

	err := r.ResolveDependencies()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestRegistry_MultiLevelDependency(t *testing.T) {
	var initOrder []string
	r := NewRegistry()

	// C depends on B, B depends on A
	pA := &testProvider{name: "a", version: "1.0.0", features: []string{"f1"}, initOrder: &initOrder}
	pB := &testProvider{name: "b", version: "1.0.0", features: []string{"f2"}, dependencies: []string{"a"}, initOrder: &initOrder}
	pC := &testProvider{name: "c", version: "1.0.0", features: []string{"f3"}, dependencies: []string{"b"}, initOrder: &initOrder}

	require.NoError(t, r.Register(pC))
	require.NoError(t, r.Register(pA))
	require.NoError(t, r.Register(pB))

	err := r.ResolveDependencies()
	require.NoError(t, err)

	err = r.InitAll(ProviderContext{})
	require.NoError(t, err)

	require.Len(t, initOrder, 3)
	// A must come before B, B must come before C
	aIdx, bIdx, cIdx := indexOf(initOrder, "a"), indexOf(initOrder, "b"), indexOf(initOrder, "c")
	assert.Less(t, aIdx, bIdx)
	assert.Less(t, bIdx, cIdx)
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
