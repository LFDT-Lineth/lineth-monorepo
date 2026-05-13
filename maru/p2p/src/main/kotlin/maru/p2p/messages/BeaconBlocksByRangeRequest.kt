/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

/**
 * Request message for fetching beacon blocks by range.
 * Similar to BeaconBlocksByRange in Ethereum consensus specs but adapted for Maru.
 *
 * @param startBlockNumber The block number to start fetching from (inclusive).
 * @param count The maximum number of blocks to fetch (maybe less than requested if not enough blocks are available).
 */
data class BeaconBlocksByRangeRequest(
  val startBlockNumber: ULong,
  val count: ULong,
)
