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
	"fmt"
	"sync"

	tsdkClient "go.temporal.io/sdk/client"
)

// Event names for hook registration. Convention: "pre-" runs before the
// operation and can abort it; "post-" runs after success.
const (
	EventPreCreateInstance  = "pre-create-instance"
	EventPostCreateInstance = "post-create-instance"
	EventPreDeleteInstance  = "pre-delete-instance"
	EventPostDeleteInstance = "post-delete-instance"

	EventPreCreateVPC  = "pre-create-vpc"
	EventPostCreateVPC = "post-create-vpc"
	EventPreDeleteVPC  = "pre-delete-vpc"
	EventPostDeleteVPC = "post-delete-vpc"

	EventPreCreateSubnet  = "pre-create-subnet"
	EventPostCreateSubnet = "post-create-subnet"
	EventPreDeleteSubnet  = "pre-delete-subnet"
	EventPostDeleteSubnet = "post-delete-subnet"

	EventPostCreateSite = "post-create-site"
	EventPostDeleteSite = "post-delete-site"

	EventPostDeleteSiteComponents = "post-delete-site-components"

	EventPostHealthEventIngested = "post-health-event-ingested"
	EventPreFaultRemediation     = "pre-fault-remediation"
	EventPostFaultRemediation    = "post-fault-remediation"
	EventPostFaultResolved       = "post-fault-resolved"
	EventPostFaultEscalated      = "post-fault-escalated"

	EventPreCreateInfiniBandPartition  = "pre-create-infiniband-partition"
	EventPostCreateInfiniBandPartition = "post-create-infiniband-partition"
	EventPreDeleteInfiniBandPartition  = "pre-delete-infiniband-partition"
	EventPostDeleteInfiniBandPartition = "post-delete-infiniband-partition"
)

// SyncHook runs inline in the activity execution context. Pre-hooks can
// return an error to abort the operation. Post-hooks that fail cause the
// activity to fail (Temporal retries).
type SyncHook struct {
	Feature string
	Event   string
	Handler func(ctx context.Context, payload interface{}) error
}

// Reaction fires a Temporal signal to a watcher workflow after an event.
// Non-blocking — the source activity does not wait for the watcher to
// process the signal. Durable — if the watcher is down, the signal is
// buffered by Temporal until the watcher restarts.
type Reaction struct {
	Feature        string
	Event          string
	TargetWorkflow string // Workflow ID of the long-running watcher
	SignalName     string // Signal channel name the watcher listens on
}

// hookKey creates a lookup key from feature+event.
func hookKey(feature, event string) string {
	return feature + ":" + event
}

// hookRegistry stores sync hooks and async reactions.
type hookRegistry struct {
	mu        sync.RWMutex
	hooks     map[string][]SyncHook
	reactions map[string][]Reaction
}

func newHookRegistry() *hookRegistry {
	return &hookRegistry{
		hooks:     make(map[string][]SyncHook),
		reactions: make(map[string][]Reaction),
	}
}

// RegisterHook adds a sync hook for a feature+event combination.
func (r *Registry) RegisterHook(hook SyncHook) {
	r.hooks.mu.Lock()
	defer r.hooks.mu.Unlock()
	key := hookKey(hook.Feature, hook.Event)
	r.hooks.hooks[key] = append(r.hooks.hooks[key], hook)
}

// RegisterReaction adds an async reaction for a feature+event combination.
func (r *Registry) RegisterReaction(reaction Reaction) {
	r.hooks.mu.Lock()
	defer r.hooks.mu.Unlock()
	key := hookKey(reaction.Feature, reaction.Event)
	r.hooks.reactions[key] = append(r.hooks.reactions[key], reaction)
}

// HookRunner provides hook firing capabilities to activities. It wraps the
// hook registry and the Temporal client for sending signals.
type HookRunner struct {
	hooks    *hookRegistry
	temporal tsdkClient.Client
}

// NewHookRunner creates a HookRunner from a registry and Temporal client.
func NewHookRunner(registry *Registry, temporal tsdkClient.Client) *HookRunner {
	return &HookRunner{
		hooks:    registry.hooks,
		temporal: temporal,
	}
}

// FireSync runs all sync hooks registered for the given feature+event.
// Returns the first error if any hook fails. For pre-hooks, this aborts
// the operation. For post-hooks, this causes the activity to fail and
// Temporal will retry.
func (hr *HookRunner) FireSync(ctx context.Context, feature, event string, payload interface{}) error {
	if hr == nil {
		return nil
	}
	hr.hooks.mu.RLock()
	hooks := hr.hooks.hooks[hookKey(feature, event)]
	hr.hooks.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook.Handler(ctx, payload); err != nil {
			return fmt.Errorf("hook %s:%s failed: %w", feature, event, err)
		}
	}
	return nil
}

// FireAsync sends Temporal signals to all registered reactions for the
// given feature+event. Non-blocking — errors are logged but do not fail
// the caller. The payload is sent as the signal argument.
func (hr *HookRunner) FireAsync(ctx context.Context, feature, event string, payload interface{}) {
	if hr == nil {
		return
	}
	hr.hooks.mu.RLock()
	reactions := hr.hooks.reactions[hookKey(feature, event)]
	hr.hooks.mu.RUnlock()

	for _, reaction := range reactions {
		err := hr.temporal.SignalWorkflow(ctx, reaction.TargetWorkflow, "", reaction.SignalName, payload)
		if err != nil {
			// Log but don't fail — the watcher may not be running yet
			// and the signal will be delivered when it starts.
			fmt.Printf("warning: failed to signal workflow %s for %s:%s: %v\n",
				reaction.TargetWorkflow, feature, event, err)
		}
	}
}
