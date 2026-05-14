/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

import maru.core.SealedBeaconBlock
import maru.p2p.MaruPeer

data class SealedBlockWithPeer(
  val sealedBeaconBlock: SealedBeaconBlock,
  val peer: MaruPeer,
)
