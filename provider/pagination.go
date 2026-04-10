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
	"strconv"

	echo "github.com/labstack/echo/v4"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ListResponse wraps paginated results with metadata.
type ListResponse struct {
	Items  interface{} `json:"items"`
	Total  int         `json:"total"`
	Offset int         `json:"offset"`
	Limit  int         `json:"limit"`
}

// ParsePagination extracts offset and limit from query params.
// Defaults to offset=0, limit=DefaultPageSize.
func ParsePagination(c echo.Context) (offset, limit int) {
	offset = 0
	limit = DefaultPageSize

	if v := c.QueryParam("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	if v := c.QueryParam("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	return offset, limit
}

// Paginate applies offset and limit to a slice length and returns
// the start and end indices. Returns 0,0 if offset exceeds total.
func Paginate(total, offset, limit int) (start, end int) {
	if offset >= total {
		return 0, 0
	}
	start = offset
	end = offset + limit
	if end > total {
		end = total
	}
	return start, end
}
