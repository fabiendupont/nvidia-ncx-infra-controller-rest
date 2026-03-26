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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRecord(siteID string) *ProvisioningRecord {
	now := time.Now().UTC()
	return &ProvisioningRecord{
		SiteID: siteID,
		Config: DPFHCPRequest{
			DPUClusterRef:   DPUClusterReference{Name: "cluster-1", Namespace: "ns-1"},
			BaseDomain:      "example.com",
			OCPReleaseImage: "quay.io/ocp:4.14",
			SSHKeySecretRef: "ssh-secret",
			PullSecretRef:   "pull-secret",
		},
		Status:  StatusPending,
		Created: now,
		Updated: now,
	}
}

func TestProvisioningStore_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-1")

		err := store.Create(record)
		require.NoError(t, err)

		got, err := store.Get("site-1")
		require.NoError(t, err)
		assert.Equal(t, "site-1", got.SiteID)
		assert.Equal(t, StatusPending, got.Status)
	})

	t.Run("duplicate site error", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-1")

		err := store.Create(record)
		require.NoError(t, err)

		err = store.Create(newTestRecord("site-1"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestProvisioningStore_GetBySiteID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-2")
		require.NoError(t, store.Create(record))

		got, err := store.GetBySiteID("site-2")
		require.NoError(t, err)
		assert.Equal(t, "site-2", got.SiteID)
	})

	t.Run("not found", func(t *testing.T) {
		store := NewProvisioningStore()

		got, err := store.GetBySiteID("nonexistent")
		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProvisioningStore_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-3")
		require.NoError(t, store.Create(record))

		record.Status = StatusReady
		record.Updated = time.Now().UTC()
		err := store.Update(record)
		require.NoError(t, err)

		got, err := store.Get("site-3")
		require.NoError(t, err)
		assert.Equal(t, StatusReady, got.Status)
	})

	t.Run("not found", func(t *testing.T) {
		store := NewProvisioningStore()

		record := newTestRecord("missing")
		err := store.Update(record)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProvisioningStore_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewProvisioningStore()
		record := newTestRecord("site-4")
		require.NoError(t, store.Create(record))

		err := store.Delete("site-4")
		require.NoError(t, err)

		got, err := store.Get("site-4")
		require.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("not found", func(t *testing.T) {
		store := NewProvisioningStore()

		err := store.Delete("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
