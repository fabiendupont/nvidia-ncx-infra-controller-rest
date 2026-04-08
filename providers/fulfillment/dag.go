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
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var exprPattern = regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

// BlueprintResource mirrors catalog.BlueprintResource for DAG compilation.
type BlueprintResource struct {
	Type       string
	DependsOn  []string
	Condition  string
	Count      string
	Properties map[string]interface{}
}

// DAGNode represents a resource in the execution graph.
type DAGNode struct {
	Name       string
	Type       string
	DependsOn  []string
	Condition  string
	Count      int
	Properties map[string]interface{}
}

// DAG represents the compiled execution graph.
type DAG struct {
	Nodes map[string]*DAGNode
	Order [][]string // topologically sorted, grouped for parallel execution
}

// CompileDAG takes a blueprint's resources and parameters, resolves
// expressions, evaluates counts, and returns a DAG ready for execution.
func CompileDAG(resources map[string]BlueprintResource, params map[string]interface{}) (*DAG, error) {
	nodes := make(map[string]*DAGNode, len(resources))

	for name, res := range resources {
		count := 1
		if res.Count != "" {
			resolved, err := resolveExprInt(res.Count, params)
			if err != nil {
				return nil, fmt.Errorf("resource %q: invalid count expression: %w", name, err)
			}
			count = resolved
		}

		props := make(map[string]interface{}, len(res.Properties))
		for k, v := range res.Properties {
			props[k] = resolveExprValue(v, params)
		}

		nodes[name] = &DAGNode{
			Name:       name,
			Type:       res.Type,
			DependsOn:  res.DependsOn,
			Condition:  res.Condition,
			Count:      count,
			Properties: props,
		}
	}

	// Topological sort into parallel layers
	order, err := topoSort(nodes)
	if err != nil {
		return nil, err
	}

	return &DAG{Nodes: nodes, Order: order}, nil
}

func topoSort(nodes map[string]*DAGNode) ([][]string, error) {
	remaining := make(map[string]*DAGNode, len(nodes))
	for k, v := range nodes {
		remaining[k] = v
	}

	resolved := make(map[string]bool)
	var layers [][]string

	for len(remaining) > 0 {
		var layer []string
		for name, node := range remaining {
			ready := true
			for _, dep := range node.DependsOn {
				if !resolved[dep] {
					ready = false
					break
				}
			}
			if ready {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("circular dependency detected in DAG")
		}
		for _, name := range layer {
			resolved[name] = true
			delete(remaining, name)
		}
		layers = append(layers, layer)
	}
	return layers, nil
}

func resolveExprValue(v interface{}, params map[string]interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	return exprPattern.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := params[inner]; ok {
			return fmt.Sprintf("%v", val)
		}
		// Leave resource references as placeholders for runtime resolution
		return match
	})
}

func resolveExprInt(expr string, params map[string]interface{}) (int, error) {
	expr = strings.TrimSpace(expr)
	inner := exprPattern.FindStringSubmatch(expr)
	if inner == nil {
		return strconv.Atoi(expr)
	}

	parts := strings.Fields(inner[1])
	if len(parts) == 1 {
		if val, ok := params[parts[0]]; ok {
			switch v := val.(type) {
			case int:
				return v, nil
			case float64:
				return int(v), nil
			}
		}
		return strconv.Atoi(parts[0])
	}

	// Simple binary expression: "param / value" or "param * value"
	if len(parts) == 3 {
		left, err := resolveOperand(parts[0], params)
		if err != nil {
			return 0, err
		}
		right, err := resolveOperand(parts[2], params)
		if err != nil {
			return 0, err
		}
		switch parts[1] {
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		case "*":
			return left * right, nil
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		}
	}

	return 0, fmt.Errorf("cannot resolve expression %q", expr)
}

func resolveOperand(s string, params map[string]interface{}) (int, error) {
	if val, ok := params[s]; ok {
		switch v := val.(type) {
		case int:
			return v, nil
		case float64:
			return int(v), nil
		}
	}
	return strconv.Atoi(s)
}

// EvaluateCondition evaluates a simple boolean condition expression.
func EvaluateCondition(condition string, params map[string]interface{}) bool {
	if condition == "" {
		return true
	}

	inner := exprPattern.FindStringSubmatch(condition)
	if inner == nil {
		return condition == "true"
	}

	parts := strings.Fields(inner[1])
	if len(parts) == 3 {
		left, err := resolveOperand(parts[0], params)
		if err != nil {
			return true // default to true if we can't evaluate
		}
		right, err := resolveOperand(parts[2], params)
		if err != nil {
			return true
		}
		switch parts[1] {
		case ">":
			return left > right
		case "<":
			return left < right
		case ">=":
			return left >= right
		case "<=":
			return left <= right
		case "==":
			return left == right
		case "!=":
			return left != right
		}
	}

	return true
}
