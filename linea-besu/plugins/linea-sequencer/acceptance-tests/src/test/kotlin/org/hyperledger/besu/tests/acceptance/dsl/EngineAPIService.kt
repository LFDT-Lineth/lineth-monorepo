/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package org.hyperledger.besu.tests.acceptance.dsl

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.databind.node.ObjectNode
import okhttp3.Call
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.transaction.eth.EthTransactions
import org.web3j.crypto.BlobUtils
import java.util.function.Supplier

/*
 * Inspired by PragueAcceptanceTestHelper class in Besu codebase. We use this class to
 * emulate Engine API calls to the Besu Node, so that we can run tests for post-merge EVM forks.
 */
class EngineAPIService(
  private val node: BesuNode,
  private val ethTransactions: EthTransactions,
  private val mapper: ObjectMapper,
) {
  private val httpClient = OkHttpClient()

  companion object {
    private const val JSONRPC_VERSION = "2.0"
    private const val JSONRPC_REQUEST_ID = 67L
  }

  /*
   * See https://hackmd.io/@danielrachi/engine_api
   *
   * The flow to build a block with the Engine API is as follows:
   * 1. Send engine_forkchoiceUpdated(EngineForkchoiceUpdatedParameter, EnginePayloadAttributesParameter) request to Besu node
   * 2. Besu node responds with payloadId
   * The Besu Node will start building a proposed block
   *
   * 3. Send engine_getPayload(payloadId) request to Besu node
   * 4. Besu node responds with executionPayload
   * Get the proposed block from the Besu node
   *
   * 5. Send engine_newPayload request to Besu node
   * Validate the proposed block. Then store the validated block for future reference.
   * Unsure why the proposed block is not stored in the previous steps where it was built.
   *
   * 6. Send engine_forkchoiceUpdated(EngineForkchoiceUpdatedParameter) request to Besu node
   * Add validated block to blockchain head.
   *
   * @param blockTimestampSeconds    The Unix timestamp (in seconds) to assign to the new block.
   * @param blockBuildingTimeMs      The duration (in milliseconds) allocated for the Besu node to build the block.
   */
  @JvmOverloads
  fun buildNewBlock(
    blockTimestampSeconds: Long,
    blockBuildingTimeMs: Long,
    stopBlockBuilding: Supplier<Boolean> = Supplier { false },
  ) {
    val latestBlock = node.execute(ethTransactions.block())

    val buildBlockRequest = createForkChoiceRequest(latestBlock.hash, blockTimestampSeconds)

    val payloadId: String
    buildBlockRequest.execute().use { buildBlockResponse ->
      // Ideally, we would deserialize directly into Besu native types such as
      // EngineUpdateForkchoiceResult and JsonRpcSuccessResponse. However, neither class
      // provides a default constructor or a constructor annotated with @JsonCreator.
      // As a result, deserializing them would require hefty boilerplate code (custom
      // deserializers and DTOs). To keep things simple and lightweight, we instead
      // parse the relevant fields manually from the expected JSON structure.
      payloadId = mapper
        .readTree(buildBlockResponse.body!!.string())
        .get("result")
        .get("payloadId")
        .asText()
      assertThat(payloadId).isNotEmpty()
    }

    // This is required to give the Besu node time to build the block. As per the Engine API spec,
    // engine_forkChoice will begin the payload build process and engine_getPayload may stop the
    // payload build process. Besu node behaviour is to stop the payload build process on
    // engine_getPayload. So unfortunately we lack a means to inspect a payload in-building without
    // interrupting it. Hence we must be conservative and wait for the 'SECONDS_PER_SLOT' time,
    // especially for slower machines running the tests.
    // See - https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md
    Thread.sleep(blockBuildingTimeMs)

    val getPayloadRequest = createGetPayloadRequest(payloadId)

    val executionPayload: ObjectNode
    val blobsBundle: ObjectNode
    val executionRequests: ArrayNode
    val newBlockHash: String
    val parentBeaconBlockRoot = Hash.ZERO.bytes.toHexString()
    val expectedBlobVersionedHashes = mapper.createArrayNode()
    getPayloadRequest.execute().use { getPayloadResponse ->
      assertThat(getPayloadResponse.code).isEqualTo(200)
      val result = mapper.readTree(getPayloadResponse.body!!.string()).get("result")
      executionPayload = result.get("executionPayload") as ObjectNode
      blobsBundle = result.get("blobsBundle") as ObjectNode
      executionRequests = result.get("executionRequests") as ArrayNode
      newBlockHash = executionPayload.get("blockHash").asText()
      // Transform KZG commitments to versioned hashes
      for (kzgCommitment in blobsBundle.get("commitments")) {
        val kzgBytes = Bytes.fromHexString(kzgCommitment.asText())
        expectedBlobVersionedHashes.add(BlobUtils.kzgToVersionedHash(kzgBytes).toString())
      }
      assertThat(newBlockHash).isNotEmpty()
    }

    val newPayloadRequest = createNewPayloadRequestV4(
      executionPayload,
      expectedBlobVersionedHashes,
      parentBeaconBlockRoot,
      executionRequests,
    )

    newPayloadRequest.execute().use { newPayloadResponse ->
      assertThat(newPayloadResponse.code).isEqualTo(200)
      val responseStatus = mapper
        .readTree(newPayloadResponse.body!!.string())
        .get("result")
        .get("status")
        .asText()
      assertThat(responseStatus).isEqualTo("VALID")
    }

    val moveChainAheadRequest = createForkChoiceRequest(newBlockHash)
    if (stopBlockBuilding.get()) {
      // if stopBlockBuilding is true, we exit before moving the chain ahead
      return
    }
    moveChainAheadRequest.execute().use { moveChainAheadResponse ->
      assertThat(moveChainAheadResponse.code).isEqualTo(200)
    }
  }

  fun importPremadeBlock(
    executionPayload: ObjectNode,
    expectedBlobVersionedHashes: ArrayNode,
    parentBeaconBlockRoot: String,
    executionRequests: ArrayNode,
  ): Response {
    val newPayloadRequest = createNewPayloadRequestV4(
      executionPayload,
      expectedBlobVersionedHashes,
      parentBeaconBlockRoot,
      executionRequests,
    )

    return newPayloadRequest.execute()
  }

  private fun createForkChoiceRequest(blockHash: String): Call {
    return createForkChoiceRequest(blockHash, null)
  }

  private fun createForkChoiceRequest(parentBlockHash: String, blockTimestamp: Long?): Call {
    // Construct the first param - EngineForkchoiceUpdatedParameter
    val params = mapper.createArrayNode()
    val forkchoiceState = mapper.createObjectNode()
    forkchoiceState.put("headBlockHash", parentBlockHash)
    forkchoiceState.put("safeBlockHash", parentBlockHash)
    forkchoiceState.put("finalizedBlockHash", parentBlockHash)
    params.add(forkchoiceState)

    // Optionally construct the second param - EnginePayloadAttributesParameter
    if (blockTimestamp != null) {
      val payloadAttributes = mapper.createObjectNode()
      payloadAttributes.put("timestamp", blockTimestamp)
      payloadAttributes.put("prevRandao", Hash.ZERO.bytes.toHexString())
      payloadAttributes.put("suggestedFeeRecipient", Address.ZERO.bytes.toHexString())
      payloadAttributes.set<ArrayNode>("withdrawals", mapper.createArrayNode())
      payloadAttributes.put("parentBeaconBlockRoot", Hash.ZERO.bytes.toHexString())
      params.add(payloadAttributes)
    }
    return createEngineCall("engine_forkchoiceUpdatedV3", params)
  }

  private fun createGetPayloadRequest(payloadId: String): Call {
    val params = mapper.createArrayNode()
    params.add(payloadId)
    return createEngineCall("engine_getPayloadV5", params)
  }

  private fun createNewPayloadRequestV4(
    executionPayload: ObjectNode,
    expectedBlobVersionedHashes: ArrayNode,
    parentBeaconBlockRoot: String,
    executionRequests: ArrayNode,
  ): Call {
    val params = mapper.createArrayNode()
    params.add(executionPayload)
    params.add(expectedBlobVersionedHashes)
    params.add(parentBeaconBlockRoot)
    params.add(executionRequests)

    return createEngineCall("engine_newPayloadV4", params)
  }

  //   private fun createNewPayloadRequestV5(
  //     executionPayload: ObjectNode,
  //     expectedBlobVersionedHashes: ArrayNode,
  //     parentBeaconBlockRoot: String,
  //     executionRequests: ArrayNode): Call {
  //
  //   // Add blockAccessList to executionPayload (required for V5)
  //   // Use "0xc0" for empty BlockAccessList
  //   executionPayload.put("blockAccessList", "0xc0")
  //
  //   val params = mapper.createArrayNode()
  //   params.add(executionPayload)
  //   params.add(expectedBlobVersionedHashes)
  //   params.add(parentBeaconBlockRoot)
  //   params.add(executionRequests)
  //
  //   return createEngineCall("engine_newPayloadV5", params)
  // }

  private fun createEngineCall(rpcMethod: String, params: ArrayNode): Call {
    val request = mapper.createObjectNode()
    request.put("jsonrpc", JSONRPC_VERSION)
    request.put("method", rpcMethod)
    request.set<ArrayNode>("params", params)
    request.put("id", JSONRPC_REQUEST_ID)

    val requestString = try {
      mapper.writeValueAsString(request)
    } catch (e: Exception) {
      throw RuntimeException("Failed to serialize JSON-RPC request for method $rpcMethod:", e)
    }

    return httpClient.newCall(
      Request.Builder()
        .url(node.engineRpcUrl().get())
        .post(requestString.toRequestBody("application/json; charset=utf-8".toMediaType()))
        .build(),
    )
  }
}
