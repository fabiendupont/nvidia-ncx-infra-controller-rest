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
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers async reactions for instance lifecycle events.
func (p *ShowbackProvider) registerHooks(registry *provider.Registry) {
	// Register async reactions for instance lifecycle
	registry.RegisterReaction(provider.Reaction{
		Feature:        "compute",
		Event:          provider.EventPostCreateInstance,
		TargetWorkflow: "showback-metering-watcher",
		SignalName:     "instance-created",
	})
	registry.RegisterReaction(provider.Reaction{
		Feature:        "compute",
		Event:          provider.EventPostDeleteInstance,
		TargetWorkflow: "showback-metering-watcher",
		SignalName:     "instance-deleted",
	})
}
