/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api

import io.javalin.Javalin
import maru.VersionProvider
import maru.api.beacon.GetBlock
import maru.api.beacon.GetBlockHeader
import maru.api.beacon.GetStateValidator
import maru.api.beacon.GetStateValidators
import maru.api.node.GetHealth
import maru.api.node.GetNetworkIdentity
import maru.api.node.GetPeer
import maru.api.node.GetPeerCount
import maru.api.node.GetPeers
import maru.api.node.GetSyncingStatus
import maru.api.node.GetVersion
import maru.p2p.NetworkDataProvider
import maru.syncing.SyncStatusProvider
import java.util.concurrent.CompletableFuture

class ApiServerImpl(
  val config: Config,
  val networkDataProvider: NetworkDataProvider,
  val versionProvider: VersionProvider,
  val chainDataProvider: ChainDataProvider,
  val syncStatusProvider: SyncStatusProvider,
  val isElOnlineProvider: () -> Boolean,
) : ApiServer {
  data class Config(
    val port: UInt,
  )

  var app: Javalin? = null

  override fun start(): CompletableFuture<Unit> {
    if (app != null) {
      app!!.start(config.port.toInt())
    } else {
      // To support apiserver restarts after stop, we need to create a new Javalin instance
      // https://github.com/javalin/javalin/issues/941
      app = Javalin
        .create()
        .exception(HandlerException::class.java) { e, ctx ->
          ctx.status(e.code).json(ApiExceptionResponse(e.code, e.message))
        }.exception(Exception::class.java) { e, ctx ->
          ctx.status(500).json(ApiExceptionResponse(500, "Internal Server Error"))
        }.get(GetNetworkIdentity.ROUTE, GetNetworkIdentity(networkDataProvider))
        .get(GetPeers.ROUTE, GetPeers(networkDataProvider))
        .get(GetPeer.ROUTE, GetPeer(networkDataProvider))
        .get(GetPeerCount.ROUTE, GetPeerCount(networkDataProvider))
        .get(GetVersion.ROUTE, GetVersion(versionProvider))
        .get(
          GetSyncingStatus.ROUTE,
          GetSyncingStatus(
            syncStatusProvider = syncStatusProvider,
            isElOnlineProvider = isElOnlineProvider,
          ),
        ).get(GetHealth.ROUTE, GetHealth())
        .get(GetBlockHeader.ROUTE, GetBlockHeader(chainDataProvider))
        .get(GetBlock.ROUTE, GetBlock(chainDataProvider))
        .get(GetStateValidator.ROUTE, GetStateValidator(chainDataProvider))
        .get(GetStateValidators.ROUTE, GetStateValidators(chainDataProvider))
        .start(config.port.toInt())
    }
    return CompletableFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    app?.stop()
    app = null
    return CompletableFuture.completedFuture(Unit)
  }

  override fun port(): Int = app!!.port()
}
