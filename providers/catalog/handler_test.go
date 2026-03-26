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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTemplate(t *testing.T, store *TemplateStore, name string) *ServiceTemplate {
	t.Helper()
	e := echo.New()
	body := `{"name":"` + name + `","version":"1.0.0","description":"a service"}`
	req := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := handleCreateTemplate(store)
	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, rec.Code)

	var tmpl ServiceTemplate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tmpl))
	return &tmpl
}

func TestHandleCreateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewTemplateStore()
		e := echo.New()
		body := `{"name":"gpu-cluster","version":"2.0.0","description":"GPU cluster template"}`
		req := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handleCreateTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)

		var tmpl ServiceTemplate
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tmpl))
		assert.Equal(t, "gpu-cluster", tmpl.Name)
		assert.Equal(t, "2.0.0", tmpl.Version)
		assert.True(t, tmpl.IsActive)
		assert.NotEqual(t, uuid.Nil, tmpl.ID)
	})

	t.Run("missing name returns 400", func(t *testing.T) {
		store := NewTemplateStore()
		e := echo.New()
		body := `{"version":"1.0.0"}`
		req := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handleCreateTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Name is required")
	})

	t.Run("missing version returns 400", func(t *testing.T) {
		store := NewTemplateStore()
		e := echo.New()
		body := `{"name":"gpu-cluster"}`
		req := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handleCreateTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Version is required")
	})
}

func TestHandleListTemplates(t *testing.T) {
	t.Run("returns active templates only", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := seedTemplate(t, store, "active-svc")

		// Add an inactive template directly.
		inactive := newTestTemplate("inactive-svc")
		inactive.IsActive = false
		require.NoError(t, store.Create(inactive))

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/templates", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handleListTemplates(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var list []ServiceTemplate
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
		assert.Len(t, list, 1)
		assert.Equal(t, tmpl.ID, list[0].ID)
	})
}

func TestHandleGetTemplate(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := seedTemplate(t, store, "my-service")

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/templates/"+tmpl.ID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(tmpl.ID.String())

		err := handleGetTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var got ServiceTemplate
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, tmpl.ID, got.ID)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := NewTemplateStore()
		missing := uuid.New()

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/templates/"+missing.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(missing.String())

		err := handleGetTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid uuid returns 400", func(t *testing.T) {
		store := NewTemplateStore()

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/templates/not-a-uuid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("not-a-uuid")

		err := handleGetTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid template ID format")
	})
}

func TestHandleUpdateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := seedTemplate(t, store, "old-name")

		e := echo.New()
		body := `{"name":"new-name"}`
		req := httptest.NewRequest(http.MethodPatch, "/templates/"+tmpl.ID.String(), strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(tmpl.ID.String())

		err := handleUpdateTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var updated ServiceTemplate
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
		assert.Equal(t, "new-name", updated.Name)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := NewTemplateStore()
		missing := uuid.New()

		e := echo.New()
		body := `{"name":"new-name"}`
		req := httptest.NewRequest(http.MethodPatch, "/templates/"+missing.String(), strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(missing.String())

		err := handleUpdateTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleDeleteTemplate(t *testing.T) {
	t.Run("success returns 204", func(t *testing.T) {
		store := NewTemplateStore()
		tmpl := seedTemplate(t, store, "to-delete")

		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/templates/"+tmpl.ID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(tmpl.ID.String())

		err := handleDeleteTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the template is soft-deleted (IsActive=false).
		got, err := store.GetByID(tmpl.ID)
		require.NoError(t, err)
		assert.False(t, got.IsActive)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := NewTemplateStore()
		missing := uuid.New()

		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/templates/"+missing.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(missing.String())

		err := handleDeleteTemplate(store)(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
