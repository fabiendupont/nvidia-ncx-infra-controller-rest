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

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
)

// HealthActivities groups all fault-remediation Temporal activities. Registered
// as a struct so Temporal discovers every exported method automatically.
type HealthActivities struct {
	faultStore          *FaultEventStore
	serviceEventStore   *ServiceEventStore
	classificationStore *ClassificationStore
}

// ClassifyAndRoute looks up the fault event, determines the remediation
// strategy from the classification-to-remediation mapping, and transitions
// the fault to the "remediating" state. Returns the mapping so downstream
// activities can use component-specific parameters (max_retries,
// validation_level, etc.).
func (a *HealthActivities) ClassifyAndRoute(ctx context.Context, faultEventID string) (*ClassificationMapping, error) {
	logger := log.With().Str("Activity", "ClassifyAndRoute").
		Str("FaultEventID", faultEventID).Logger()

	logger.Info().Msg("classifying fault event")

	fault, err := a.faultStore.GetByID(faultEventID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve fault event")
		return nil, fmt.Errorf("failed to retrieve fault event %s: %w", faultEventID, err)
	}

	// Already past classification — idempotent
	if fault.State == FaultStateRemediating || fault.State == FaultStateResolved || fault.State == FaultStateEscalated {
		logger.Info().Str("State", fault.State).Msg("fault already classified, skipping")
		if fault.Classification != nil {
			mapping, _ := a.classificationStore.Get(*fault.Classification)
			return mapping, nil
		}
		return nil, nil
	}

	// Suppressed faults are skipped
	if fault.State == FaultStateSuppressed {
		logger.Info().Msg("fault is suppressed, skipping")
		return nil, temporal.NewNonRetryableApplicationError(
			"fault is suppressed",
			"FAULT_SUPPRESSED",
			nil,
		)
	}

	// Look up remediation mapping
	var mapping *ClassificationMapping
	if fault.Classification != nil {
		mapping, _ = a.classificationStore.Get(*fault.Classification)
	}
	if mapping == nil {
		logger.Warn().Str("Classification", derefString(fault.Classification)).
			Msg("no remediation mapping found, escalating immediately")
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("no remediation mapping for classification %q", derefString(fault.Classification)),
			"NO_MAPPING",
			nil,
		)
	}

	// Transition to remediating
	now := time.Now()
	fault.State = FaultStateRemediating
	fault.AcknowledgedAt = &now
	if _, err := a.faultStore.Update(fault); err != nil {
		logger.Warn().Err(err).Msg("failed to update fault event state")
		return nil, fmt.Errorf("failed to update fault event %s: %w", faultEventID, err)
	}

	logger.Info().Str("Remediation", mapping.Remediation).Msg("fault classified and routed")
	return mapping, nil
}

// IsolateFault sets the affected machine into maintenance mode and creates
// a service_event for the affected tenant (if one is allocated). Idempotent:
// checks current maintenance state before acting.
func (a *HealthActivities) IsolateFault(ctx context.Context, faultEventID string) error {
	logger := log.With().Str("Activity", "IsolateFault").
		Str("FaultEventID", faultEventID).Logger()

	logger.Info().Msg("isolating fault")

	fault, err := a.faultStore.GetByID(faultEventID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve fault event")
		return fmt.Errorf("failed to retrieve fault event %s: %w", faultEventID, err)
	}

	// Set machine maintenance mode (placeholder — in production this calls
	// the compute service interface to PATCH the machine).
	if fault.MachineID != nil {
		logger.Info().Str("MachineID", *fault.MachineID).
			Msg("setting machine maintenance mode")
	}

	// Create service_event for affected tenant if an instance is allocated.
	if fault.TenantID != nil {
		now := time.Now()
		se := &ServiceEvent{
			OrgID:     fault.OrgID,
			TenantID:  *fault.TenantID,
			Summary:   fmt.Sprintf("Automated remediation in progress for %s fault", fault.Component),
			Impact:    fmt.Sprintf("1 %s temporarily unavailable", fault.Component),
			State:     "active",
			StartedAt: now,
		}
		if fault.InstanceID != nil {
			se.InstanceID = fault.InstanceID
		}
		if _, createErr := a.serviceEventStore.Create(se); createErr != nil {
			logger.Warn().Err(createErr).Msg("failed to create service event")
			return fmt.Errorf("failed to create service event for fault %s: %w", faultEventID, createErr)
		}
		logger.Info().Str("ServiceEventID", se.ID).Msg("service event created")
	}

	logger.Info().Msg("fault isolated")
	return nil
}

// RemediateGPU executes the GPU reset for the affected fault. In the
// prototype this is a placeholder that logs the action and updates state.
// In production the actual nvidia-smi call goes through the site agent.
func (a *HealthActivities) RemediateGPU(ctx context.Context, faultEventID string, mapping ClassificationMapping) error {
	logger := log.With().Str("Activity", "RemediateGPU").
		Str("FaultEventID", faultEventID).
		Str("Remediation", mapping.Remediation).Logger()

	logger.Info().Msg("starting GPU remediation")

	fault, err := a.faultStore.GetByID(faultEventID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve fault event")
		return fmt.Errorf("failed to retrieve fault event %s: %w", faultEventID, err)
	}

	// Already resolved or escalated — idempotent
	if fault.State == FaultStateResolved || fault.State == FaultStateEscalated {
		logger.Info().Str("State", fault.State).Msg("fault already handled, skipping remediation")
		return nil
	}

	// Placeholder: in production this would call the site agent to run
	// nvidia-smi --gpu-reset --id={gpu_index} on the target machine.
	logger.Info().Msgf("Executing GPU reset for fault %s", faultEventID)

	// Increment remediation attempts
	fault.RemediationAttempts++
	if _, err := a.faultStore.Update(fault); err != nil {
		logger.Warn().Err(err).Msg("failed to update remediation attempts")
		return fmt.Errorf("failed to update fault event %s: %w", faultEventID, err)
	}

	logger.Info().Int("Attempts", fault.RemediationAttempts).Msg("GPU remediation completed")
	return nil
}

