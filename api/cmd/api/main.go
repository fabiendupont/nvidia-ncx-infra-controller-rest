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

package main

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	tClient "go.temporal.io/sdk/client"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"

	capis "github.com/NVIDIA/ncx-infra-controller-rest/api/internal/server"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/config"

	sc "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/client/site"

	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/catalog"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/compute"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/dpfhcp"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/firmware"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/fulfillment"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/health"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/networking"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/nvswitch"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/showback"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/site"

	ansiblefabric "github.com/NVIDIA/ncx-infra-controller-rest/providers/ansible-fabric"

	// Imports for API doc generation
	_ "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model"
)

const (
	// ZerologMessageFieldName specifies the field name for log message
	ZerologMessageFieldName = "msg"
	// ZerologLevelFieldName specifies the field name for log level
	ZerologLevelFieldName = "type"
)

// @title NVIDIA Forge Cloud API
// @version 1.0
// @description Forge Cloud API allows you to manage datacenter resources from Cloud
// @termsOfService https://ngc.nvidia.com/legal/terms

// @contact.name NVIDIA Forge Cloud
// @contact.email forge@nvidia.com

// @license.name Proprietary

// @BasePath /
// @schemes http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.LevelFieldName = ZerologLevelFieldName
	zerolog.MessageFieldName = ZerologMessageFieldName

	cfg := config.NewConfig()
	defer cfg.Close()

	dbConfig := cfg.GetDBConfig()

	// Initialize DB connection
	dbSession, err := cdb.NewSession(context.Background(), dbConfig.Host, dbConfig.Port, dbConfig.Name, dbConfig.User, dbConfig.Password, "")
	if err != nil {
		log.Panic().Err(err).Msg("failed to initialize DB session")
	} else {
		defer dbSession.Close()
	}

	// Initialize Temporal client and namespace client
	// Client objects are expensive so they are only initialized once
	tcfg, err := cfg.GetTemporalConfig()

	if err != nil {
		log.Panic().Err(err).Msg("failed to get Temporal config")
	}

	tc, tnc, err := capis.InitTemporalClients(tcfg, cfg.GetTracingEnabled())

	if err != nil {
		log.Panic().Err(err).Msg("failed to create Temporal clients")
	} else {
		defer tc.Close()
		defer tnc.Close()
	}

	_, err = tc.CheckHealth(context.Background(), &tClient.CheckHealthRequest{})
	if err != nil {
		log.Panic().Err(err).Msg("failed to check Temporal health")
	}

	scp := sc.NewClientPool(tcfg)

	// Set up deployment profiles
	provider.RegisterProfileProviders(provider.ProfileManagement, []func() provider.Provider{
		func() provider.Provider { return networking.New() },
		func() provider.Provider { return compute.New() },
		func() provider.Provider { return health.New() },
		func() provider.Provider { return nvswitch.New() },
		// firmware omitted: depends on nico-site which is not in this profile
	})
	provider.RegisterProfileProviders(provider.ProfileSite, []func() provider.Provider{
		func() provider.Provider { return networking.New() },
		func() provider.Provider { return compute.New() },
		func() provider.Provider { return site.New() },
		func() provider.Provider { return health.New() },
		func() provider.Provider { return nvswitch.New() },
	})
	provider.RegisterProfileProviders(provider.ProfileManagementWithSite, []func() provider.Provider{
		func() provider.Provider { return networking.New() },
		func() provider.Provider { return compute.New() },
		func() provider.Provider { return site.New() },
		func() provider.Provider { return health.New() },
		func() provider.Provider { return firmware.New() },
		func() provider.Provider { return nvswitch.New() },
	})
	provider.RegisterProfileProviders(provider.ProfileNCP, []func() provider.Provider{
		func() provider.Provider { return networking.New() },
		func() provider.Provider { return compute.New() },
		func() provider.Provider { return site.New() },
		func() provider.Provider { return health.New() },
		func() provider.Provider { return firmware.New() },
		func() provider.Provider { return nvswitch.New() },
		func() provider.Provider { return catalog.New() },
		func() provider.Provider { return fulfillment.New() },
		func() provider.Provider { return showback.New() },
		func() provider.Provider { return dpfhcp.New() },
		func() provider.Provider { return ansiblefabric.NewFromEnv() },
	})

	// Create provider registry and register providers for the active profile
	profile := provider.GetProfile()
	log.Info().Str("profile", profile).Msg("loading deployment profile")

	factories := provider.GetProfileProviders(profile)
	if factories == nil {
		log.Panic().Str("profile", profile).Msg("unknown deployment profile")
	}

	registry := provider.NewRegistry()
	for _, factory := range factories {
		if err := registry.Register(factory()); err != nil {
			log.Panic().Err(err).Msg("failed to register provider")
		}
	}

	// Resolve dependencies and initialize providers
	if err := registry.ResolveDependencies(); err != nil {
		log.Panic().Err(err).Msg("failed to resolve provider dependencies")
	}

	apiPathPrefix := "/org/:orgName/" + cfg.GetAPIName()

	if err := registry.InitAll(provider.ProviderContext{
		DB:             dbSession,
		Temporal:       tc,
		TemporalNS:     tnc,
		SiteClientPool: scp,
		Config:         cfg,
		Registry:       registry,
		APIPathPrefix:  apiPathPrefix,
	}); err != nil {
		log.Panic().Err(err).Msg("failed to initialize providers")
	}

	log.Info().Int("count", len(registry.APIProviders())).Msg("providers initialized")

	// Initialize API Echo instance
	e := capis.InitAPIServer(cfg, dbSession, tc, tnc, scp, registry)

	mconfig := cfg.GetMetricsConfig()
	if mconfig.Enabled {
		// Initialize Prometheus Echo instance
		ep := capis.InitMetricsServer(e)

		// Start Prometheus server
		log.Info().Msg("starting Metrics server")
		go func() {
			ep.Logger.Fatal(ep.Start(mconfig.GetListenAddr()))
		}()
	}

	// Start main server
	log.Info().Msg("starting API server")
	e.Logger.Fatal(e.Start(":8388"))
}
