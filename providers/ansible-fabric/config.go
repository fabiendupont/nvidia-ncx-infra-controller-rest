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

package ansiblefabric

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// TemplateConfig maps NICo lifecycle events to AAP job template IDs.
// Each template is an Ansible playbook that configures switches using
// the nvidia.nvue collection.
type TemplateConfig struct {
	// Ethernet fabric automation (nvidia.nvue collection)
	CreateVPC    int `json:"create_vpc"`
	DeleteVPC    int `json:"delete_vpc"`
	CreateSubnet int `json:"create_subnet"`
	DeleteSubnet int `json:"delete_subnet"`

	// InfiniBand fabric automation (UFM REST API)
	CreateIBPartition int `json:"create_ib_partition"`
	DeleteIBPartition int `json:"delete_ib_partition"`

	// Instance lifecycle (port configuration)
	ConfigureInstance   int `json:"configure_instance"`
	DeconfigureInstance int `json:"deconfigure_instance"`
}

// ProviderConfig holds the full configuration for the ansible-fabric provider.
type ProviderConfig struct {
	// AAPURL is the base URL of the AAP Controller.
	AAPURL string

	// AAPToken is the authentication token for the AAP Controller.
	AAPToken string

	// AAPUsername is the AAP Controller username. Used only if AAPToken is empty.
	AAPUsername string

	// AAPPassword is the AAP Controller password. Used only if AAPToken is empty.
	AAPPassword string

	// Templates maps lifecycle events to AAP job template IDs.
	Templates TemplateConfig

	// JobPollInterval is how often to poll AAP for job completion status.
	// Defaults to 5 seconds if zero.
	JobPollInterval time.Duration

	// JobTimeout is the maximum time to wait for a job to complete.
	// Defaults to 10 minutes if zero.
	JobTimeout time.Duration
}

// ConfigFromEnv reads the ansible-fabric provider configuration from
// environment variables. Template IDs default to 0 (disabled) if not set.
func ConfigFromEnv() ProviderConfig {
	cfg := ProviderConfig{
		AAPURL:      os.Getenv("AAP_URL"),
		AAPToken:    os.Getenv("AAP_TOKEN"),
		AAPUsername: os.Getenv("AAP_USERNAME"),
		AAPPassword: os.Getenv("AAP_PASSWORD"),
	}

	cfg.Templates = TemplateConfig{
		CreateVPC:           envInt("AAP_TEMPLATE_CREATE_VPC"),
		DeleteVPC:           envInt("AAP_TEMPLATE_DELETE_VPC"),
		CreateSubnet:        envInt("AAP_TEMPLATE_CREATE_SUBNET"),
		DeleteSubnet:        envInt("AAP_TEMPLATE_DELETE_SUBNET"),
		CreateIBPartition:   envInt("AAP_TEMPLATE_CREATE_IB_PARTITION"),
		DeleteIBPartition:   envInt("AAP_TEMPLATE_DELETE_IB_PARTITION"),
		ConfigureInstance:   envInt("AAP_TEMPLATE_CONFIGURE_INSTANCE"),
		DeconfigureInstance: envInt("AAP_TEMPLATE_DECONFIGURE_INSTANCE"),
	}

	if d := os.Getenv("AAP_JOB_POLL_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			cfg.JobPollInterval = parsed
		}
	}

	if d := os.Getenv("AAP_JOB_TIMEOUT"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			cfg.JobTimeout = parsed
		}
	}

	return cfg
}

// Validate checks that the minimum required configuration is present.
func (c *ProviderConfig) Validate() error {
	if c.AAPURL == "" {
		return fmt.Errorf("AAP_URL is required")
	}
	if c.AAPToken == "" && c.AAPUsername == "" {
		return fmt.Errorf("either AAP_TOKEN or AAP_USERNAME/AAP_PASSWORD is required")
	}
	return nil
}

// HasTemplate returns true if a job template ID is configured for the given
// operation. A zero template ID means the operation is disabled.
func (c *TemplateConfig) HasTemplate(templateID int) bool {
	return templateID > 0
}

func envInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
