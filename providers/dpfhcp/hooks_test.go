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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreCreateInstanceCheck_NoDPFRecord(t *testing.T) {
	store := NewProvisioningStore()
	p := &DPFHCPProvider{store: store}

	payload := map[string]interface{}{"site_id": "site-1"}
	err := p.preCreateInstanceCheck(context.Background(), payload)
	require.NoError(t, err)
}

func TestPreCreateInstanceCheck_DPFReady(t *testing.T) {
	store := NewProvisioningStore()
	record := newTestRecord("site-1")
	record.Status = StatusReady
	require.NoError(t, store.Create(record))

	p := &DPFHCPProvider{store: store}

	payload := map[string]interface{}{"site_id": "site-1"}
	err := p.preCreateInstanceCheck(context.Background(), payload)
	require.NoError(t, err)
}

func TestPreCreateInstanceCheck_DPFProvisioning(t *testing.T) {
	store := NewProvisioningStore()
	record := newTestRecord("site-1")
	record.Status = StatusProvisioning
	require.NoError(t, store.Create(record))

	p := &DPFHCPProvider{store: store}

	payload := map[string]interface{}{"site_id": "site-1"}
	err := p.preCreateInstanceCheck(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")
	assert.Contains(t, err.Error(), "Provisioning")
}

func TestPreCreateInstanceCheck_NilPayload(t *testing.T) {
	store := NewProvisioningStore()
	p := &DPFHCPProvider{store: store}

	err := p.preCreateInstanceCheck(context.Background(), nil)
	require.NoError(t, err)
}

func TestExtractSiteID(t *testing.T) {
	t.Run("site_id key", func(t *testing.T) {
		payload := map[string]interface{}{"site_id": "abc-123"}
		id, ok := extractSiteID(payload)
		assert.True(t, ok)
		assert.Equal(t, "abc-123", id)
	})

	t.Run("siteID key", func(t *testing.T) {
		payload := map[string]interface{}{"siteID": "def-456"}
		id, ok := extractSiteID(payload)
		assert.True(t, ok)
		assert.Equal(t, "def-456", id)
	})

	t.Run("nil payload", func(t *testing.T) {
		id, ok := extractSiteID(nil)
		assert.False(t, ok)
		assert.Equal(t, "", id)
	})

	t.Run("missing key", func(t *testing.T) {
		payload := map[string]interface{}{"other": "value"}
		id, ok := extractSiteID(payload)
		assert.False(t, ok)
		assert.Equal(t, "", id)
	})

	t.Run("non-string value", func(t *testing.T) {
		payload := map[string]interface{}{"site_id": 12345}
		id, ok := extractSiteID(payload)
		assert.False(t, ok)
		assert.Equal(t, "", id)
	})
}
