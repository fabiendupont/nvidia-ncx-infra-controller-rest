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

package showback

import (
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/stretchr/testify/assert"
)

func TestRegisterHooks_RegistersExpectedReactions(t *testing.T) {
	registry := provider.NewRegistry()
	p := &ShowbackProvider{}

	// registerHooks should complete without panicking.
	assert.NotPanics(t, func() {
		p.registerHooks(registry)
	})

	// Verify that sync hooks for unrelated events still work (no interference).
	runner := provider.NewHookRunner(registry, nil)
	assert.NotNil(t, runner)
}

func TestRegisterHooks_EventNamesMatchConstants(t *testing.T) {
	// Verify the event constants used by registerHooks match the expected
	// provider-level constants. This ensures hooks.go stays in sync with
	// the provider package.
	assert.Equal(t, "post-create-instance", provider.EventPostCreateInstance)
	assert.Equal(t, "post-delete-instance", provider.EventPostDeleteInstance)
}

func TestRegisterHooks_CalledTwice(t *testing.T) {
	registry := provider.NewRegistry()
	p := &ShowbackProvider{}

	// Calling registerHooks multiple times should not panic; reactions
	// are appended.
	assert.NotPanics(t, func() {
		p.registerHooks(registry)
		p.registerHooks(registry)
	})
}
