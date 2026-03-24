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
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/config"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/stretchr/testify/assert"

	temporalClient "go.temporal.io/sdk/client"
	tmocks "go.temporal.io/sdk/mocks"
)

func TestNewAPIRoutes(t *testing.T) {
	type args struct {
		dbSession *cdb.Session
		tc        temporalClient.Client
		tnc       temporalClient.NamespaceClient
		cfg       *config.Config
	}

	tc := &tmocks.Client{}
	tnc := &tmocks.NamespaceClient{}

	cfg := common.GetTestConfig()

	// Core routes only (domain routes are now registered by providers)
	routeCount := map[string]int{
		"metadata":                1,
		"user":                    1,
		"service-account":         1,
		"infrastructure-provider": 4,
		"tenant":                  4,
		"tenant-account":          5,
		"stats":                   1,
		"audit":                   2,
	}

	totalRouteCount := 0
	for _, v := range routeCount {
		totalRouteCount += v
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "test initializing API routes",
			args: args{
				dbSession: &cdb.Session{},
				tc:        tc,
				tnc:       tnc,
				cfg:       cfg,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAPIRoutes(tt.args.dbSession, tt.args.tc, tt.args.tnc, tt.args.cfg)

			assert.Equal(t, totalRouteCount, len(got))

			for _, route := range got {
				assert.Contains(t, route.Path, "/org/:orgName/"+cfg.GetAPIName())
			}
		})
	}
}
