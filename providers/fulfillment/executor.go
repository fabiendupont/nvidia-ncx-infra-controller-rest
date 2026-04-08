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

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BlueprintExecutionWorkflow is a Temporal workflow that executes a
// compiled DAG from a blueprint. It processes resources layer by layer,
// executing independent resources in parallel within each layer.
func BlueprintExecutionWorkflow(ctx workflow.Context, dag DAG, orderID string, params map[string]interface{}) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting blueprint execution", "orderID", orderID, "layers", len(dag.Order))

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    10,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Track created resources for rollback on failure
	var createdResources []ResourceRef
	// Track outputs for expression resolution in dependent resources
	outputs := make(map[string]map[string]interface{})

	for layerIdx, layer := range dag.Order {
		logger.Info("Executing DAG layer", "layer", layerIdx, "resources", layer)

		// Execute all resources in this layer in parallel
		var futures []workflow.Future
		var futureNames []string

		for _, name := range layer {
			node := dag.Nodes[name]

			// Evaluate condition at runtime
			if node.Condition != "" && !EvaluateCondition(node.Condition, params) {
				logger.Info("Skipping resource (condition false)", "resource", name)
				continue
			}

			for i := 0; i < node.Count; i++ {
				resourceName := name
				if node.Count > 1 {
					resourceName = fmt.Sprintf("%s-%d", name, i)
				}

				input := CreateResourceInput{
					Name:       resourceName,
					Type:       node.Type,
					Properties: node.Properties,
					Outputs:    outputs,
				}

				future := workflow.ExecuteActivity(ctx, "CreateResource", input)
				futures = append(futures, future)
				futureNames = append(futureNames, resourceName)
			}
		}

		// Wait for all resources in this layer to complete
		for i, future := range futures {
			var result CreateResourceOutput
			if err := future.Get(ctx, &result); err != nil {
				logger.Error("Resource creation failed, starting rollback",
					"resource", futureNames[i], "error", err)

				// Rollback created resources in reverse order
				rollbackErr := rollback(ctx, createdResources)
				if rollbackErr != nil {
					logger.Error("Rollback failed", "error", rollbackErr)
				}
				return fmt.Errorf("resource %s failed: %w", futureNames[i], err)
			}

			outputs[futureNames[i]] = result.Outputs
			createdResources = append(createdResources, ResourceRef{
				Name: futureNames[i],
				Type: result.Type,
				ID:   result.ResourceID,
			})
		}
	}

	logger.Info("Blueprint execution completed", "orderID", orderID,
		"resources_created", len(createdResources))
	return nil
}

func rollback(ctx workflow.Context, resources []ResourceRef) error {
	logger := workflow.GetLogger(ctx)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: 2 * time.Second,
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Delete in reverse order
	for i := len(resources) - 1; i >= 0; i-- {
		ref := resources[i]
		logger.Info("Rolling back resource", "resource", ref.Name, "type", ref.Type)
		err := workflow.ExecuteActivity(ctx, "DeleteResource", ref).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to rollback resource", "resource", ref.Name, "error", err)
		}
	}
	return nil
}

// CreateResourceInput is the input to the CreateResource activity.
type CreateResourceInput struct {
	Name       string
	Type       string
	Properties map[string]interface{}
	Outputs    map[string]map[string]interface{} // outputs from previous resources
}

// CreateResourceOutput is the output from the CreateResource activity.
type CreateResourceOutput struct {
	ResourceID string
	Type       string
	Outputs    map[string]interface{} // e.g., {"id": "uuid", "ip": "10.0.1.5"}
}

// ResourceRef tracks a created resource for rollback.
type ResourceRef struct {
	Name string
	Type string
	ID   string
}

// ExecutionActivities handles individual resource CRUD for the DAG executor.
type ExecutionActivities struct {
	orderStore   *OrderStore
	serviceStore *ServiceStore
}

// NewExecutionActivities creates execution activities.
func NewExecutionActivities(orderStore *OrderStore, serviceStore *ServiceStore) *ExecutionActivities {
	return &ExecutionActivities{
		orderStore:   orderStore,
		serviceStore: serviceStore,
	}
}

// CreateResource creates a single NICo resource based on its type.
// In production, this would call the appropriate service interface
// (networkingsvc for VPCs, computesvc for instances, etc.).
func (a *ExecutionActivities) CreateResource(ctx context.Context, input CreateResourceInput) (*CreateResourceOutput, error) {
	logger := log.With().Str("activity", "CreateResource").Str("resource", input.Name).Str("type", input.Type).Logger()
	logger.Info().Msg("creating resource")

	// Placeholder: log the resource creation
	// In production, dispatch to the appropriate provider's service interface
	resourceID := fmt.Sprintf("res-%s-%d", input.Name, time.Now().UnixMilli())

	logger.Info().Str("resource_id", resourceID).Msg("resource created")

	return &CreateResourceOutput{
		ResourceID: resourceID,
		Type:       input.Type,
		Outputs:    map[string]interface{}{"id": resourceID},
	}, nil
}

// DeleteResource deletes a NICo resource (for rollback).
func (a *ExecutionActivities) DeleteResource(ctx context.Context, ref ResourceRef) error {
	logger := log.With().Str("activity", "DeleteResource").Str("resource", ref.Name).Str("type", ref.Type).Logger()
	logger.Info().Str("resource_id", ref.ID).Msg("deleting resource (rollback)")
	return nil
}
