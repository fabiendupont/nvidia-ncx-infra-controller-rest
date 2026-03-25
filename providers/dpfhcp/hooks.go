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

package dpfhcp

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// registerHooks registers async reactions and sync hooks for DPF HCP
// site lifecycle events.
func (p *DPFHCPProvider) registerHooks(registry *provider.Registry) {
	// Async: auto-provision on site creation
	registry.RegisterReaction(provider.Reaction{
		Feature:        "site",
		Event:          "post-create-site",
		TargetWorkflow: "dpfhcp-site-watcher",
		SignalName:     "site-created",
	})

	// Async: teardown on site deletion
	registry.RegisterReaction(provider.Reaction{
		Feature:        "site",
		Event:          provider.EventPostDeleteSiteComponents,
		TargetWorkflow: "dpfhcp-site-watcher",
		SignalName:     "site-deleted",
	})

	// Sync: block instance creation if DPF not ready
	registry.RegisterHook(provider.SyncHook{
		Feature: "compute",
		Event:   provider.EventPreCreateInstance,
		Handler: p.preCreateInstanceCheck,
	})
}

// preCreateInstanceCheck blocks instance creation if the DPF HCP
// infrastructure is required for the site but is not yet ready.
// Returns nil if DPF HCP is ready or not required for the site.
func (p *DPFHCPProvider) preCreateInstanceCheck(ctx context.Context, payload interface{}) error {
	logger := log.With().Str("Hook", "preCreateInstanceCheck").Logger()

	logger.Debug().Msg("checking DPF HCP readiness for instance creation")

	// Extract site ID from the payload. The payload is expected to carry
	// site identification; if it cannot be resolved, allow the operation
	// to proceed (DPF HCP may not be relevant for this site).
	siteID, ok := extractSiteID(payload)
	if !ok {
		logger.Debug().Msg("could not extract site ID from payload, allowing operation")
		return nil
	}

	record, err := p.store.GetBySiteID(siteID)
	if err != nil {
		// No provisioning record means DPF HCP is not configured for this site
		logger.Debug().Str("SiteID", siteID).Msg("no DPF HCP provisioning record found, allowing operation")
		return nil
	}

	if record.Status != StatusReady {
		logger.Warn().Str("SiteID", siteID).Str("Status", string(record.Status)).
			Msg("DPF HCP infrastructure is not ready")
		return fmt.Errorf("DPF HCP infrastructure for site %s is not ready (current status: %s)", siteID, string(record.Status))
	}

	logger.Debug().Str("SiteID", siteID).Msg("DPF HCP infrastructure is ready")
	return nil
}

// extractSiteID attempts to extract a site ID from the hook payload.
// Returns the site ID and true if successful, or empty string and false
// if the payload does not contain a site ID.
func extractSiteID(payload interface{}) (string, bool) {
	if payload == nil {
		return "", false
	}

	// Support map-based payloads with a "site_id" or "siteID" key
	if m, ok := payload.(map[string]interface{}); ok {
		if id, exists := m["site_id"]; exists {
			if s, ok := id.(string); ok {
				return s, true
			}
		}
		if id, exists := m["siteID"]; exists {
			if s, ok := id.(string); ok {
				return s, true
			}
		}
	}

	return "", false
}
