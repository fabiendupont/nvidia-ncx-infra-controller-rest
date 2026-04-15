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

package api

import (
	"net/http"

	tClient "go.temporal.io/sdk/client"

	apiHandler "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/config"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
)

// NewAPIRoutes returns core API routes (identity, tenancy, audit).
// Domain-specific routes (networking, compute, site, health) are
// registered by their respective providers via RegisterRoutes.
func NewAPIRoutes(dbSession *cdb.Session, tc tClient.Client, tnc tClient.NamespaceClient, cfg *config.Config) []Route {
	apiName := cfg.GetAPIName()

	apiPathPrefix := "/org/:orgName/" + apiName

	apiRoutes := []Route{
		// Metadata endpoint
		{
			Path:    apiPathPrefix + "/metadata",
			Method:  http.MethodGet,
			Handler: apiHandler.NewMetadataHandler(),
		},
		// User endpoint
		{
			Path:    apiPathPrefix + "/user/current",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetUserHandler(dbSession),
		},
		// Service Account endpoint
		{
			Path:    apiPathPrefix + "/service-account/current",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetCurrentServiceAccountHandler(dbSession, cfg),
		},
		// Infrastructure Provider endpoints
		{
			Path:    apiPathPrefix + "/infrastructure-provider",
			Method:  http.MethodPost,
			Handler: apiHandler.NewCreateInfrastructureProviderHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/infrastructure-provider/current",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetCurrentInfrastructureProviderHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/infrastructure-provider/current",
			Method:  http.MethodPatch,
			Handler: apiHandler.NewUpdateCurrentInfrastructureProviderHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/infrastructure-provider/current/stats",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetCurrentInfrastructureProviderStatsHandler(dbSession, tc, cfg),
		},
		// Tenant endpoints
		{
			Path:    apiPathPrefix + "/tenant",
			Method:  http.MethodPost,
			Handler: apiHandler.NewCreateTenantHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/current",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetCurrentTenantHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/current",
			Method:  http.MethodPatch,
			Handler: apiHandler.NewUpdateCurrentTenantHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/current/stats",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetCurrentTenantStatsHandler(dbSession, tc, cfg),
		},
		// Tenant Instance Type Stats endpoint
		{
			Path:    apiPathPrefix + "/tenant/instance-type/stats",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetTenantInstanceTypeStatsHandler(dbSession, cfg),
		},
		// TenantAccount endpoints
		{
			Path:    apiPathPrefix + "/tenant/account",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetAllTenantAccountHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/account/:id",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetTenantAccountHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/account",
			Method:  http.MethodPost,
			Handler: apiHandler.NewCreateTenantAccountHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/account/:id",
			Method:  http.MethodPatch,
			Handler: apiHandler.NewUpdateTenantAccountHandler(dbSession, tc, cfg),
		},
		{
			Path:    apiPathPrefix + "/tenant/account/:id",
			Method:  http.MethodDelete,
			Handler: apiHandler.NewDeleteTenantAccountHandler(dbSession, tc, cfg),
		},
		// Audit Log endpoints
		{
			Path:    apiPathPrefix + "/audit",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetAllAuditEntryHandler(dbSession),
		},
		{
			Path:    apiPathPrefix + "/audit/:id",
			Method:  http.MethodGet,
			Handler: apiHandler.NewGetAuditEntryHandler(dbSession),
		},
	}

	return apiRoutes
}
