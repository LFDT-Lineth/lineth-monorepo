/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import maru.consensus.ElFork
import maru.core.ExecutionPayload
import maru.executionlayer.manager.PayloadAttributes
import java.math.BigInteger

/**
 * Deterministic inputs matching the golden request fixtures in `engine-api/teku-requests`, which were captured from
 * the Teku/Web3J based Engine API client.
 */
object EngineApiTestData {
  private val objectMapper = jacksonObjectMapper()

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

  /** Request captured from the Teku client, without the JSON-RPC `id`. */
  fun tekuRequest(fork: ElFork, call: String): JsonNode {
    val resource = "/engine-api/teku-requests/${fork.name.lowercase()}-$call.json"
    val json =
      requireNotNull(EngineApiTestData::class.java.getResource(resource)) { "missing fixture $resource" }.readText()
    return objectMapper.readTree(json)
  }

  /** The execution payload JSON object exactly as the Teku client sent it in `engine_newPayload` for [fork]. */
  fun executionPayloadJson(fork: ElFork): ObjectNode = tekuRequest(fork, "newPayload")["params"][0].deepCopy()

  /** A spec-shaped `engine_getPayload` result for [fork], wrapping [executionPayloadJson] in an envelope from V2. */
  fun getPayloadResultJson(fork: ElFork): JsonNode {
    val payload = executionPayloadJson(fork)
    if (EngineApiForkMethods.of(fork).getPayloadVersion == 1) return payload
    return objectMapper.createObjectNode().apply {
      set<JsonNode>("executionPayload", payload)
      put("blockValue", "0x0")
      putObject("blobsBundle").apply {
        putArray("commitments")
        putArray("proofs")
        putArray("blobs")
      }
      put("shouldOverrideBuilder", false)
      putArray("executionRequests")
    }
  }

  fun withoutId(request: JsonNode): JsonNode = (request.deepCopy<JsonNode>() as ObjectNode).apply { remove("id") }
}
