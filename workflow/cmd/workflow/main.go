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
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryZerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	zlogadapter "logur.dev/adapter/zerolog"
	"logur.dev/logur"

	tsdkClient "go.temporal.io/sdk/client"
	tsdkConverter "go.temporal.io/sdk/converter"
	tsdkWorker "go.temporal.io/sdk/worker"

	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"

	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/config"

	cwm "github.com/NVIDIA/ncx-infra-controller-rest/workflow/internal/metrics"
	cwfh "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/health"
	cwfn "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/namespace"

	sc "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/client/site"

	// Core workflows (identity/tenancy — not domain providers)
	tenantActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/tenant"
	userActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/user"
	tenantWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/tenant"
	userWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/user"

	// Metrics activities (require Prometheus registry, stay in main)
	instanceActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/instance"
	subnetActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/subnet"
	vpcActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/vpc"

	// Site workflow triggers (cron)
	siteWorkflow "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/workflow/site"

	// Provider framework
	"github.com/NVIDIA/ncx-infra-controller-rest/provider"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/compute"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/health"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/networking"
	"github.com/NVIDIA/ncx-infra-controller-rest/providers/site"
)

const (
	// ZerologMessageFieldName specifies the field name for log message
	ZerologMessageFieldName = "msg"
	// ZerologLevelFieldName specifies the field name for log level
	ZerologLevelFieldName = "type"
)

