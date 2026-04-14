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

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard"
)

// FeatureStatus describes the runtime status of a single feature.
type FeatureStatus = standard.FeatureStatus

// GetCapabilities returns the status of all known features.
// Each feature reports whether it is available, which provider implements it,
// and the provider version. Use this to check whether a feature like
// "fault-management" is available before calling its API.
func (c *Client) GetCapabilities(ctx context.Context) (map[string]FeatureStatus, *ApiError) {
	ctx = WithLogger(ctx, c.Logger)
	ctx = context.WithValue(ctx, standard.ContextAccessToken, c.Config.Token)

	logger := LoggerFromContext(ctx)
	logger.Info().Msgf("Getting Capabilities for org: %s", c.Config.Org)

	result, resp, err := c.apiClient.MetadataAPI.GetCapabilities(ctx, c.apiMetadata.Organization).Execute()
	apiErr := HandleResponseError(resp, err)
	if apiErr != nil {
		return nil, apiErr
	}
	return result.GetFeatures(), nil
}
