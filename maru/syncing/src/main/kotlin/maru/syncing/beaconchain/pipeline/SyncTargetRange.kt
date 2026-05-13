/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

/** * Represents a range of blocks to be synchronized.
 *
 * @property startBlock The starting block number (inclusive).
 * @property endBlock The ending block number (inclusive).
 */
data class SyncTargetRange(
  val startBlock: ULong,
  val endBlock: ULong,
)
