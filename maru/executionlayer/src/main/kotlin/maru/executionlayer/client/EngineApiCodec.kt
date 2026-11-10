/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import linea.kotlin.decodeHex
import linea.kotlin.toBigIntegerFromHex
import linea.kotlin.toULongFromHex
import maru.core.ExecutionPayload
import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.LatestBlockMetadata
import maru.executionlayer.manager.PayloadAttributes
import maru.executionlayer.manager.PayloadStatus

/**
 * Converts between Maru domain types and Engine API JSON for every supported fork.
 *
 * Request params are plain maps and lists; byte arrays and numbers are hex-encoded by the request object mapper.
 * Maru does not use blobs, withdrawals, beacon roots or execution requests, so these are always sent empty or zero.
 */
internal class EngineApiCodec(
  private val methods: EngineApiForkMethods,
) {
  fun getPayloadParams(payloadId: ByteArray): List<Any> {
    require(payloadId.size == PAYLOAD_ID_SIZE) { "payloadId must be $PAYLOAD_ID_SIZE bytes, got ${payloadId.size}" }
    return listOf(payloadId)
  }

  fun newPayloadParams(executionPayload: ExecutionPayload): List<Any> =
    buildList {
      add(executionPayload.toJson())
      if (methods.newPayloadVersion >= 3) {
        add(emptyList<ByteArray>()) // expectedBlobVersionedHashes
        add(ZERO_HASH) // parentBeaconBlockRoot
      }
      if (methods.newPayloadVersion >= 4) {
        add(emptyList<ByteArray>()) // executionRequests
      }
    }

  fun forkChoiceUpdatedParams(
    headBlockHash: ByteArray,
    safeBlockHash: ByteArray,
    finalizedBlockHash: ByteArray,
    payloadAttributes: PayloadAttributes?,
  ): List<Any?> =
    listOf(
      mapOf(
        "headBlockHash" to headBlockHash,
        "safeBlockHash" to safeBlockHash,
        "finalizedBlockHash" to finalizedBlockHash,
      ),
      payloadAttributes?.toJson(),
    )

  fun decodeGetPayloadResult(result: Any?): ExecutionPayload {
    val response = result.asJsonObject("getPayload")
    val payload = if (methods.getPayloadVersion == 1) response else response.requiredObject("executionPayload")
    return payload.toExecutionPayload()
  }

  fun decodeNewPayloadResult(result: Any?): PayloadStatus = result.asJsonObject("newPayload").toPayloadStatus()

  fun decodeForkChoiceUpdatedResult(result: Any?): ForkChoiceUpdatedResult {
    val response = result.asJsonObject("forkchoiceUpdated")
    return ForkChoiceUpdatedResult(
      payloadStatus = response.requiredObject("payloadStatus").toPayloadStatus(),
      payloadId = response.optionalText("payloadId")?.decodeHex(),
    )
  }

  private fun ExecutionPayload.toJson(): Map<String, Any> =
    buildMap {
      put("parentHash", parentHash)
      put("feeRecipient", feeRecipient)
      put("stateRoot", stateRoot)
      put("receiptsRoot", receiptsRoot)
      put("logsBloom", logsBloom)
      put("prevRandao", prevRandao)
      put("blockNumber", blockNumber)
      put("gasLimit", gasLimit)
      put("gasUsed", gasUsed)
      put("timestamp", timestamp)
      put("extraData", extraData)
      put("baseFeePerGas", baseFeePerGas)
      put("blockHash", blockHash)
      put("transactions", transactions)
      if (methods.schemaVersion >= 2) {
        put("withdrawals", emptyList<Any>())
      }
      if (methods.schemaVersion >= 3) {
        put("blobGasUsed", 0UL)
        put("excessBlobGas", 0UL)
      }
      if (methods.schemaVersion >= 4) {
        put("blockAccessList", requireNotNull(blockAccessList) { "Amsterdam requires blockAccessList" })
        put("slotNumber", requireNotNull(slotNumber) { "Amsterdam requires slotNumber" })
      }
    }

  private fun PayloadAttributes.toJson(): Map<String, Any> =
    buildMap {
      put("timestamp", timestamp)
      put("prevRandao", prevRandao)
      put("suggestedFeeRecipient", suggestedFeeRecipient)
      if (methods.schemaVersion >= 2) {
        put("withdrawals", emptyList<Any>())
      }
      if (methods.schemaVersion >= 3) {
        put("parentBeaconBlockRoot", ZERO_HASH)
      }
      if (methods.schemaVersion >= 4) {
        put("slotNumber", requireNotNull(slotNumber) { "Amsterdam requires slotNumber" })
        put("targetGasLimit", requireNotNull(targetGasLimit) { "Amsterdam requires targetGasLimit" })
      }
    }

  private fun JsonNode.toExecutionPayload(): ExecutionPayload {
    val hasAmsterdamFields = methods.schemaVersion >= 4
    return ExecutionPayload(
      parentHash = requiredBytes("parentHash"),
      feeRecipient = requiredBytes("feeRecipient"),
      stateRoot = requiredBytes("stateRoot"),
      receiptsRoot = requiredBytes("receiptsRoot"),
      logsBloom = requiredBytes("logsBloom"),
      prevRandao = requiredBytes("prevRandao"),
      blockNumber = requiredQuantity("blockNumber"),
      gasLimit = requiredQuantity("gasLimit"),
      gasUsed = requiredQuantity("gasUsed"),
      timestamp = requiredQuantity("timestamp"),
      extraData = requiredBytes("extraData"),
      baseFeePerGas = requiredText("baseFeePerGas").toBigIntegerFromHex(),
      blockHash = requiredBytes("blockHash"),
      transactions = requiredArray("transactions").map { it.asText().decodeHex() },
      blockAccessList = if (hasAmsterdamFields) requiredBytes("blockAccessList") else null,
      slotNumber = if (hasAmsterdamFields) requiredQuantity("slotNumber") else null,
    )
  }

  companion object {
    private const val PAYLOAD_ID_SIZE = 8
    private val ZERO_HASH = ByteArray(32)

    fun decodeLatestBlockMetadata(result: Any?): LatestBlockMetadata {
      val block = result.asJsonObject("eth_getBlockByNumber")
      return LatestBlockMetadata(
        blockHash = block.requiredBytes("hash"),
        timestamp = block.requiredQuantity("timestamp"),
      )
    }

    private fun JsonNode.toPayloadStatus(): PayloadStatus =
      PayloadStatus(
        status = requiredText("status").toExecutionPayloadStatus(),
        latestValidHash = optionalText("latestValidHash")?.decodeHex(),
        validationError = optionalText("validationError"),
      )

    private fun String.toExecutionPayloadStatus(): ExecutionPayloadStatus =
      ExecutionPayloadStatus.entries.firstOrNull { it.name == this }
        ?: throw IllegalArgumentException("Unknown payload status '$this'")

    private fun Any?.asJsonObject(context: String): JsonNode {
      require(this is JsonNode && this.isObject) { "Unexpected $context result: expected a JSON object but got $this" }
      return this
    }

    private fun JsonNode.requiredField(field: String): JsonNode =
      get(field)?.takeUnless { it.isNull } ?: throw IllegalArgumentException("Missing field '$field' in $this")

    private fun JsonNode.requiredObject(field: String): JsonNode =
      requiredField(field).also { require(it.isObject) { "Field '$field' is not a JSON object" } }

    private fun JsonNode.requiredArray(field: String): JsonNode =
      requiredField(field).also { require(it.isArray) { "Field '$field' is not a JSON array" } }

    private fun JsonNode.requiredText(field: String): String = requiredField(field).asText()

    private fun JsonNode.optionalText(field: String): String? = get(field)?.takeUnless { it.isNull }?.asText()

    private fun JsonNode.requiredBytes(field: String): ByteArray = requiredText(field).decodeHex()

    private fun JsonNode.requiredQuantity(field: String): ULong = requiredText(field).toULongFromHex()
  }
}
