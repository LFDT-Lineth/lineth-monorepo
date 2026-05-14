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
import maru.syncing.SyncStatusProvider

data class GetSyncingStatusResponse(
  val data: SyncingStatusData,
)

data class SyncingStatusData(
  @JsonProperty("head_slot") val headSlot: String,
  @JsonProperty("sync_distance") val syncDistance: String,
  @JsonProperty("is_syncing") val isSyncing: Boolean,
  @JsonProperty("is_optimistic") val isOptimistic: Boolean,
  @JsonProperty("el_offline") val elOffline: Boolean,
)

class GetSyncingStatus(
  private val syncStatusProvider: SyncStatusProvider,
  private val isElOnlineProvider: () -> Boolean,
) : Handler {
  override fun handle(ctx: Context) {
    ctx.status(200).json(
      GetSyncingStatusResponse(
        data = SyncingStatusData(
          headSlot = syncStatusProvider.getCLSyncTarget().toString(),
          syncDistance = syncStatusProvider.getBeaconSyncDistance().toString(),
          isSyncing = !syncStatusProvider.isBeaconChainSynced(),
          isOptimistic = true, // we only support optimistic mode for now
          elOffline = !isElOnlineProvider.invoke(),
        ),
      ),
    )
  }

  companion object {
    const val ROUTE = "/eth/v1/node/syncing"
  }
}
