/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.node

import com.fasterxml.jackson.annotation.JsonProperty
import io.javalin.http.Context
import io.javalin.http.Handler
import io.javalin.http.HttpStatus
import maru.p2p.NetworkDataProvider

data class GetPeersResponse(
  val data: List<PeerData>,
  val meta: PeerMetaData,
)

data class PeerData(
  @JsonProperty("peer_id") val peerId: String,
  val enr: String?,
  @JsonProperty("last_seen_p2p_address") val lastSeenP2PAddress: String,
  val state: String,
  val direction: String,
)

data class PeerMetaData(
  val count: Int,
)

class GetPeers(
  val networkDataProvider: NetworkDataProvider,
) : Handler {
  override fun handle(ctx: Context) {
    val peers = networkDataProvider.getPeers().map { it.toPeerData() }
    ctx.json(GetPeersResponse(data = peers, meta = PeerMetaData(count = peers.count())))
    ctx.status(HttpStatus.OK)
  }

  companion object {
    const val ROUTE = "/eth/v1/node/peers"
  }
}
