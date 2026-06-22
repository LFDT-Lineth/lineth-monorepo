/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.core.SealedBeaconBlock

/**
 * Response message containing sealed beacon blocks.
 *
 * @param blocks The list of sealed beacon blocks matching the requested range
 */
data class BeaconBlocksByRangeResponse(
  val blocks: List<SealedBeaconBlock>,
)
