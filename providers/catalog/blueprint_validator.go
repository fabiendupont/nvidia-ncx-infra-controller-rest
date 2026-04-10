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

package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

var exprPattern = regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

// ValidationResult holds blueprint validation results.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ValidateBlueprint checks a blueprint for structural correctness:
// name/version present, valid resource types, valid dependency refs,
// no cycles, valid expression syntax, and tenant constraints.
func ValidateBlueprint(b *Blueprint) ValidationResult {
	var errs []string

	if b.Name == "" {
		errs = append(errs, "name is required")
	}
	if b.Version == "" {
		errs = append(errs, "version is required")
	}

	// Validate resource types and dependencies
	for name, res := range b.Resources {
		if !isValidResourceType(res.Type) {
			errs = append(errs, fmt.Sprintf("resource %q: unknown type %q", name, res.Type))
		}

		// Tenant-owned blueprints can only reference blueprint/* types, not nico/*
		if b.IsTenantOwned() && !strings.HasPrefix(res.Type, "blueprint/") {
			errs = append(errs, fmt.Sprintf("resource %q: tenant blueprints can only reference blueprint/* types, not %q", name, res.Type))
		}

		for _, dep := range res.DependsOn {
			if _, ok := b.Resources[dep]; !ok {
				errs = append(errs, fmt.Sprintf("resource %q: depends_on %q does not exist", name, dep))
			}
		}
		// Validate expressions reference defined parameters or resources
		for _, expr := range extractExpressions(res.Properties) {
			if err := validateExpression(expr, b.Parameters, b.Resources); err != nil {
				errs = append(errs, fmt.Sprintf("resource %q: %v", name, err))
			}
		}
		if res.Condition != "" {
			if err := validateExpression(res.Condition, b.Parameters, b.Resources); err != nil {
				errs = append(errs, fmt.Sprintf("resource %q condition: %v", name, err))
			}
		}
		if res.Count != "" {
			if err := validateExpression(res.Count, b.Parameters, b.Resources); err != nil {
				errs = append(errs, fmt.Sprintf("resource %q count: %v", name, err))
			}
		}
	}

	// Check total resource count
	if len(b.Resources) > MaxResourceCount {
		errs = append(errs, fmt.Sprintf("blueprint has %d resources, maximum is %d", len(b.Resources), MaxResourceCount))
	}

	// Check for cycles
	if err := detectCycles(b.Resources); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate pricing if present
	if b.Pricing != nil {
		if b.Pricing.Unit != "hour" && b.Pricing.Unit != "month" && b.Pricing.Unit != "one-time" {
			errs = append(errs, fmt.Sprintf("pricing unit must be hour, month, or one-time, got %q", b.Pricing.Unit))
		}
	}

	return ValidationResult{Valid: len(errs) == 0, Errors: errs}
}

func isValidResourceType(t string) bool {
	if strings.HasPrefix(t, "blueprint/") {
		return true
	}
	for _, rt := range AvailableResourceTypes {
		if rt == t {
			return true
		}
	}
	return false
}

func extractExpressions(props map[string]interface{}) []string {
	var exprs []string
	for _, v := range props {
		switch val := v.(type) {
		case string:
			for _, match := range exprPattern.FindAllStringSubmatch(val, -1) {
				exprs = append(exprs, match[1])
			}
		}
	}
	return exprs
}

func validateExpression(expr string, params map[string]BlueprintParameter, resources map[string]BlueprintResource) error {
	// Strip {{ }} if present
	expr = strings.TrimSpace(expr)
	expr = strings.TrimPrefix(expr, "{{")
	expr = strings.TrimSuffix(expr, "}}")
	expr = strings.TrimSpace(expr)

	if expr == "" {
		return fmt.Errorf("empty expression")
	}

	// Extract the first identifier (before any . or operator)
	parts := strings.FieldsFunc(expr, func(r rune) bool {
		return r == '.' || r == ' ' || r == '>' || r == '<' || r == '/' || r == '*' || r == '[' || r == '+'
	})
	if len(parts) == 0 {
		return nil
	}

	ref := parts[0]
	if _, ok := params[ref]; ok {
		return nil
	}
	if _, ok := resources[ref]; ok {
		return nil
	}
	// Allow numeric literals
	for _, c := range ref {
		if c < '0' || c > '9' {
			return fmt.Errorf("expression references undefined identifier %q", ref)
		}
	}
	return nil
}

func detectCycles(resources map[string]BlueprintResource) error {
	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done

	var visit func(name string) error
	visit = func(name string) error {
		switch visited[name] {
		case 2:
			return nil
		case 1:
			return fmt.Errorf("circular dependency detected involving %q", name)
		}
		visited[name] = 1
		res, ok := resources[name]
		if !ok {
			return nil
		}
		for _, dep := range res.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visited[name] = 2
		return nil
	}

	for name := range resources {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

// topologicalSort returns resources in dependency order, grouped into
// layers that can execute in parallel. Used by the validator to verify
// the DAG structure and by external callers for execution planning.
func topologicalSort(resources map[string]BlueprintResource) ([][]string, error) {
	if err := detectCycles(resources); err != nil {
		return nil, err
	}

	remaining := make(map[string]BlueprintResource)
	for k, v := range resources {
		remaining[k] = v
	}

	resolved := make(map[string]bool)
	var layers [][]string

	for len(remaining) > 0 {
		var layer []string
		for name, res := range remaining {
			allResolved := true
			for _, dep := range res.DependsOn {
				if !resolved[dep] {
					allResolved = false
					break
				}
			}
			if allResolved {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("unresolvable dependencies remain")
		}
		for _, name := range layer {
			resolved[name] = true
			delete(remaining, name)
		}
		layers = append(layers, layer)
	}
	return layers, nil
}
