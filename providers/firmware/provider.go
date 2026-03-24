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

package firmware

import (
	"context"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

// FirmwareProvider implements the firmware feature provider.
type FirmwareProvider struct {
	dbSession *cdb.Session
}

// New creates a new FirmwareProvider.
func New() *FirmwareProvider {
	return &FirmwareProvider{}
}

func (p *FirmwareProvider) Name() string           { return "nico-firmware" }
func (p *FirmwareProvider) Version() string        { return "1.0.6" }
func (p *FirmwareProvider) Features() []string     { return []string{"firmware"} }
func (p *FirmwareProvider) Dependencies() []string { return []string{"nico-site"} }

func (p *FirmwareProvider) Init(ctx provider.ProviderContext) error {
	p.dbSession = ctx.DB
	return nil
}

func (p *FirmwareProvider) Shutdown(_ context.Context) error {
	return nil
}
