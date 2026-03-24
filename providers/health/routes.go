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

package health

import (
	echo "github.com/labstack/echo/v4"
)

// RegisterRoutes satisfies the provider.APIProvider interface.
// The health provider currently exposes no versioned API routes; the
// /healthz and /readyz system routes are registered separately by the
// core. This provider exists to advertise the "health" feature via the
// capability endpoint.
func (p *HealthProvider) RegisterRoutes(_ *echo.Group) {}
