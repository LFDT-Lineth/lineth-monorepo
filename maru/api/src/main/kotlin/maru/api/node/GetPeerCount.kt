/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.node

import io.javalin.http.Context
import io.javalin.http.Handler
import maru.p2p.NetworkDataProvider
import maru.p2p.PeerInfo

data class GetPeerCountResponse(
  val data: PeerCountData,
)

data class PeerCountData(
  val disconnected: String,
  val connected: String,
  val connecting: String,
  val disconnecting: String,
)

class GetPeerCount(
  val networkDataProvider: NetworkDataProvider,
) : Handler {
  override fun handle(ctx: Context) {
    val peerCountMap =
      networkDataProvider
        .getPeers()
        .groupBy { it.status }
        .mapValues { it.value.size }

    ctx.json(
      GetPeerCountResponse(
        data = PeerCountData(
          disconnected = peerCountMap.getOrDefault(PeerInfo.PeerStatus.DISCONNECTED, 0).toString(),
          connected = peerCountMap.getOrDefault(PeerInfo.PeerStatus.CONNECTED, 0).toString(),
          connecting = peerCountMap.getOrDefault(PeerInfo.PeerStatus.CONNECTING, 0).toString(),
          disconnecting = peerCountMap.getOrDefault(PeerInfo.PeerStatus.DISCONNECTING, 0).toString(),
        ),
      ),
    )
  }

  companion object {
    const val ROUTE = "/eth/v1/node/peer_count"
  }
}
