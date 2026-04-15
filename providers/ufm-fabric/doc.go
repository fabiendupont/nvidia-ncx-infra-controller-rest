/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

// Package ufmfabric implements a NICo provider that manages InfiniBand
// partition (PKEY) lifecycle on UFM Enterprise directly via its REST API,
// without an intermediary orchestration layer like Ansible/AAP.
//
// This provider replaces the IB partition handling in the ansible-fabric
// provider for deployments that do not require AAP. Each NICo IB partition
// event is translated into a UFM REST API call:
//
//   - IB partition create → POST /ufmRest/resources/pkeys/add + GUID membership
//   - IB partition delete → DELETE /ufmRest/resources/pkeys/{pkey}
//
// The UFM REST client is provided by the github.com/fabiendupont/nvidia-ufm-api
// package (internal/ufmclient).
package ufmfabric
