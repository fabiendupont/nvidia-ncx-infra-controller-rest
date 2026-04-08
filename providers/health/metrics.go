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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// FaultMetrics exposes fault event data as Prometheus metrics for
// Grafana dashboards, AlertManager rules, and SLA reporting.
type FaultMetrics struct {
	// Gauges — current state
	openFaults *prometheus.GaugeVec
	// Counters — cumulative
	totalFaults    *prometheus.CounterVec
	totalResolved  *prometheus.CounterVec
	totalEscalated *prometheus.CounterVec
	// Histograms — latency distributions
	mttr *prometheus.HistogramVec

	faultStore *FaultStore
}

// NewFaultMetrics creates and registers Prometheus metrics for fault events.
func NewFaultMetrics(reg prometheus.Registerer, faultStore *FaultStore) *FaultMetrics {
	m := &FaultMetrics{
		faultStore: faultStore,

		openFaults: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "nico",
				Subsystem: "fault",
				Name:      "events_open",
				Help:      "Current number of open fault events by component and severity.",
			},
			[]string{"component", "severity"},
		),

		totalFaults: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "nico",
				Subsystem: "fault",
				Name:      "events_total",
				Help:      "Total number of fault events ingested by component and source.",
			},
			[]string{"component", "source"},
		),

		totalResolved: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "nico",
				Subsystem: "fault",
				Name:      "events_resolved_total",
				Help:      "Total number of fault events resolved by component.",
			},
			[]string{"component"},
		),

		totalEscalated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "nico",
				Subsystem: "fault",
				Name:      "events_escalated_total",
				Help:      "Total number of fault events escalated by component.",
			},
			[]string{"component"},
		),

		mttr: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "nico",
				Subsystem: "fault",
				Name:      "mttr_seconds",
				Help:      "Mean time to repair (seconds) by component.",
				Buckets:   []float64{60, 120, 300, 600, 900, 1200, 1800, 3600, 7200},
			},
			[]string{"component"},
		),
	}

	reg.MustRegister(m.openFaults)
	reg.MustRegister(m.totalFaults)
	reg.MustRegister(m.totalResolved)
	reg.MustRegister(m.totalEscalated)
	reg.MustRegister(m.mttr)

	return m
}

// RecordIngestion increments the total fault counter when a new fault
// is ingested.
func (m *FaultMetrics) RecordIngestion(component, source string) {
	m.totalFaults.With(prometheus.Labels{
		"component": component,
		"source":    source,
	}).Inc()
}

// RecordResolution increments the resolved counter and records MTTR
// when a fault is resolved.
func (m *FaultMetrics) RecordResolution(component string, detectedAt, resolvedAt time.Time) {
	m.totalResolved.With(prometheus.Labels{
		"component": component,
	}).Inc()

	mttrSeconds := resolvedAt.Sub(detectedAt).Seconds()
	m.mttr.With(prometheus.Labels{
		"component": component,
	}).Observe(mttrSeconds)
}

// RecordEscalation increments the escalated counter.
func (m *FaultMetrics) RecordEscalation(component string) {
	m.totalEscalated.With(prometheus.Labels{
		"component": component,
	}).Inc()
}

// RefreshOpenFaults recomputes the open faults gauge from the store.
// Called periodically or after state changes.
func (m *FaultMetrics) RefreshOpenFaults() {
	if m.faultStore == nil {
		return
	}

	// Reset all gauges to zero before recomputing
	m.openFaults.Reset()

	// Count open faults by component and severity
	allFaults := m.faultStore.GetAll(FaultEventFilter{
		State: []string{FaultStateOpen, FaultStateRemediating, FaultStateAcknowledged},
	})

	counts := make(map[string]map[string]float64)
	for _, f := range allFaults {
		if counts[f.Component] == nil {
			counts[f.Component] = make(map[string]float64)
		}
		counts[f.Component][f.Severity]++
	}

	for component, severities := range counts {
		for severity, count := range severities {
			m.openFaults.With(prometheus.Labels{
				"component": component,
				"severity":  severity,
			}).Set(count)
		}
	}
}
