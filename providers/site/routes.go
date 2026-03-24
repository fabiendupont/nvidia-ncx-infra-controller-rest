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

package site

import (
	"net/http"

	echo "github.com/labstack/echo/v4"

	apiHandler "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler"
)

// RegisterRoutes registers all site-related API routes on the given Echo group.
func (p *SiteProvider) RegisterRoutes(group *echo.Group) {
	prefix := p.apiPathPrefix

	// Site endpoints
	group.Add(http.MethodPost, prefix+"/site", apiHandler.NewCreateSiteHandler(p.dbSession, p.tc, p.tnc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/site", apiHandler.NewGetAllSiteHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/site/:id", apiHandler.NewGetSiteHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/site/:id", apiHandler.NewUpdateSiteHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/site/:id", apiHandler.NewDeleteSiteHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/site/:id/status-history", apiHandler.NewGetSiteStatusDetailsHandler(p.dbSession).Handle)

	// ExpectedMachine endpoints
	group.Add(http.MethodPost, prefix+"/expected-machine", apiHandler.NewCreateExpectedMachineHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/expected-machine", apiHandler.NewGetAllExpectedMachineHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/expected-machine/:id", apiHandler.NewGetExpectedMachineHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/expected-machine/:id", apiHandler.NewUpdateExpectedMachineHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/expected-machine/:id", apiHandler.NewDeleteExpectedMachineHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// ExpectedPowerShelf endpoints
	group.Add(http.MethodPost, prefix+"/expected-power-shelf", apiHandler.NewCreateExpectedPowerShelfHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/expected-power-shelf", apiHandler.NewGetAllExpectedPowerShelfHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/expected-power-shelf/:id", apiHandler.NewGetExpectedPowerShelfHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/expected-power-shelf/:id", apiHandler.NewUpdateExpectedPowerShelfHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/expected-power-shelf/:id", apiHandler.NewDeleteExpectedPowerShelfHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)

	// ExpectedSwitch endpoints
	group.Add(http.MethodPost, prefix+"/expected-switch", apiHandler.NewCreateExpectedSwitchHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/expected-switch", apiHandler.NewGetAllExpectedSwitchHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodGet, prefix+"/expected-switch/:id", apiHandler.NewGetExpectedSwitchHandler(p.dbSession, p.tc, p.cfg).Handle)
	group.Add(http.MethodPatch, prefix+"/expected-switch/:id", apiHandler.NewUpdateExpectedSwitchHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
	group.Add(http.MethodDelete, prefix+"/expected-switch/:id", apiHandler.NewDeleteExpectedSwitchHandler(p.dbSession, p.tc, p.scp, p.cfg).Handle)
}
