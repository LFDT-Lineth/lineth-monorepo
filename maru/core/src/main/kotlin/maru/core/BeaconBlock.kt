/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

/** BeaconBlock will be part of the QBFT Proposal payload */
data class BeaconBlock(
  val beaconBlockHeader: BeaconBlockHeader,
  val beaconBlockBody: BeaconBlockBody,
)
