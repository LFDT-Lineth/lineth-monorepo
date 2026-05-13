/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.clients.beacon

import maru.api.beacon.GetBlockResponse
import maru.api.beacon.GetStateValidatorsResponse
import maru.api.node.SyncingStatusData
import net.consensys.linea.async.toSafeFuture
import org.http4k.client.DualSyncAsyncHttpHandler
import org.http4k.client.OkHttp
import org.http4k.core.Body
import org.http4k.core.Method
import org.http4k.core.Request
import org.http4k.core.Status.Companion.OK
import org.http4k.format.Jackson.auto
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class Genesis(
  val genesis_time: String,
  val genesis_validators_root: String,
  val genesis_fork_version: String,
)

// Interface extracted from previous concrete client methods
interface BeaconChainClient {
  // node operations
  fun getSyncingStatus(): SafeFuture<SyncingStatusData>

  // beacon chain operations
  fun getGenesis(): SafeFuture<Genesis>

  fun getValidators(stateId: String = "head"): SafeFuture<GetStateValidatorsResponse>

  fun getBlock(blockId: String): SafeFuture<GetBlockResponse>
}

class Http4kBeaconChainClient(
  private val baseUrl: String,
  private val client: DualSyncAsyncHttpHandler = OkHttp(),
) : BeaconChainClient {
  private data class DataEnvelop<T>(
    val data: T,
  )

  // Response lenses for parsing
  private val genesisLens = Body.auto<DataEnvelop<Genesis>>().toLens()
  private val syncingLens = Body.auto<DataEnvelop<SyncingStatusData>>().toLens()
  private val validatorsLens = Body.auto<GetStateValidatorsResponse>().toLens()
  private val getBlockLens = Body.auto<GetBlockResponse>().toLens()

  private fun <T> supply(block: () -> T): SafeFuture<T> = SafeFuture.supplyAsync(block).toSafeFuture()

  override fun getSyncingStatus(): SafeFuture<SyncingStatusData> =
    supply {
      val request = Request(Method.GET, "$baseUrl/eth/v1/node/syncing")
      val response = client(request)
      when (response.status) {
        OK -> syncingLens(response).data
        else -> throw Exception("Failed to get syncing status: ${response.status}")
      }
    }

  override fun getGenesis(): SafeFuture<Genesis> =
    supply {
      val request = Request(Method.GET, "$baseUrl/eth/v1/beacon/genesis")
      val response = client(request)
      when (response.status) {
        OK -> genesisLens(response).data
        else -> throw Exception("Failed to get genesis: ${response.status}")
      }
    }

  override fun getValidators(stateId: String): SafeFuture<GetStateValidatorsResponse> =
    supply {
      val request = Request(Method.GET, "$baseUrl/eth/v1/beacon/states/$stateId/validators")
      val response = client(request)
      when (response.status) {
        OK -> validatorsLens(response)
        else -> throw Exception("Failed to get validators: ${response.status}")
      }
    }

  override fun getBlock(blockId: String): SafeFuture<GetBlockResponse> =
    supply {
      val request = Request(Method.GET, "$baseUrl/eth/v2/beacon/blocks/$blockId")
      val response = client(request)
      when (response.status) {
        OK -> getBlockLens(response)
        else -> throw Exception("Failed to get block: ${response.status}")
      }
    }
}

// Usage example
fun main() {
  val beaconClient: BeaconChainClient = Http4kBeaconChainClient("http://localhost:25060")
  // Example async call (commented):
  beaconClient.getSyncingStatus().get().also { println(it) }
  beaconClient.getBlock("head").get().also { println(it) }
}