func main() {
	// Initialize context
	ctx := context.Background()

	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.LevelFieldName = ZerologLevelFieldName
	zerolog.MessageFieldName = ZerologMessageFieldName

	cfg := config.NewConfig()
	defer cfg.Close()

	dbConfig := cfg.GetDBConfig()

	// Initialize DB connection
	dbSession, err := cdb.NewSession(ctx, dbConfig.Host, dbConfig.Port, dbConfig.Name, dbConfig.User, dbConfig.Password, "")
	if err != nil {
		log.Panic().Err(err).Msg("failed to initialize DB session")
	} else {
		defer dbSession.Close()
	}

	// Initializer Temporal client
	// Create the client object just once per process
	log.Info().Msg("creating Temporal client")

	// set up sentry client
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN != "" {
		// Initialize Sentry
		err := sentry.Init(sentry.ClientOptions{
			Dsn: sentryDSN,
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				// Modify or filter events before sending them to Sentry
				return event
			},
			Debug:            true,
			AttachStacktrace: true,
		})
		if err != nil {
			log.Error().Err(err).Msg("Sentry initialization failed")
		} else {
			defer sentry.Flush(2 * time.Second)

			// Configure Zerolog to use Sentry as a writer
			sentryWriter, err := sentryZerolog.New(sentryZerolog.Config{
				ClientOptions: sentry.ClientOptions{
					Dsn: sentryDSN,
				},
				Options: sentryZerolog.Options{
					Levels:          []zerolog.Level{zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel},
					WithBreadcrumbs: true,
					FlushTimeout:    3 * time.Second,
				},
			})
			if err != nil {
				log.Error().Err(err).Msg("failed to create Sentry writer")
			} else {
				defer sentryWriter.Close()

				// Use Sentry writer in Zerolog
				log.Logger = zerolog.New(zerolog.MultiLevelWriter(os.Stderr, sentryWriter))
			}
		}
	}

	tLogger := logur.LoggerToKV(zlogadapter.New(zerolog.New(os.Stderr)))
	var tc tsdkClient.Client

	tcfg, err := cfg.GetTemporalConfig()
	if err != nil {
		log.Panic().Err(err).Msg("failed to get Temporal config")
	}

	var tInterceptors []interceptor.ClientInterceptor

	if cfg.GetTracingEnabled() {
		otelInterceptor, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{TextMapPropagator: otel.GetTextMapPropagator()})
		if err != nil {
			log.Panic().Err(err).Msg("unable to get otelInterceptor")
		}
		tInterceptors = append(tInterceptors, otelInterceptor)
	}

	tc, err = tsdkClient.NewLazyClient(tsdkClient.Options{
		HostPort:  fmt.Sprintf("%v:%v", tcfg.Host, tcfg.Port),
		Namespace: tcfg.Namespace,
		ConnectionOptions: tsdkClient.ConnectionOptions{
			TLS: tcfg.ClientTLSCfg,
		},
		DataConverter: tsdkConverter.NewCompositeDataConverter(
			tsdkConverter.NewNilPayloadConverter(),
			tsdkConverter.NewByteSlicePayloadConverter(),
			tsdkConverter.NewProtoJSONPayloadConverterWithOptions(tsdkConverter.ProtoJSONPayloadConverterOptions{
				AllowUnknownFields: true,
			}),
			tsdkConverter.NewProtoPayloadConverter(),
			tsdkConverter.NewJSONPayloadConverter(),
		),
		// Interceptors: tInterceptors,
		Logger: tLogger,
	})

	if err != nil {
		log.Panic().Err(err).Msg("failed to create Temporal client")
	} else {
		defer tc.Close()
	}

	w := tsdkWorker.New(tc, tcfg.Queue, tsdkWorker.Options{
		WorkflowPanicPolicy:              tsdkWorker.FailWorkflow,
		MaxConcurrentActivityTaskPollers: 10,
		MaxConcurrentWorkflowTaskPollers: 10,
	})

	siteClientPool := sc.NewClientPool(tcfg)

	// Create provider registry and register providers
	registry := provider.NewRegistry()

	if err := registry.Register(networking.New()); err != nil {
		log.Panic().Err(err).Msg("failed to register networking provider")
	}
	if err := registry.Register(compute.New()); err != nil {
		log.Panic().Err(err).Msg("failed to register compute provider")
	}
	if err := registry.Register(site.New()); err != nil {
		log.Panic().Err(err).Msg("failed to register site provider")
	}
	if err := registry.Register(health.New()); err != nil {
		log.Panic().Err(err).Msg("failed to register health provider")
	}

	if err := registry.ResolveDependencies(); err != nil {
		log.Panic().Err(err).Msg("failed to resolve provider dependencies")
	}

	hooks := provider.NewHookRunner(registry, tc)

	if err := registry.InitAll(provider.ProviderContext{
		DB:                     dbSession,
		Temporal:               tc,
		Config:                 cfg,
		Registry:               registry,
		TemporalNamespace:      tcfg.Namespace,
		TemporalQueue:          tcfg.Queue,
		WorkflowSiteClientPool: siteClientPool,
		Hooks:                  hooks,
	}); err != nil {
		log.Panic().Err(err).Msg("failed to initialize providers")
	}

	log.Info().Str("Temporal Namespace", tcfg.Namespace).Msg("registering workflow and activities")

	// Register provider workflows and activities
	for _, wp := range registry.WorkflowProviders() {
		wp.RegisterWorkflows(w)
		wp.RegisterActivities(w)
		log.Info().Str("provider", wp.Name()).Msg("registered provider workflows and activities")
	}

	// Register core workflows (identity/tenancy — not domain providers)
	if tcfg.Namespace == cwfn.CloudNamespace {
		w.RegisterWorkflow(userWorkflow.UpdateUserFromNGC)
		w.RegisterWorkflow(userWorkflow.UpdateUserFromNGCWithAuxiliaryID)
	} else if tcfg.Namespace == cwfn.SiteNamespace {
		w.RegisterWorkflow(tenantWorkflow.UpdateTenantInventory)
	}

	// Register core activities
	tenantManager := tenantActivity.NewManageTenant(dbSession, siteClientPool)
	w.RegisterActivity(&tenantManager)

	if tcfg.Namespace == cwfn.CloudNamespace {
		userManager := userActivity.NewManageUser(dbSession, cfg)
		w.RegisterActivity(&userManager)
	}

	// Serve health endpoint
	hconfig := cfg.GetHealthzConfig()
	if hconfig.Enabled {
		go func() {
			log.Info().Msg("starting health check API server")
			http.HandleFunc("/healthz", cwfh.StatusHandler)
			http.HandleFunc("/readyz", cwfh.StatusHandler)

			serr := http.ListenAndServe(hconfig.GetListenAddr(), nil)
			if serr != nil {
				log.Panic().Err(serr).Msg("failed to start health check server")
			}
		}()
	}

	mconfig := cfg.GetMetricsConfig()
	if mconfig.Enabled {
		// Serve Prometheus metrics
		go func() {
			log.Info().Msg("starting Prometheus metrics server")

			reg := prometheus.NewRegistry()
			reg.MustRegister(collectors.NewGoCollector())

			// Register core metrics
			cm := cwm.NewCoreMetrics(reg)
			// TODO: Set version here when available
			cm.Info.With(prometheus.Labels{"version": "unknown", "namespace": tcfg.Namespace}).Set(1)

			if tcfg.Namespace == cwfn.SiteNamespace {
				// Register common inventory metrics activity
				inventoryMetricsManager := cwm.NewManageInventoryMetrics(reg, dbSession)
				w.RegisterActivity(&inventoryMetricsManager)

				// Register inventory operation metrics activity
				vpcLifecycleMetricsManager := vpcActivity.NewManageVpcLifecycleMetrics(reg, dbSession)
				w.RegisterActivity(&vpcLifecycleMetricsManager)

				subnetLifecycleMetricsManager := subnetActivity.NewManageSubnetLifecycleMetrics(reg, dbSession)
				w.RegisterActivity(&subnetLifecycleMetricsManager)

				instanceLifecycleMetricsManager := instanceActivity.NewManageInstanceLifecycleMetrics(reg, dbSession)
				w.RegisterActivity(&instanceLifecycleMetricsManager)
			}

			promHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})

			http.Handle("/metrics", promHandler)
			serr := http.ListenAndServe(mconfig.GetListenAddr(), nil)
			if serr != nil {
				log.Panic().Err(serr).Msg("failed to start Prometheus metrics server")
			}
		}()
	}

	// Start listening to the Task Queue
	log.Info().Str("Temporal Namespace", tcfg.Namespace).Msg("starting Temporal worker")
	err = w.Run(tsdkWorker.InterruptCh())
	if err != nil {
		log.Panic().Err(err).Str("Temporal Namespace", tcfg.Namespace).Msg("failed to start worker")
	}

	// Trigger cron workflow
	if tcfg.Namespace == cwfn.CloudNamespace {
		_, err := siteWorkflow.ExecuteMonitorHealthForAllSitesWorkflow(ctx, tc)
		if err != nil {
			log.Error().Err(err).Msg("failed to trigger Site Health Monitor workflow")
		}

		// Trigger MonitorTemporalCertExpirationForAllSites
		_, err = siteWorkflow.ExecuteMonitorTemporalCertExpirationForAllSites(ctx, tc)
		if err != nil {
			log.Error().Err(err).Msg("failed to trigger Temporal Cert Expiration Monitor workflow")
		}
		// NOTE: This will stay disabled until Site Agent is ready
		// _, err = siteWorkflow.ExecuteCheckHealthForAllSitesWorkflow(ctx, tc)
		// if err != nil {
		// 	log.Error().Err(err).Msg("failed to trigger Site Agent Health Monitor workflow")
		// }

		// Trigger MonitorSiteTemporalNamespaces
		_, err = siteWorkflow.ExecuteMonitorSiteTemporalNamespaces(ctx, tc)
		if err != nil {
			log.Error().Err(err).Msg("failed to trigger Monitor Site Temporal Namespaces workflow")
		}
	}
	// NOTE: Log messages past this point do not show up in the log output
}
