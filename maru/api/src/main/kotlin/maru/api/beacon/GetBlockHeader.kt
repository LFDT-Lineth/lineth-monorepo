/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.beacon

import com.fasterxml.jackson.annotation.JsonProperty
import io.javalin.http.Context
import io.javalin.http.Handler
import maru.api.ChainDataProvider

// https://ethereum.github.io/beacon-APIs/#/Beacon/getBlockHeader
data class GetBlockHeaderResponse(
  @JsonProperty("execution_optimistic")val executionOptimistic: Boolean,
  @JsonProperty("finalized") val finalized: Boolean,
  @JsonProperty("data") val data: SignedBeaconBlockHeader,
)

class GetBlockHeader(
  val chainDataProvider: ChainDataProvider,
) : Handler {
  override fun handle(ctx: Context) {
    val maruSealedBeaconBlock = getBlock(ctx.pathParam(BLOCK_ID), chainDataProvider)
    val signedBeaconBlockHeader =
      SignedBeaconBlockHeader(
        message = maruSealedBeaconBlock.beaconBlock.beaconBlockHeader.toBeaconBlockHeader(),
        signature = "0x",
      )
    val response =
      GetBlockHeaderResponse(
        executionOptimistic = false,
        finalized = false,
        data = signedBeaconBlockHeader,
      )
    ctx.status(200).json(response)
  }

  companion object {
    const val BLOCK_ID: String = "block_id"
    const val ROUTE: String = "/eth/v1/beacon/headers/{$BLOCK_ID}"
  }
}
