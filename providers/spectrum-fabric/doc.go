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

// Package spectrumfabric implements a NICo provider that manages Spectrum-X
// switch fabric directly via the NVUE REST API, without an intermediary
// orchestration layer like Ansible/AAP or Netris Controller.
//
// This provider communicates with Cumulus Linux switches running NVUE by
// sending JSON PATCH requests to the NVUE declarative API. Each NICo
// networking event (VPC create/delete, Subnet create/delete) is translated
// into NVUE configuration changes:
//
//   - VPC lifecycle maps to VRF objects on Spectrum switches
//     (NVUE path: /vrf/{name})
//   - Subnet lifecycle maps to VxLAN VNI + bridge VLAN configuration
//     (NVUE paths: /nve/vxlan, /interface/{name}/bridge/domain)
//
// The NVUE REST API follows a revision-based workflow: changes are staged
// into a revision, then committed atomically. This provider uses that
// workflow for every fabric sync operation:
//
//  1. Create a new NVUE revision
//  2. PATCH the desired configuration into the revision
//  3. Apply (commit) the revision
//  4. Poll until the apply completes
//
// The NVUE REST client is provided by the github.com/NVIDIA/nvue-client-go
// package. The sync functions use its ConfigureAndApply helper for create
// operations and explicit revision + delete + apply for removal operations.
package spectrumfabric
