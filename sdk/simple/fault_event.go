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

package simple

import (
	"context"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard"
)

// FaultEvent represents an infrastructure fault in the simple SDK.
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

// FaultIngestionRequest is the payload for reporting a fault.
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

// FaultEventUpdateRequest is a request to update a fault event.
type FaultEventUpdateRequest struct {
	State           *string                `json:"state,omitempty"`
	SuppressedUntil *time.Time             `json:"suppressed_until,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// FaultEventFilter encapsulates filter parameters for listing fault events.
type FaultEventFilter struct {
	Severity  *string
	Component *string
	State     *string
	SiteID    *string
	MachineID *string
	Sort      *string
	Order     *string
}

// FaultSummary is the aggregated fault summary response.
type FaultSummary = standard.FaultSummary

// ServiceEventResponse is the tenant-safe view of a service event.
type ServiceEventResponse = standard.ServiceEventResponse

// FaultEventManager manages fault event and service event operations.
type FaultEventManager struct {
	client *Client
}

// NewFaultEventManager creates a new FaultEventManager.
func NewFaultEventManager(client *Client) FaultEventManager {
	return FaultEventManager{client: client}
}

func faultEventFromStandard(api standard.FaultEvent) FaultEvent {
	e := FaultEvent{
		Metadata: api.Metadata,
	}
	if api.Id != nil {
		e.ID = *api.Id
	}
	if api.OrgId != nil {
		e.OrgID = *api.OrgId
	}
	e.TenantID = api.TenantId
	if api.SiteId != nil {
		e.SiteID = *api.SiteId
	}
	e.MachineID = api.MachineId
	e.InstanceID = api.InstanceId
	if api.Source != nil {
		e.Source = *api.Source
	}
	if api.Severity != nil {
		e.Severity = *api.Severity
	}
	if api.Component != nil {
		e.Component = *api.Component
	}
	e.Classification = api.Classification
	if api.Message != nil {
		e.Message = *api.Message
	}
	if api.State != nil {
		e.State = *api.State
	}
	if api.DetectedAt != nil {
		e.DetectedAt = *api.DetectedAt
	}
	e.AcknowledgedAt = api.AcknowledgedAt
	e.ResolvedAt = api.ResolvedAt
	e.SuppressedUntil = api.SuppressedUntil
	e.RemediationWorkflowID = api.RemediationWorkflowId
	if api.RemediationAttempts != nil {
		e.RemediationAttempts = int(*api.RemediationAttempts)
	}
	if api.EscalationLevel != nil {
		e.EscalationLevel = int(*api.EscalationLevel)
	}
	if api.CreatedAt != nil {
		e.CreatedAt = *api.CreatedAt
	}
	if api.UpdatedAt != nil {
		e.UpdatedAt = *api.UpdatedAt
	}
	return e
}

// IngestFaultEvent reports a new fault event.
func (fm FaultEventManager) IngestFaultEvent(ctx context.Context, request FaultIngestionRequest) (*FaultEvent, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	apiReq := standard.FaultIngestionRequest{
		Source:    request.Source,
		Severity:  request.Severity,
		Component: request.Component,
		Message:   request.Message,
	}
	apiReq.Classification = request.Classification
	apiReq.MachineId = request.MachineID
	apiReq.DetectedAt = request.DetectedAt
	apiReq.Metadata = request.Metadata

	result, resp, err := fm.client.apiClient.HealthAPI.IngestFaultEvent(ctx, fm.client.apiMetadata.Organization).
		FaultIngestionRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	e := faultEventFromStandard(*result)
	return &e, nil
}

// GetFaultEvents returns all fault events with optional filtering.
func (fm FaultEventManager) GetFaultEvents(ctx context.Context, filter *FaultEventFilter, paginationFilter *OffsetPaginationFilter) ([]FaultEvent, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	req := fm.client.apiClient.HealthAPI.ListFaultEvents(ctx, fm.client.apiMetadata.Organization)
	if filter != nil {
		if filter.Severity != nil {
			req = req.Severity(*filter.Severity)
		}
		if filter.Component != nil {
			req = req.Component(*filter.Component)
		}
		if filter.State != nil {
			req = req.State(*filter.State)
		}
		if filter.SiteID != nil {
			req = req.SiteId(*filter.SiteID)
		}
		if filter.MachineID != nil {
			req = req.MachineId(*filter.MachineID)
		}
		if filter.Sort != nil {
			req = req.Sort(*filter.Sort)
		}
		if filter.Order != nil {
			req = req.Order(*filter.Order)
		}
	}
	if paginationFilter != nil {
		if paginationFilter.Offset != nil {
			req = req.Offset(int32(*paginationFilter.Offset))
		}
		if paginationFilter.Limit != nil {
			req = req.Limit(int32(*paginationFilter.Limit))
		}
	}

	apiEvents, resp, err := req.Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}

	events := make([]FaultEvent, 0, len(apiEvents))
	for _, api := range apiEvents {
		events = append(events, faultEventFromStandard(api))
	}
	return events, nil
}

// GetFaultEvent returns a fault event by ID.
func (fm FaultEventManager) GetFaultEvent(ctx context.Context, id string) (*FaultEvent, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	result, resp, err := fm.client.apiClient.HealthAPI.GetFaultEvent(ctx, fm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	e := faultEventFromStandard(*result)
	return &e, nil
}

// UpdateFaultEvent updates a fault event (acknowledge, suppress, resolve).
func (fm FaultEventManager) UpdateFaultEvent(ctx context.Context, id string, request FaultEventUpdateRequest) (*FaultEvent, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	apiReq := standard.UpdateFaultEventRequest{}
	apiReq.State = request.State
	apiReq.SuppressedUntil = request.SuppressedUntil
	apiReq.Metadata = request.Metadata

	result, resp, err := fm.client.apiClient.HealthAPI.UpdateFaultEvent(ctx, fm.client.apiMetadata.Organization, id).
		UpdateFaultEventRequest(apiReq).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	e := faultEventFromStandard(*result)
	return &e, nil
}

// GetFaultSummary returns aggregated fault counts.
func (fm FaultEventManager) GetFaultSummary(ctx context.Context) (*FaultSummary, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	result, resp, err := fm.client.apiClient.HealthAPI.GetFaultSummary(ctx, fm.client.apiMetadata.Organization).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// TriggerFaultRemediation triggers remediation for a fault event.
func (fm FaultEventManager) TriggerFaultRemediation(ctx context.Context, id string) (*FaultEvent, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	result, resp, err := fm.client.apiClient.HealthAPI.TriggerFaultRemediation(ctx, fm.client.apiMetadata.Organization, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	e := faultEventFromStandard(*result)
	return &e, nil
}

// GetServiceEvents returns service events for a tenant.
func (fm FaultEventManager) GetServiceEvents(ctx context.Context, tenantID string) ([]ServiceEventResponse, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	result, resp, err := fm.client.apiClient.HealthAPI.ListServiceEvents(ctx, fm.client.apiMetadata.Organization, tenantID).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetActiveServiceEvents returns active service events for a tenant.
func (fm FaultEventManager) GetActiveServiceEvents(ctx context.Context, tenantID string) ([]ServiceEventResponse, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	result, resp, err := fm.client.apiClient.HealthAPI.GetActiveServiceEvents(ctx, fm.client.apiMetadata.Organization, tenantID).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}

// GetServiceEvent returns a single service event.
func (fm FaultEventManager) GetServiceEvent(ctx context.Context, tenantID string, id string) (*ServiceEventResponse, *ApiError) {
	ctx = WithLogger(ctx, fm.client.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, fm.client.Config.Token)

	result, resp, err := fm.client.apiClient.HealthAPI.GetServiceEvent(ctx, fm.client.apiMetadata.Organization, tenantID, id).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result, nil
}
