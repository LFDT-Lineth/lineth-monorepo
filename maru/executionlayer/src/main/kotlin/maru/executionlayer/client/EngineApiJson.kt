/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.annotation.JsonInclude
import com.fasterxml.jackson.databind.DeserializationFeature
import com.fasterxml.jackson.databind.ObjectMapper
import linea.s11n.jackson.ethApiObjectMapper
import linea.s11n.jackson.ethByteAsHexDeserialisersModule
import linea.s11n.jackson.ethNumberAsHexDeserialisersModule
import java.math.BigInteger

/**
 * Engine API JSON object mapper: byte arrays and numbers are hex-encoded, fields unknown to Maru are ignored.
 */
internal val engineApiObjectMapper: ObjectMapper =
  ethApiObjectMapper
    .copy()
    .registerModules(ethByteAsHexDeserialisersModule, ethNumberAsHexDeserialisersModule)
    .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)

/**
 * `ExecutionPayloadV1` to `ExecutionPayloadV4`. Fields added by later versions are null, and so omitted, for earlier
 * forks.
 */
@JsonInclude(JsonInclude.Include.NON_NULL)
internal class ExecutionPayloadJson(
  val parentHash: ByteArray,
  val feeRecipient: ByteArray,
  val stateRoot: ByteArray,
  val receiptsRoot: ByteArray,
  val logsBloom: ByteArray,
  val prevRandao: ByteArray,
  val blockNumber: ULong,
  val gasLimit: ULong,
  val gasUsed: ULong,
  val timestamp: ULong,
  val extraData: ByteArray,
  val baseFeePerGas: BigInteger,
  val blockHash: ByteArray,
  val transactions: List<ByteArray>,
  val withdrawals: List<Any>? = null,
  val blobGasUsed: ULong? = null,
  val excessBlobGas: ULong? = null,
  val blockAccessList: ByteArray? = null,
  val slotNumber: ULong? = null,
)

/** `getPayload` V2+ response envelope; V1 returns the payload itself. */
internal class GetPayloadResponseJson(
  val executionPayload: ExecutionPayloadJson,
)

internal class ForkchoiceStateJson(
  val headBlockHash: ByteArray,
  val safeBlockHash: ByteArray,
  val finalizedBlockHash: ByteArray,
)

/** `PayloadAttributesV1` to `PayloadAttributesV4`, with later version fields omitted for earlier forks. */
@JsonInclude(JsonInclude.Include.NON_NULL)
internal class PayloadAttributesJson(
  val timestamp: ULong,
  val prevRandao: ByteArray,
  val suggestedFeeRecipient: ByteArray,
  val withdrawals: List<Any>? = null,
  val parentBeaconBlockRoot: ByteArray? = null,
  val slotNumber: ULong? = null,
  val targetGasLimit: ULong? = null,
)

internal class PayloadStatusJson(
  val status: String,
  val latestValidHash: ByteArray? = null,
  val validationError: String? = null,
)

internal class ForkChoiceUpdatedResultJson(
  val payloadStatus: PayloadStatusJson,
  val payloadId: ByteArray? = null,
)

internal class BlockMetadataJson(
  val hash: ByteArray,
  val timestamp: ULong,
)
