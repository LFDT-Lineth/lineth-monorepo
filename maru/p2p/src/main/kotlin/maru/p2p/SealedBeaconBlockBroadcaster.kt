/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.core.SealedBeaconBlock
import tech.pegasys.teku.infrastructure.async.SafeFuture

class SealedBeaconBlockBroadcaster(
  val p2PNetwork: P2PNetwork,
) : SealedBeaconBlockHandler<Unit> {
  override fun handleSealedBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<Unit> {
    // TODO: New block message might need an intermediary wrapper in the future
    val message = MessageData(GossipMessageType.BEACON_BLOCK, payload = sealedBeaconBlock)
    p2PNetwork.broadcastMessage(message)
    return SafeFuture.completedFuture(Unit)
  }
}
