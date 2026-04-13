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

package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func testUser(org string, roles ...string) *cdbm.User {
	return &cdbm.User{
		OrgData: cdbm.OrgData{
			org: {
				Name:  org,
				Roles: roles,
			},
		},
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", testUser("myorg", RoleProviderAdmin))
	c.Set("orgName", "myorg")

	handler := withTestRole(RoleProviderAdmin)
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_Denied(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", testUser("myorg", RoleTenantAdmin))
	c.Set("orgName", "myorg")

	handler := withTestRole(RoleProviderAdmin)
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_NoUser_PassThrough(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No user set — dev/test mode

	handler := withTestRole(RoleProviderAdmin)
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_WrongOrg(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", testUser("other-org", RoleProviderAdmin))
	c.Set("orgName", "myorg")

	handler := withTestRole(RoleProviderAdmin)
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireAuth_Allowed(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", testUser("myorg"))
	c.Set("orgName", "myorg")

	mw := RequireAuth()
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAuth_NotMember(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", testUser("other-org"))
	c.Set("orgName", "myorg")

	mw := RequireAuth()
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_MultipleRoles(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", testUser("myorg", RoleBlueprintAuthor))
	c.Set("orgName", "myorg")

	// User has BLUEPRINT_AUTHOR, allowed if either PROVIDER_ADMIN or BLUEPRINT_AUTHOR
	handler := withTestRole(RoleProviderAdmin, RoleBlueprintAuthor)
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func withTestRole(roles ...string) echo.HandlerFunc {
	mw := RequireRole(roles...)
	return mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
}
