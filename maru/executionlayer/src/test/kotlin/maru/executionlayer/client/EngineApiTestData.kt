/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import linea.kotlin.encodeHex
import maru.consensus.ElFork
import maru.core.ExecutionPayload
import maru.executionlayer.manager.PayloadAttributes
import java.math.BigInteger

/**
 * Deterministic client inputs, and the Engine API JSON expected for them. The JSON is written out independently of the
 * client's codec, with quantities spelled out as literals.
 */
object EngineApiTestData {
  private val objectMapper = jacksonObjectMapper()
  private const val ZERO_HASH = "0x0000000000000000000000000000000000000000000000000000000000000000"

  fun bytes(size: Int, seed: Int): ByteArray = ByteArray(size) { (seed + it).toByte() }

  val headBlockHash = bytes(32, 14)
  val safeBlockHash = bytes(32, 15)
  val finalizedBlockHash = bytes(32, 16)
  val payloadId = bytes(8, 17)

  fun executionPayload(fork: ElFork): ExecutionPayload {
    val isAmsterdam = fork == ElFork.Amsterdam
    return ExecutionPayload(
      parentHash = bytes(32, 1),
      feeRecipient = bytes(20, 2),
      stateRoot = bytes(32, 3),
      receiptsRoot = bytes(32, 4),
      logsBloom = bytes(256, 5),
      prevRandao = bytes(32, 6),
      blockNumber = 100UL,
      gasLimit = 30_000_000UL,
      gasUsed = 21_000UL,
      timestamp = 1_700_000_000UL,
      extraData = bytes(4, 7),
      baseFeePerGas = BigInteger.valueOf(7),
      blockHash = bytes(32, 8),
      transactions = listOf(bytes(3, 9), bytes(2, 10)),
      blockAccessList = if (isAmsterdam) bytes(5, 11) else null,
      slotNumber = if (isAmsterdam) 42UL else null,
    )
  }

  fun payloadAttributes(fork: ElFork): PayloadAttributes {
    val isAmsterdam = fork == ElFork.Amsterdam
    return PayloadAttributes(
      timestamp = 1_700_000_002UL,
      prevRandao = bytes(32, 12),
      suggestedFeeRecipient = bytes(20, 13),
      slotNumber = if (isAmsterdam) 43UL else null,
      targetGasLimit = if (isAmsterdam) 60_000_000UL else null,
    )
  }

  private fun schemaVersion(fork: ElFork) = EngineApiForkMethods.of(fork).schemaVersion

  /** `ExecutionPayloadV1` to `ExecutionPayloadV4` JSON of [executionPayload] for [fork]. */
  fun executionPayloadJson(fork: ElFork): ObjectNode =
    objectMapper.createObjectNode().apply {
      put("parentHash", bytes(32, 1).encodeHex())
      put("feeRecipient", bytes(20, 2).encodeHex())
      put("stateRoot", bytes(32, 3).encodeHex())
      put("receiptsRoot", bytes(32, 4).encodeHex())
      put("logsBloom", bytes(256, 5).encodeHex())
      put("prevRandao", bytes(32, 6).encodeHex())
      put("blockNumber", "0x64")
      put("gasLimit", "0x1c9c380")
      put("gasUsed", "0x5208")
      put("timestamp", "0x6553f100")
      put("extraData", bytes(4, 7).encodeHex())
      put("baseFeePerGas", "0x7")
      put("blockHash", bytes(32, 8).encodeHex())
      putArray("transactions").add(bytes(3, 9).encodeHex()).add(bytes(2, 10).encodeHex())
      if (schemaVersion(fork) >= 2) putArray("withdrawals")
      if (schemaVersion(fork) >= 3) {
        put("blobGasUsed", "0x0")
        put("excessBlobGas", "0x0")
      }
      if (schemaVersion(fork) >= 4) {
        put("blockAccessList", bytes(5, 11).encodeHex())
        put("slotNumber", "0x2a")
      }
    }

  /** `PayloadAttributesV1` to `PayloadAttributesV4` JSON of [payloadAttributes] for [fork]. */
  private fun payloadAttributesJson(fork: ElFork): ObjectNode =
    objectMapper.createObjectNode().apply {
      put("timestamp", "0x6553f102")
      put("prevRandao", bytes(32, 12).encodeHex())
      put("suggestedFeeRecipient", bytes(20, 13).encodeHex())
      if (schemaVersion(fork) >= 2) putArray("withdrawals")
      if (schemaVersion(fork) >= 3) put("parentBeaconBlockRoot", ZERO_HASH)
      if (schemaVersion(fork) >= 4) {
        put("slotNumber", "0x2b")
        put("targetGasLimit", "0x3938700")
      }
    }

  private fun forkchoiceStateJson(): ObjectNode =
    objectMapper.createObjectNode().apply {
      put("headBlockHash", headBlockHash.encodeHex())
      put("safeBlockHash", safeBlockHash.encodeHex())
      put("finalizedBlockHash", finalizedBlockHash.encodeHex())
    }

  private fun request(method: String, params: ArrayNode.() -> Unit): JsonNode =
    objectMapper.createObjectNode().apply {
      put("jsonrpc", "2.0")
      put("method", method)
      putArray("params").params()
    }

  /** `engine_newPayload` request for [executionPayload], without the JSON-RPC `id`. */
  fun newPayloadRequest(fork: ElFork): JsonNode {
    val methods = EngineApiForkMethods.of(fork)
    return request(methods.newPayload) {
      add(executionPayloadJson(fork))
      if (methods.newPayloadVersion >= 3) {
        addArray()
        add(ZERO_HASH)
      }
      if (methods.newPayloadVersion >= 4) addArray()
    }
  }

  /** `engine_forkchoiceUpdated` request, with [payloadAttributes] when [withAttributes], without the JSON-RPC `id`. */
  fun forkChoiceUpdatedRequest(fork: ElFork, withAttributes: Boolean): JsonNode =
    request(EngineApiForkMethods.of(fork).forkChoiceUpdated) {
      add(forkchoiceStateJson())
      if (withAttributes) add(payloadAttributesJson(fork)) else addNull()
    }

  /** `engine_getPayload` request for [payloadId], without the JSON-RPC `id`. */
  fun getPayloadRequest(fork: ElFork): JsonNode =
    request(EngineApiForkMethods.of(fork).getPayload) { add(payloadId.encodeHex()) }

  /** `eth_getBlockByNumber` request for the latest block header, without the JSON-RPC `id`. */
  fun latestBlockRequest(): JsonNode =
    request("eth_getBlockByNumber") {
      add("latest")
      add(false)
    }

  /**
   * `engine_getPayload` result for [fork] carrying [executionPayloadJson]: the payload itself for V1, otherwise the
   * response object with the fields required by the method version.
   */
  fun getPayloadResultJson(fork: ElFork): ObjectNode {
    val version = EngineApiForkMethods.of(fork).getPayloadVersion
    val payload = executionPayloadJson(fork)
    if (version == 1) return payload
    return objectMapper.createObjectNode().apply {
      set<JsonNode>("executionPayload", payload)
      put("blockValue", "0x0")
      if (version >= 3) {
        putObject("blobsBundle").apply {
          putArray("commitments")
          putArray("proofs")
          putArray("blobs")
        }
        put("shouldOverrideBuilder", false)
      }
      if (version >= 4) putArray("executionRequests")
    }
  }

  fun withoutId(request: JsonNode): JsonNode = (request.deepCopy<JsonNode>() as ObjectNode).apply { remove("id") }
}
