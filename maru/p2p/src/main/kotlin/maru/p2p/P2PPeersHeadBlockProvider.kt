/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class P2PPeersHeadBlockProvider(
  private val peerLookup: PeerLookup,
) : PeersHeadBlockProvider {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  /**
   * Returns a map of peer IDs to their latest block numbers.
   * Only includes peers that have a valid status with a block number.
   */
  override fun getPeersHeads(): Map<String, ULong> =
    peerLookup
      .getPeers()
      .mapNotNull { peer ->
        try {
          val status = peer.getStatus()
          if (status != null) {
            peer.toPeerInfo().nodeId to status.latestBlockNumber
          } else {
            null
          }
        } catch (e: Exception) {
          log.debug("Failed to get status from peer={}, errorMessage={}", peer, e.message)
          null
        }
      }.toMap()
}
