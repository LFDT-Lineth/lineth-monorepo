/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import maru.core.ExecutionPayload
import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.LatestBlockMetadata
import maru.executionlayer.manager.PayloadAttributes
import maru.executionlayer.manager.PayloadStatus

/**
 * Converts between Maru domain types and the Engine API JSON types of every supported fork, which [engineApiObjectMapper]
 * (de)serializes.
 *
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
      ForkchoiceStateJson(headBlockHash, safeBlockHash, finalizedBlockHash),
      payloadAttributes?.toJson(),
    )

  fun decodeGetPayloadResult(result: Any?): ExecutionPayload {
    val payload =
      if (methods.getPayloadVersion == 1) {
        result.decode<ExecutionPayloadJson>("getPayload")
      } else {
        result.decode<GetPayloadResponseJson>("getPayload").executionPayload
      }
    return payload.toDomain()
  }

  fun decodeNewPayloadResult(result: Any?): PayloadStatus = result.decode<PayloadStatusJson>("newPayload").toDomain()

  fun decodeForkChoiceUpdatedResult(result: Any?): ForkChoiceUpdatedResult {
    val response = result.decode<ForkChoiceUpdatedResultJson>("forkchoiceUpdated")
    return ForkChoiceUpdatedResult(payloadStatus = response.payloadStatus.toDomain(), payloadId = response.payloadId)
  }

  private fun ExecutionPayload.toJson(): ExecutionPayloadJson {
    val schemaVersion = methods.schemaVersion
    return ExecutionPayloadJson(
      parentHash = parentHash,
      feeRecipient = feeRecipient,
      stateRoot = stateRoot,
      receiptsRoot = receiptsRoot,
      logsBloom = logsBloom,
      prevRandao = prevRandao,
      blockNumber = blockNumber,
      gasLimit = gasLimit,
      gasUsed = gasUsed,
      timestamp = timestamp,
      extraData = extraData,
      baseFeePerGas = baseFeePerGas,
      blockHash = blockHash,
      transactions = transactions,
      withdrawals = if (schemaVersion >= 2) emptyList() else null,
      blobGasUsed = if (schemaVersion >= 3) 0UL else null,
      excessBlobGas = if (schemaVersion >= 3) 0UL else null,
      blockAccessList =
      if (schemaVersion >= 4) requireNotNull(blockAccessList) { "Amsterdam requires blockAccessList" } else null,
      slotNumber = if (schemaVersion >= 4) requireNotNull(slotNumber) { "Amsterdam requires slotNumber" } else null,
    )
  }

  private fun PayloadAttributes.toJson(): PayloadAttributesJson {
    val schemaVersion = methods.schemaVersion
    return PayloadAttributesJson(
      timestamp = timestamp,
      prevRandao = prevRandao,
      suggestedFeeRecipient = suggestedFeeRecipient,
      withdrawals = if (schemaVersion >= 2) emptyList() else null,
      parentBeaconBlockRoot = if (schemaVersion >= 3) ZERO_HASH else null,
      slotNumber = if (schemaVersion >= 4) requireNotNull(slotNumber) { "Amsterdam requires slotNumber" } else null,
      targetGasLimit =
      if (schemaVersion >= 4) requireNotNull(targetGasLimit) { "Amsterdam requires targetGasLimit" } else null,
    )
  }

  private fun ExecutionPayloadJson.toDomain(): ExecutionPayload {
    val hasAmsterdamFields = methods.schemaVersion >= 4
    return ExecutionPayload(
      parentHash = parentHash,
      feeRecipient = feeRecipient,
      stateRoot = stateRoot,
      receiptsRoot = receiptsRoot,
      logsBloom = logsBloom,
      prevRandao = prevRandao,
      blockNumber = blockNumber,
      gasLimit = gasLimit,
      gasUsed = gasUsed,
      timestamp = timestamp,
      extraData = extraData,
      baseFeePerGas = baseFeePerGas,
      blockHash = blockHash,
      transactions = transactions,
      blockAccessList =
      if (hasAmsterdamFields) requireNotNull(blockAccessList) { "Missing field 'blockAccessList'" } else null,
      slotNumber = if (hasAmsterdamFields) requireNotNull(slotNumber) { "Missing field 'slotNumber'" } else null,
    )
  }

  companion object {
    private const val PAYLOAD_ID_SIZE = 8
    private val ZERO_HASH = ByteArray(32)

    fun decodeLatestBlockMetadata(result: Any?): LatestBlockMetadata {
      val block = result.decode<BlockMetadataJson>("eth_getBlockByNumber")
      return LatestBlockMetadata(blockHash = block.hash, timestamp = block.timestamp)
    }

    private fun PayloadStatusJson.toDomain(): PayloadStatus =
      PayloadStatus(
        status = ExecutionPayloadStatus.entries.firstOrNull { it.name == status }
          ?: throw IllegalArgumentException("Unknown payload status '$status'"),
        latestValidHash = latestValidHash,
        validationError = validationError,
      )

    private inline fun <reified T> Any?.decode(context: String): T {
      require(this is JsonNode && this.isObject) { "Unexpected $context result: expected a JSON object but got $this" }
      return engineApiObjectMapper.treeToValue(this, T::class.java)
    }
  }
}
