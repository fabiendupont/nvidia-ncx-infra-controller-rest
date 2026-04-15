/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ufmfabric

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/fabiendupont/nvidia-ufm-api/pkg/ufmclient"
)

// SyncIBPartitionToFabric creates an InfiniBand partition (PKEY) on
// the UFM-managed fabric and adds GUID members to it.
//
// The UFM REST API workflow:
//  1. POST /ufmRest/resources/pkeys/add — create empty partition with PKEY
//  2. POST /ufmRest/resources/pkeys/   — add GUID members (if any)
//
// This operation is idempotent — if the PKEY already exists, UFM returns
// 400 but the partition remains intact. We check existence first to
// avoid noisy errors.
func (p *UFMFabricProvider) SyncIBPartitionToFabric(ctx context.Context, partitionID, partitionName, pkey, tenantID string, guids []string) error {
	logger := log.With().Str("provider", p.Name()).Str("partition_id", partitionID).Logger()

	if !p.config.Features.SyncIBPartition {
		logger.Debug().Msg("IB partition sync disabled, skipping")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	logger.Info().
		Str("partition_name", partitionName).
		Str("pkey", pkey).
		Int("guid_count", len(guids)).
		Msg("syncing IB partition to UFM fabric")

	// Step 1: Create the empty partition.
	err := p.client.CreateEmptyPKey(ctx, &ufmclient.PKeyCreateRequest{
		PKey:          pkey,
		PartitionName: partitionName,
		IPOverIB:      true,
		Index0:        false,
	})
	if err != nil {
		// Check if it's a "already exists" error (UFM returns 400).
		if apiErr, ok := err.(*ufmclient.APIError); ok && apiErr.StatusCode == 400 && strings.Contains(apiErr.Body, "exist") {
			logger.Info().Str("pkey", pkey).Msg("PKEY already exists on UFM, skipping creation")
		} else {
			return fmt.Errorf("creating PKEY %s on UFM: %w", pkey, err)
		}
	}

	// Step 2: Add GUID members if provided.
	if len(guids) > 0 {
		err := p.client.AddGUIDsToPKey(ctx, &ufmclient.PKeyAddGUIDsRequest{
			PKey:          pkey,
			GUIDs:         guids,
			PartitionName: partitionName,
			IPOverIB:      true,
			Index0:        false,
			Membership:    "full",
		})
		if err != nil {
			return fmt.Errorf("adding %d GUIDs to PKEY %s on UFM: %w", len(guids), pkey, err)
		}

		logger.Info().
			Int("guid_count", len(guids)).
			Msg("GUIDs added to PKEY on UFM")
	}

	logger.Info().
		Str("pkey", pkey).
		Str("partition_name", partitionName).
		Msg("IB partition synced to UFM fabric")

	return nil
}

// RemoveIBPartitionFromFabric removes an InfiniBand partition (PKEY)
// from the UFM-managed fabric.
//
// The UFM REST API call:
//
//	DELETE /ufmRest/resources/pkeys/{pkey}
//
// This operation is idempotent — if the PKEY does not exist, UFM
// returns 404 which we treat as success.
func (p *UFMFabricProvider) RemoveIBPartitionFromFabric(ctx context.Context, partitionID, pkey string) error {
	logger := log.With().Str("provider", p.Name()).Str("partition_id", partitionID).Logger()

	if !p.config.Features.SyncIBPartition {
		logger.Debug().Msg("IB partition sync disabled, skipping removal")
		return nil
	}

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	logger.Info().Str("pkey", pkey).Msg("removing IB partition from UFM fabric")

	err := p.client.DeletePKey(ctx, pkey)
	if err != nil {
		// 404 means the PKEY is already gone — treat as success.
		if apiErr, ok := err.(*ufmclient.APIError); ok && apiErr.StatusCode == 404 {
			logger.Info().Str("pkey", pkey).Msg("PKEY not found on UFM, already removed")
			return nil
		}
		return fmt.Errorf("deleting PKEY %s from UFM: %w", pkey, err)
	}

	logger.Info().Str("pkey", pkey).Msg("IB partition removed from UFM fabric")

	return nil
}

// AddGUIDsToPartition adds GUID members to an existing IB partition
// on the UFM-managed fabric.
//
// The UFM REST API call:
//
//	POST /ufmRest/resources/pkeys/ (with guids array)
func (p *UFMFabricProvider) AddGUIDsToPartition(ctx context.Context, pkey string, guids []string, membership string) error {
	logger := log.With().Str("provider", p.Name()).Str("pkey", pkey).Logger()

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	if membership == "" {
		membership = "full"
	}

	logger.Info().
		Int("guid_count", len(guids)).
		Str("membership", membership).
		Msg("adding GUIDs to PKEY on UFM")

	err := p.client.AddGUIDsToPKey(ctx, &ufmclient.PKeyAddGUIDsRequest{
		PKey:       pkey,
		GUIDs:      guids,
		IPOverIB:   true,
		Membership: membership,
	})
	if err != nil {
		return fmt.Errorf("adding GUIDs to PKEY %s: %w", pkey, err)
	}

	return nil
}

// RemoveGUIDsFromPartition removes GUID members from an existing IB
// partition on the UFM-managed fabric.
//
// The UFM REST API call:
//
//	DELETE /ufmRest/resources/pkeys/{pkey}/guids/{guid1},{guid2}
func (p *UFMFabricProvider) RemoveGUIDsFromPartition(ctx context.Context, pkey string, guids []string) error {
	logger := log.With().Str("provider", p.Name()).Str("pkey", pkey).Logger()

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	logger.Info().
		Int("guid_count", len(guids)).
		Msg("removing GUIDs from PKEY on UFM")

	err := p.client.RemoveGUIDsFromPKey(ctx, pkey, guids)
	if err != nil {
		return fmt.Errorf("removing GUIDs from PKEY %s: %w", pkey, err)
	}

	return nil
}

// AddHostsToPartition adds all ports of named hosts to an IB partition
// on the UFM-managed fabric. This is an async operation — UFM returns
// a job that we poll until completion.
//
// The UFM REST API call:
//
//	POST /ufmRest/resources/pkeys/hosts
func (p *UFMFabricProvider) AddHostsToPartition(ctx context.Context, pkey string, hosts []string) error {
	logger := log.With().Str("provider", p.Name()).Str("pkey", pkey).Logger()

	p.syncMu.Lock()
	defer p.syncMu.Unlock()

	logger.Info().
		Int("host_count", len(hosts)).
		Msg("adding hosts to PKEY on UFM")

	job, err := p.client.AddHostsToPKey(ctx, &ufmclient.PKeyAddHostsRequest{
		PKey:       pkey,
		HostsNames: strings.Join(hosts, ","),
		IPOverIB:   true,
		Membership: "full",
	})
	if err != nil {
		return fmt.Errorf("adding hosts to PKEY %s: %w", pkey, err)
	}

	// Wait for the async job to complete.
	pollInterval := p.config.JobPollInterval
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	_, err = p.client.WaitForJob(ctx, job.ID, pollInterval)
	if err != nil {
		return fmt.Errorf("waiting for host-add job on PKEY %s: %w", pkey, err)
	}

	logger.Info().
		Int("host_count", len(hosts)).
		Msg("hosts added to PKEY on UFM")

	return nil
}
