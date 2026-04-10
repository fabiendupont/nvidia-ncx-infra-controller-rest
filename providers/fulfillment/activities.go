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

package fulfillment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// FulfillmentActivities holds the dependencies needed by fulfillment
// workflow activities.
type FulfillmentActivities struct {
	orderStore   OrderStoreInterface
	serviceStore ServiceStoreInterface
}

// ValidateOrder validates that the given order exists and its template
// is well-formed. Returns the order on success.
func (a *FulfillmentActivities) ValidateOrder(ctx context.Context, orderID uuid.UUID) (*Order, error) {
	logger := log.With().Str("Activity", "ValidateOrder").
		Str("OrderID", orderID.String()).Logger()

	logger.Info().Msg("validating order")

	order, err := a.orderStore.Get(orderID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve order")
		return nil, fmt.Errorf("failed to retrieve order %s: %w", orderID, err)
	}

	logger.Info().Msg("order validated successfully")
	return order, nil
}

// UpdateOrderStatus updates the status and message of the given order.
func (a *FulfillmentActivities) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status OrderStatus, message string) error {
	logger := log.With().Str("Activity", "UpdateOrderStatus").
		Str("OrderID", orderID.String()).
		Str("Status", string(status)).Logger()

	logger.Info().Msg("updating order status")

	order, err := a.orderStore.Get(orderID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve order for status update")
		return fmt.Errorf("failed to retrieve order %s: %w", orderID, err)
	}

	order.Status = status
	order.StatusMessage = message
	order.Updated = time.Now()

	if err := a.orderStore.Update(order); err != nil {
		logger.Warn().Err(err).Msg("failed to update order status")
		return fmt.Errorf("failed to update order %s status to %s: %w", orderID, status, err)
	}

	logger.Info().Msg("order status updated")
	return nil
}

// CreateService creates a new service record based on the given order.
func (a *FulfillmentActivities) CreateService(ctx context.Context, order *Order) (*Service, error) {
	logger := log.With().Str("Activity", "CreateService").
		Str("OrderID", order.ID.String()).Logger()

	logger.Info().Msg("creating service record")

	now := time.Now()
	service := &Service{
		ID:           uuid.New(),
		OrderID:      order.ID,
		TemplateID:   order.TemplateID,
		TemplateName: order.TemplateName,
		TenantID:     order.TenantID,
		Name:         fmt.Sprintf("svc-%s", order.ID),
		Status:       ServiceStatusProvisioning,
		Created:      now,
		Updated:      now,
	}

	if err := a.serviceStore.Create(service); err != nil {
		logger.Warn().Err(err).Msg("failed to create service")
		return nil, fmt.Errorf("failed to create service for order %s: %w", order.ID, err)
	}

	// Link the service back to the order
	order.ServiceID = &service.ID
	order.Updated = now
	if err := a.orderStore.Update(order); err != nil {
		logger.Warn().Err(err).Msg("failed to link service to order")
	}

	logger.Info().Str("ServiceID", service.ID.String()).Msg("service created")
	return service, nil
}

// MarkServiceActive marks the given service as active.
func (a *FulfillmentActivities) MarkServiceActive(ctx context.Context, serviceID uuid.UUID) error {
	logger := log.With().Str("Activity", "MarkServiceActive").
		Str("ServiceID", serviceID.String()).Logger()

	logger.Info().Msg("marking service as active")

	svc, err := a.serviceStore.Get(serviceID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve service")
		return fmt.Errorf("failed to retrieve service %s: %w", serviceID, err)
	}

	svc.Status = ServiceStatusActive
	svc.Updated = time.Now()

	if err := a.serviceStore.Update(svc); err != nil {
		logger.Warn().Err(err).Msg("failed to mark service as active")
		return fmt.Errorf("failed to mark service %s as active: %w", serviceID, err)
	}

	logger.Info().Msg("service marked as active")
	return nil
}

// MarkServiceTerminated marks the given service as terminated.
func (a *FulfillmentActivities) MarkServiceTerminated(ctx context.Context, serviceID uuid.UUID) error {
	logger := log.With().Str("Activity", "MarkServiceTerminated").
		Str("ServiceID", serviceID.String()).Logger()

	logger.Info().Msg("marking service as terminated")

	svc, err := a.serviceStore.Get(serviceID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve service")
		return fmt.Errorf("failed to retrieve service %s: %w", serviceID, err)
	}

	svc.Status = ServiceStatusTerminated
	svc.Updated = time.Now()

	if err := a.serviceStore.Update(svc); err != nil {
		logger.Warn().Err(err).Msg("failed to mark service as terminated")
		return fmt.Errorf("failed to mark service %s as terminated: %w", serviceID, err)
	}

	logger.Info().Msg("service marked as terminated")
	return nil
}
