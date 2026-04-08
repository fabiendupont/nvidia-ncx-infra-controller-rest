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

package health

import "time"

// Fault event states.
const (
	FaultStateOpen         = "open"
	FaultStateAcknowledged = "acknowledged"
	FaultStateRemediating  = "remediating"
	FaultStateResolved     = "resolved"
	FaultStateEscalated    = "escalated"
	FaultStateSuppressed   = "suppressed"
)

// Severities.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Components.
const (
	ComponentGPU      = "gpu"
	ComponentNVSwitch = "nvswitch"
	ComponentNetwork  = "network"
	ComponentStorage  = "storage"
	ComponentPower    = "power"
	ComponentCooling  = "cooling"
	ComponentMemory   = "memory"
	ComponentCPU      = "cpu"
	ComponentBMC      = "bmc"
)

// FaultEvent represents an infrastructure fault tracked through its full
// lifecycle: detection, classification, remediation, and resolution.
type FaultEvent struct {
	ID                    string                 `json:"id"`
	OrgID                 string                 `json:"org_id"`
	TenantID              *string                `json:"tenant_id,omitempty"`
	SiteID                string                 `json:"site_id"`
	MachineID             *string                `json:"machine_id,omitempty"`
	InstanceID            *string                `json:"instance_id,omitempty"`
	Source                string                 `json:"source"`
	Severity              string                 `json:"severity"`
	Component             string                 `json:"component"`
	Classification        *string                `json:"classification,omitempty"`
	Message               string                 `json:"message"`
	State                 string                 `json:"state"`
	DetectedAt            time.Time              `json:"detected_at"`
	AcknowledgedAt        *time.Time             `json:"acknowledged_at,omitempty"`
	ResolvedAt            *time.Time             `json:"resolved_at,omitempty"`
	SuppressedUntil       *time.Time             `json:"suppressed_until,omitempty"`
	RemediationWorkflowID *string                `json:"remediation_workflow_id,omitempty"`
	RemediationAttempts   int                    `json:"remediation_attempts"`
	EscalationLevel       int                    `json:"escalation_level"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// ServiceEvent represents a tenant-facing service disruption derived from
// one or more underlying fault events. No infrastructure details are exposed.
type ServiceEvent struct {
	ID                    string     `json:"id"`
	OrgID                 string     `json:"org_id"`
	TenantID              string     `json:"tenant_id"`
	InstanceID            *string    `json:"instance_id,omitempty"`
	Summary               string     `json:"summary"`
	Impact                string     `json:"impact"`
	State                 string     `json:"state"`
	StartedAt             time.Time  `json:"started_at"`
	EstimatedResolutionAt *time.Time `json:"estimated_resolution_at,omitempty"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
	DowntimeExcluded      bool       `json:"downtime_excluded"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// FaultIngestionRequest is the webhook payload for fault ingestion.
type FaultIngestionRequest struct {
	Source         string                 `json:"source"`
	Severity       string                 `json:"severity"`
	Component      string                 `json:"component"`
	Classification *string                `json:"classification,omitempty"`
	Message        string                 `json:"message"`
	MachineID      *string                `json:"machine_id,omitempty"`
	DetectedAt     *time.Time             `json:"detected_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// FaultEventFilter contains fields for querying fault events.
type FaultEventFilter struct {
	SiteID    *string
	MachineID *string
	Severity  []string
	Component []string
	State     []string
	Source    *string
}

// MachineContext holds the IDs resolved from a machine lookup.
type MachineContext struct {
	SiteID     string
	TenantID   *string
	InstanceID *string
}

// ClassificationMapping defines the remediation strategy for a fault
// classification.
type ClassificationMapping struct {
	Classification  string `json:"classification"`
	Component       string `json:"component"`
	Severity        string `json:"severity"`
	Remediation     string `json:"remediation"`
	MaxRetries      int    `json:"max_retries"`
	ValidationLevel int    `json:"validation_level,omitempty"`
	RecheckInterval string `json:"recheck_interval,omitempty"`
	EscalateMessage string `json:"escalate_message,omitempty"`
}

// FaultSummary is the aggregated response for the summary endpoint.
type FaultSummary struct {
	BySeverity  map[string]int     `json:"by_severity"`
	ByComponent map[string]int     `json:"by_component"`
	ByState     map[string]int     `json:"by_state"`
	BySite      []SiteFaultSummary `json:"by_site"`
}

// SiteFaultSummary holds per-site fault counts.
type SiteFaultSummary struct {
	SiteID   string `json:"site_id"`
	Open     int    `json:"open"`
	Critical int    `json:"critical"`
}
