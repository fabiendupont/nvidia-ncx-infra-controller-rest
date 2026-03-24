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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookRunner_FireSync_NoHooks(t *testing.T) {
	registry := NewRegistry()
	runner := NewHookRunner(registry, nil)

	err := runner.FireSync(context.Background(), "compute", "post-create-instance", nil)
	assert.NoError(t, err)
}

func TestHookRunner_FireSync_NilRunner(t *testing.T) {
	var runner *HookRunner
	err := runner.FireSync(context.Background(), "compute", "post-create-instance", nil)
	assert.NoError(t, err)
}

func TestHookRunner_FireSync_Success(t *testing.T) {
	registry := NewRegistry()

	var received interface{}
	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "post-create-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			received = payload
			return nil
		},
	})

	runner := NewHookRunner(registry, nil)
	err := runner.FireSync(context.Background(), "compute", "post-create-instance", "instance-123")

	assert.NoError(t, err)
	assert.Equal(t, "instance-123", received)
}

func TestHookRunner_FireSync_PreHookAborts(t *testing.T) {
	registry := NewRegistry()

	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "pre-create-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			return errors.New("quota exceeded")
		},
	})

	runner := NewHookRunner(registry, nil)
	err := runner.FireSync(context.Background(), "compute", "pre-create-instance", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")
}

func TestHookRunner_FireSync_MultipleHooks(t *testing.T) {
	registry := NewRegistry()

	var order []string
	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "post-create-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			order = append(order, "hook-1")
			return nil
		},
	})
	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "post-create-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			order = append(order, "hook-2")
			return nil
		},
	})

	runner := NewHookRunner(registry, nil)
	err := runner.FireSync(context.Background(), "compute", "post-create-instance", nil)

	assert.NoError(t, err)
	assert.Equal(t, []string{"hook-1", "hook-2"}, order)
}

func TestHookRunner_FireSync_FirstErrorStops(t *testing.T) {
	registry := NewRegistry()

	var order []string
	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "pre-delete-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			order = append(order, "hook-1")
			return errors.New("blocked")
		},
	})
	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "pre-delete-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			order = append(order, "hook-2")
			return nil
		},
	})

	runner := NewHookRunner(registry, nil)
	err := runner.FireSync(context.Background(), "compute", "pre-delete-instance", nil)

	require.Error(t, err)
	assert.Equal(t, []string{"hook-1"}, order) // hook-2 never ran
}

func TestHookRunner_FireSync_DifferentEvents(t *testing.T) {
	registry := NewRegistry()

	var called bool
	registry.RegisterHook(SyncHook{
		Feature: "compute",
		Event:   "post-create-instance",
		Handler: func(ctx context.Context, payload interface{}) error {
			called = true
			return nil
		},
	})

	runner := NewHookRunner(registry, nil)
	// Fire a different event — the hook should NOT be called
	err := runner.FireSync(context.Background(), "compute", "post-delete-instance", nil)

	assert.NoError(t, err)
	assert.False(t, called)
}

func TestHookRunner_FireAsync_NilRunner(t *testing.T) {
	var runner *HookRunner
	// Should not panic
	runner.FireAsync(context.Background(), "compute", "post-create-instance", nil)
}

func TestHookRunner_FireAsync_NoReactions(t *testing.T) {
	registry := NewRegistry()
	runner := NewHookRunner(registry, nil)
	// Should not panic even with nil temporal client
	runner.FireAsync(context.Background(), "compute", "post-create-instance", nil)
}

func TestRegistry_RegisterReaction(t *testing.T) {
	registry := NewRegistry()

	registry.RegisterReaction(Reaction{
		Feature:        "compute",
		Event:          "post-create-instance",
		TargetWorkflow: "billing-watcher",
		SignalName:     "instance-created",
	})

	key := hookKey("compute", "post-create-instance")
	assert.Len(t, registry.hooks.reactions[key], 1)
	assert.Equal(t, "billing-watcher", registry.hooks.reactions[key][0].TargetWorkflow)
}
