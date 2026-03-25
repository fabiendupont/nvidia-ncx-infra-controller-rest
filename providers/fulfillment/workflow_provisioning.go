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
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TenantProvisioningWorkflow orchestrates the full provisioning sequence
// when a tenant places an order. Each step is a Temporal activity.
func TenantProvisioningWorkflow(ctx workflow.Context, orderID uuid.UUID) error {
	logger := log.With().Str("Workflow", "TenantProvisioning").
		Str("OrderID", orderID.String()).Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var activities FulfillmentActivities

	// Step 1: Validate order and template
	logger.Info().Msg("validating order")
	var order Order
	err := workflow.ExecuteActivity(ctx, activities.ValidateOrder, orderID).Get(ctx, &order)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to validate order")
		return err
	}

	// Step 2: Update order status to Provisioning
	logger.Info().Msg("updating order status to provisioning")
	err = workflow.ExecuteActivity(ctx, activities.UpdateOrderStatus, orderID, OrderStatusProvisioning, "provisioning started").Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to update order status")
		return err
	}

	// Step 3: Create service record
	logger.Info().Msg("creating service record")
	var service Service
	err = workflow.ExecuteActivity(ctx, activities.CreateService, &order).Get(ctx, &service)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create service record")
		return err
	}

	// Step 4: Placeholder: Create VPC (would call networking service)
	logger.Info().Msg("placeholder: VPC creation would happen here")

	// Step 5: Placeholder: Create allocation (would call compute service)
	logger.Info().Msg("placeholder: compute allocation would happen here")

	// Step 6: Placeholder: Deploy workload
	logger.Info().Msg("placeholder: workload deployment would happen here")

	// Step 7: Mark service as Active, order as Ready
	logger.Info().Msg("marking service as active")
	err = workflow.ExecuteActivity(ctx, activities.MarkServiceActive, service.ID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to mark service as active")
		return err
	}

	logger.Info().Msg("updating order status to ready")
	err = workflow.ExecuteActivity(ctx, activities.UpdateOrderStatus, orderID, OrderStatusReady, "provisioning complete").Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to update order status to ready")
		return err
	}

	logger.Info().Msg("completing workflow")
	return nil
}

// TenantTeardownWorkflow reverses provisioning by tearing down resources
// in reverse order: delete workload, delete allocation, delete VPC,
// then mark the service as terminated.
func TenantTeardownWorkflow(ctx workflow.Context, serviceID uuid.UUID) error {
	logger := log.With().Str("Workflow", "TenantTeardown").
		Str("ServiceID", serviceID.String()).Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var activities FulfillmentActivities

	// Reverse order: delete workload -> delete allocation -> delete VPC -> mark terminated
	logger.Info().Msg("placeholder: workload deletion would happen here")
	logger.Info().Msg("placeholder: compute deallocation would happen here")
	logger.Info().Msg("placeholder: VPC deletion would happen here")

	// Mark service as terminated
	logger.Info().Msg("marking service as terminated")
	err := workflow.ExecuteActivity(ctx, activities.MarkServiceTerminated, serviceID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to mark service as terminated")
		return err
	}

	logger.Info().Msg("completing workflow")
	return nil
}

// ServiceScaleWorkflow modifies a running service by updating its
// resources based on the provided parameters.
func ServiceScaleWorkflow(ctx workflow.Context, serviceID uuid.UUID, params map[string]interface{}) error {
	logger := log.With().Str("Workflow", "ServiceScale").
		Str("ServiceID", serviceID.String()).Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Placeholder: scaling activities would be executed here based on params
	logger.Info().Interface("params", params).Msg("placeholder: service scaling would happen here")

	logger.Info().Msg("completing workflow")
	return nil
}
