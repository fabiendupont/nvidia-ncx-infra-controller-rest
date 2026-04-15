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

// loadDefaultClassifications returns the default classification-to-remediation
// mappings for all component types. These are built into the provider and can
// be overridden by operators via the classifications API.
func loadDefaultClassifications() map[string]*ClassificationMapping {
	return map[string]*ClassificationMapping{
		// GPU faults
		"gpu-xid-48": {
			Classification:  "gpu-xid-48",
			Component:       ComponentGPU,
			Severity:        SeverityCritical,
			Remediation:     "gpu-reset",
			MaxRetries:      2,
			ValidationLevel: 3,
		},
		"gpu-xid-79": {
			Classification:  "gpu-xid-79",
			Component:       ComponentGPU,
			Severity:        SeverityCritical,
			Remediation:     "gpu-reset",
			MaxRetries:      1,
			ValidationLevel: 3,
			EscalateMessage: "GPU fallen off bus — likely hardware failure",
		},
		"gpu-thermal": {
			Classification:  "gpu-thermal",
			Component:       ComponentCooling,
			Severity:        SeverityWarning,
			Remediation:     "wait-and-recheck",
			MaxRetries:      6,
			RecheckInterval: "5m",
			EscalateMessage: "Sustained thermal throttling",
		},
		"dcgm-ecc-dbe": {
			Classification:  "dcgm-ecc-dbe",
			Component:       ComponentGPU,
			Severity:        SeverityCritical,
			Remediation:     "gpu-reset",
			MaxRetries:      1,
			ValidationLevel: 3,
		},
		"dcgm-ecc-sbe-trending": {
			Classification:  "dcgm-ecc-sbe-trending",
			Component:       ComponentGPU,
			Severity:        SeverityWarning,
			Remediation:     "monitor",
			MaxRetries:      24,
			RecheckInterval: "1h",
			EscalateMessage: "SBE count increasing — schedule replacement",
		},

		// NVSwitch faults
		"nvswitch-firmware-failed": {
			Classification:  "nvswitch-firmware-failed",
			Component:       ComponentNVSwitch,
			Severity:        SeverityWarning,
			Remediation:     "nvswitch-firmware-retry",
			MaxRetries:      2,
			EscalateMessage: "Firmware update failed after retries",
		},
		"nvswitch-unreachable": {
			Classification:  "nvswitch-unreachable",
			Component:       ComponentNVSwitch,
			Severity:        SeverityCritical,
			Remediation:     "nvswitch-power-cycle",
			MaxRetries:      1,
			EscalateMessage: "NVSwitch unreachable after power cycle",
		},

		// Power faults
		"power-psu-fault": {
			Classification:  "power-psu-fault",
			Component:       ComponentPower,
			Severity:        SeverityCritical,
			Remediation:     "power-redundancy-check",
			MaxRetries:      1,
			EscalateMessage: "PSU failure — check redundancy",
		},
		"power-sensor-upper-critical": {
			Classification:  "power-sensor-upper-critical",
			Component:       ComponentPower,
			Severity:        SeverityCritical,
			Remediation:     "power-wait-and-recheck",
			MaxRetries:      5,
			RecheckInterval: "2m",
			EscalateMessage: "Sensor reading sustained above critical threshold",
		},
		"power-sensor-upper-caution": {
			Classification:  "power-sensor-upper-caution",
			Component:       ComponentPower,
			Severity:        SeverityWarning,
			Remediation:     "power-wait-and-recheck",
			MaxRetries:      12,
			RecheckInterval: "5m",
		},

		// Network / DPU faults
		"nhc-ib-link-down": {
			Classification:  "nhc-ib-link-down",
			Component:       ComponentNetwork,
			Severity:        SeverityCritical,
			Remediation:     "dpu-reset",
			MaxRetries:      2,
			ValidationLevel: 2,
		},
		"dpu-unreachable": {
			Classification:  "dpu-unreachable",
			Component:       ComponentNetwork,
			Severity:        SeverityCritical,
			Remediation:     "dpu-bmc-reset",
			MaxRetries:      1,
			EscalateMessage: "DPU unreachable after BMC reset — may need BFB re-flash",
		},
		"dpu-hbn-down": {
			Classification: "dpu-hbn-down",
			Component:      ComponentNetwork,
			Severity:       SeverityCritical,
			Remediation:    "dpu-hbn-restart",
			MaxRetries:     2,
		},

		// BMC faults
		"bmc-unreachable": {
			Classification:  "bmc-unreachable",
			Component:       ComponentBMC,
			Severity:        SeverityWarning,
			Remediation:     "bmc-reset",
			MaxRetries:      2,
			EscalateMessage: "BMC unreachable after cold reset",
		},

		// Storage faults
		"storage-nvme-wear-critical": {
			Classification:  "storage-nvme-wear-critical",
			Component:       ComponentStorage,
			Severity:        SeverityWarning,
			Remediation:     "schedule-replacement",
			EscalateMessage: "NVMe wear level critical — schedule drive replacement",
		},
		"storage-nvme-failed": {
			Classification:  "storage-nvme-failed",
			Component:       ComponentStorage,
			Severity:        SeverityCritical,
			Remediation:     "escalate-immediately",
			EscalateMessage: "NVMe drive failed — RMA required",
		},
	}
}