// ValidateRecovery runs component-specific validation after remediation.
// In the prototype this is a placeholder that logs success. In production
// this would run DCGM diagnostics via the site agent.
func (a *HealthActivities) ValidateRecovery(ctx context.Context, faultEventID string, mapping ClassificationMapping) error {
	logger := log.With().Str("Activity", "ValidateRecovery").
		Str("FaultEventID", faultEventID).
		Int("ValidationLevel", mapping.ValidationLevel).Logger()

	logger.Info().Msg("validating recovery")

	fault, err := a.faultStore.GetByID(faultEventID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve fault event")
		return fmt.Errorf("failed to retrieve fault event %s: %w", faultEventID, err)
	}

	// Already resolved or escalated — idempotent
	if fault.State == FaultStateResolved || fault.State == FaultStateEscalated {
		logger.Info().Str("State", fault.State).Msg("fault already handled, skipping validation")
		return nil
	}

	// Placeholder: in production this would run dcgmi diag on the target
	// machine via the site agent and check the result.
	logger.Info().Msgf("Recovery validation passed for fault %s", faultEventID)

	logger.Info().Msg("recovery validated")
	return nil
}

// RestoreService removes maintenance mode from the affected machine,
// resolves the fault event, and resolves any linked service events.
// Idempotent: checks state before acting.
func (a *HealthActivities) RestoreService(ctx context.Context, faultEventID string) error {
	logger := log.With().Str("Activity", "RestoreService").
		Str("FaultEventID", faultEventID).Logger()

	logger.Info().Msg("restoring service")

	fault, err := a.faultStore.GetByID(faultEventID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve fault event")
		return fmt.Errorf("failed to retrieve fault event %s: %w", faultEventID, err)
	}

	// Already resolved — idempotent
	if fault.State == FaultStateResolved {
		logger.Info().Msg("fault already resolved, skipping")
		return nil
	}

	// Remove machine maintenance mode (placeholder — in production this
	// calls the compute service interface).
	if fault.MachineID != nil {
		logger.Info().Str("MachineID", *fault.MachineID).
			Msg("removing machine maintenance mode")
	}

	// Resolve fault event
	now := time.Now()
	fault.State = FaultStateResolved
	fault.ResolvedAt = &now
	if _, err := a.faultStore.Update(fault); err != nil {
		logger.Warn().Err(err).Msg("failed to resolve fault event")
		return fmt.Errorf("failed to resolve fault event %s: %w", faultEventID, err)
	}

	// Resolve linked service events for this tenant. Walk all active
	// service events for the tenant and resolve those matching the
	// component. This is a simplified approach; a production implementation
	// would use the fault_service_event join table.
	if fault.TenantID != nil {
		for _, se := range a.serviceEventStore.GetByTenantID(*fault.TenantID) {
			if se.State != "active" {
				continue
			}
			se.State = "resolved"
			resolvedAt := time.Now()
			se.ResolvedAt = &resolvedAt
			se.DowntimeExcluded = true
			if _, updateErr := a.serviceEventStore.Update(se); updateErr != nil {
				logger.Warn().Err(updateErr).Str("ServiceEventID", se.ID).
					Msg("failed to resolve service event")
			}
		}
	}

	logger.Info().Msg("service restored")
	return nil
}

// EscalateFault transitions the fault event to the "escalated" state,
// increments the escalation level, and records the escalation reason.
// Called when any remediation step fails after retries.
func (a *HealthActivities) EscalateFault(ctx context.Context, faultEventID string, reason string) error {
	logger := log.With().Str("Activity", "EscalateFault").
		Str("FaultEventID", faultEventID).
		Str("Reason", reason).Logger()

	logger.Info().Msg("escalating fault")

	fault, err := a.faultStore.GetByID(faultEventID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve fault event")
		return fmt.Errorf("failed to retrieve fault event %s: %w", faultEventID, err)
	}

	// Already escalated or resolved — idempotent
	if fault.State == FaultStateEscalated || fault.State == FaultStateResolved {
		logger.Info().Str("State", fault.State).Msg("fault already in terminal state, skipping")
		return nil
	}

	fault.State = FaultStateEscalated
	fault.EscalationLevel++
	if fault.Metadata == nil {
		fault.Metadata = make(map[string]interface{})
	}
	fault.Metadata["escalation_reason"] = reason

	if _, err := a.faultStore.Update(fault); err != nil {
		logger.Warn().Err(err).Msg("failed to escalate fault event")
		return fmt.Errorf("failed to escalate fault event %s: %w", faultEventID, err)
	}

	logger.Info().Int("EscalationLevel", fault.EscalationLevel).Msg("fault escalated")
	return nil
}

// derefString safely dereferences a string pointer, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
