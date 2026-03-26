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

package dpfhcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validRequestBody = `{
	"dpuClusterRef": {"name": "cluster-1", "namespace": "ns-1"},
	"baseDomain": "example.com",
	"ocpReleaseImage": "quay.io/ocp:4.14",
	"sshKeySecretRef": "ssh-secret",
	"pullSecretRef": "pull-secret"
}`

func setupEchoContext(method, path, body string, siteID string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if siteID != "" {
		c.SetParamNames("siteId")
		c.SetParamValues(siteID)
	}
	return c, rec
}

func TestHandleProvision(t *testing.T) {
	t.Run("success 201", func(t *testing.T) {
		store := NewProvisioningStore()
		handler := handleProvision(store)

		c, rec := setupEchoContext(http.MethodPost, "/sites/site-1/dpf-hcp", validRequestBody, "site-1")

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)

		var record ProvisioningRecord
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &record))
		assert.Equal(t, "site-1", record.SiteID)
		assert.Equal(t, StatusPending, record.Status)
		assert.Equal(t, "cluster-1", record.Config.DPUClusterRef.Name)
	})

	t.Run("duplicate 409", func(t *testing.T) {
		store := NewProvisioningStore()
		handler := handleProvision(store)

		// First request succeeds
		c, _ := setupEchoContext(http.MethodPost, "/sites/site-1/dpf-hcp", validRequestBody, "site-1")
		require.NoError(t, handler(c))

		// Second request should return 409
		c, rec := setupEchoContext(http.MethodPost, "/sites/site-1/dpf-hcp", validRequestBody, "site-1")
		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, rec.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "conflict", body["error"])
	})

	t.Run("missing fields 400", func(t *testing.T) {
		store := NewProvisioningStore()
		handler := handleProvision(store)

		// Missing baseDomain
		incompleteBody := `{
			"dpuClusterRef": {"name": "cluster-1", "namespace": "ns-1"},
			"ocpReleaseImage": "quay.io/ocp:4.14",
			"sshKeySecretRef": "ssh-secret",
			"pullSecretRef": "pull-secret"
		}`

		c, rec := setupEchoContext(http.MethodPost, "/sites/site-1/dpf-hcp", incompleteBody, "site-1")
		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "validation_error", body["error"])
		assert.Contains(t, body["message"], "baseDomain")
	})

	t.Run("missing siteId 400", func(t *testing.T) {
		store := NewProvisioningStore()
		handler := handleProvision(store)

		c, rec := setupEchoContext(http.MethodPost, "/sites//dpf-hcp", validRequestBody, "")
		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestHandleGetStatus(t *testing.T) {
	t.Run("found 200", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-1")
		require.NoError(t, store.Create(record))

		handler := handleGetStatus(store)
		c, rec := setupEchoContext(http.MethodGet, "/sites/site-1/dpf-hcp", "", "site-1")

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var got ProvisioningRecord
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, "site-1", got.SiteID)
	})

	t.Run("not found 404", func(t *testing.T) {
		store := NewProvisioningStore()
		handler := handleGetStatus(store)

		c, rec := setupEchoContext(http.MethodGet, "/sites/missing/dpf-hcp", "", "missing")

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "not_found", body["error"])
	})
}

func TestHandleDelete(t *testing.T) {
	t.Run("success 202", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-1")
		require.NoError(t, store.Create(record))

		handler := handleDelete(store)
		c, rec := setupEchoContext(http.MethodDelete, "/sites/site-1/dpf-hcp", "", "site-1")

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, rec.Code)

		// Verify the record status is now Deleting
		got, err := store.GetBySiteID("site-1")
		require.NoError(t, err)
		assert.Equal(t, StatusDeleting, got.Status)
	})

	t.Run("not found 404", func(t *testing.T) {
		store := NewProvisioningStore()
		handler := handleDelete(store)

		c, rec := setupEchoContext(http.MethodDelete, "/sites/missing/dpf-hcp", "", "missing")

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "not_found", body["error"])
	})
}
