/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.mappers

import maru.core.ExecutionPayload
import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.PayloadAttributes
import maru.executionlayer.manager.PayloadStatus
import maru.extensions.fromHexToByteArray
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.apache.tuweni.units.bigints.UInt256
import org.bouncycastle.math.ec.custom.sec.SecP256K1Curve
import org.hyperledger.besu.crypto.SECPSignature
import org.hyperledger.besu.datatypes.AccessListEntry
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.core.Transaction
import org.hyperledger.besu.ethereum.core.encoding.EncodingContext
import org.hyperledger.besu.ethereum.core.encoding.TransactionEncoder
import org.web3j.protocol.core.methods.response.EthBlock
import tech.pegasys.teku.ethereum.executionclient.schema.ExecutionPayloadV1
import tech.pegasys.teku.ethereum.executionclient.schema.ExecutionPayloadV2
import tech.pegasys.teku.ethereum.executionclient.schema.ExecutionPayloadV3
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV1
import tech.pegasys.teku.infrastructure.bytes.Bytes20
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import java.math.BigInteger
import kotlin.jvm.optionals.getOrNull
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceUpdatedResult as TekuForkChoiceUpdatedResult
import tech.pegasys.teku.spec.executionlayer.PayloadStatus as TekuPayloadStatus

object Mappers {
  private fun recIdFromV(v: BigInteger): Pair<Byte, BigInteger?> {
    val recId: Byte
    var chainId: BigInteger? = null
    if (
      v == Transaction.REPLAY_UNPROTECTED_V_BASE ||
      v == Transaction.REPLAY_UNPROTECTED_V_BASE_PLUS_1
    ) {
      recId = v.subtract(Transaction.REPLAY_UNPROTECTED_V_BASE).byteValueExact()
    } else if (v > Transaction.REPLAY_PROTECTED_V_MIN) {
      chainId = v.subtract(Transaction.REPLAY_PROTECTED_V_BASE).divide(Transaction.TWO)
      recId = v.subtract(Transaction.TWO.multiply(chainId).add(Transaction.REPLAY_PROTECTED_V_BASE)).byteValueExact()
    } else {
      throw RuntimeException("An unsupported encoded `v` value of $v was found")
    }
    return Pair(recId, chainId)
  }

  // TODO: Test
  private fun EthBlock.TransactionObject.toByteArray(): ByteArray {
    val isFrontier = this.type == "0x0"
    val (recId, chainId) =
      if (isFrontier) {
        recIdFromV(this.v.toBigInteger())
      } else {
        Pair(this.v.toByte(), BigInteger.valueOf(this.chainId))
      }
    val signature =
      SECPSignature.create(
        BigInteger(this.r.removePrefix("0x"), 16),
        BigInteger(this.s.removePrefix("0x"), 16),
        recId,
        SecP256K1Curve().order,
      )

    val transaction =
      Transaction
        .builder()
        .nonce(this.nonce.toLong())
        .also { builder ->
          if (isFrontier || this.type == "0x1") {
            builder.gasPrice(Wei.of(this.gasPrice))
          } else {
            builder.maxPriorityFeePerGas(Wei.of(this.maxPriorityFeePerGas))
            builder.maxFeePerGas(Wei.of(this.maxFeePerGas))
          }
        }.gasLimit(this.gas.toLong())
        .to(Address.fromHexString(this.to))
        .value(Wei.of(this.value))
        .signature(signature)
        .payload(Bytes.fromHexString(this.input))
        .also { builder ->
          this.accessList?.also { accessList ->
            builder.accessList(
              accessList.map { entry ->
                AccessListEntry.createAccessListEntry(
                  Address.fromHexString(entry.address),
                  entry.storageKeys,
                )
              },
            )
          }
        }.sender(Address.fromHexString(this.from))
        .apply {
          if (chainId != null) {
            chainId(chainId)
          }
        }.build()

    return TransactionEncoder.encodeOpaqueBytes(transaction, EncodingContext.BLOCK_BODY).toArray()
  }

  fun EthBlock.Block.toDomain(): ExecutionPayload {
    val transactions =
      this.transactions.map {
        val transaction = it.get() as EthBlock.TransactionObject
        transaction.toByteArray()
      }

    return ExecutionPayload(
      parentHash = this.parentHash.fromHexToByteArray(),
      feeRecipient = this.miner.fromHexToByteArray(),
      stateRoot = this.stateRoot.fromHexToByteArray(),
      receiptsRoot = this.receiptsRoot.fromHexToByteArray(),
      logsBloom = this.logsBloom.fromHexToByteArray(),
      prevRandao = this.mixHash.fromHexToByteArray(),
      blockNumber = this.number.toLong().toULong(),
      gasLimit = this.gasLimit.toLong().toULong(),
      gasUsed = this.gasUsed.toLong().toULong(),
      timestamp = this.timestamp.toLong().toULong(),
      extraData = this.extraData.fromHexToByteArray(),
      baseFeePerGas = this.baseFeePerGas,
      blockHash = this.hash.fromHexToByteArray(),
      transactions = transactions, // Transactions are omitted
    )
  }

  fun ExecutionPayloadV3.toDomainExecutionPayload() =
    ExecutionPayload(
      parentHash = this.parentHash.toArray(),
      feeRecipient = this.feeRecipient.wrappedBytes.toArray(),
      stateRoot = this.stateRoot.toArray(),
      receiptsRoot = this.receiptsRoot.toArray(),
      logsBloom = this.logsBloom.toArray(),
      prevRandao = this.prevRandao.toArray(),
      blockNumber = this.blockNumber.longValue().toULong(),
      gasLimit = this.gasLimit.longValue().toULong(),
      gasUsed = this.gasUsed.longValue().toULong(),
      timestamp = this.timestamp.longValue().toULong(),
      extraData = this.extraData.toArray(),
      baseFeePerGas = this.baseFeePerGas.toBigInteger(),
      blockHash = this.blockHash.toArray(),
      transactions = this.transactions.map { it.toArray() },
    )

  fun ExecutionPayloadV1.toDomainExecutionPayload() =
    ExecutionPayload(
      parentHash = this.parentHash.toArray(),
      feeRecipient = this.feeRecipient.wrappedBytes.toArray(),
      stateRoot = this.stateRoot.toArray(),
      receiptsRoot = this.receiptsRoot.toArray(),
      logsBloom = this.logsBloom.toArray(),
      prevRandao = this.prevRandao.toArray(),
      blockNumber = this.blockNumber.longValue().toULong(),
      gasLimit = this.gasLimit.longValue().toULong(),
      gasUsed = this.gasUsed.longValue().toULong(),
      timestamp = this.timestamp.longValue().toULong(),
      extraData = this.extraData.toArray(),
      baseFeePerGas = this.baseFeePerGas.toBigInteger(),
      blockHash = this.blockHash.toArray(),
      transactions = this.transactions.map { it.toArray() },
    )

  fun ExecutionPayload.toExecutionPayloadV3() =
    ExecutionPayloadV3(
      /* parentHash */
      Bytes32.wrap(this.parentHash),
      /* feeRecipient */
      Bytes20(Bytes.wrap(this.feeRecipient)),
      /* stateRoot */
      Bytes32.wrap(this.stateRoot),
      /* receiptsRoot */
      Bytes32.wrap(this.receiptsRoot),
      /* logsBloom */
      Bytes.wrap(this.logsBloom),
      /* prevRandao */
      Bytes32.wrap(this.prevRandao),
      /* blockNumber */
      UInt64.valueOf(this.blockNumber.toString()),
      /* gasLimit */
      UInt64.valueOf(this.gasLimit.toString()),
      /* gasUsed */
      UInt64.valueOf(this.gasUsed.toString()),
      /* timestamp */
      UInt64.valueOf(this.timestamp.toString()),
      /* extraData */
      Bytes.wrap(this.extraData),
      /* baseFeePerGas */
      UInt256.valueOf(this.baseFeePerGas),
      /* blockHash */
      Bytes32.wrap(this.blockHash),
      /* transactions */
      this.transactions.map { Bytes.wrap(it) },
      /* withdrawals */
      emptyList(),
      /* blobGasUsed */
      UInt64.ZERO,
      /* excessBlobGas */
      UInt64.ZERO,
    )

  fun ExecutionPayload.toExecutionPayloadV2() =
    ExecutionPayloadV2(
      /* parentHash */
      Bytes32.wrap(this.parentHash),
      /* feeRecipient */
      Bytes20(Bytes.wrap(this.feeRecipient)),
      /* stateRoot */
      Bytes32.wrap(this.stateRoot),
      /* receiptsRoot */
      Bytes32.wrap(this.receiptsRoot),
      /* logsBloom */
      Bytes.wrap(this.logsBloom),
      /* prevRandao */
      Bytes32.wrap(this.prevRandao),
      /* blockNumber */
      UInt64.valueOf(this.blockNumber.toString()),
      /* gasLimit */
      UInt64.valueOf(this.gasLimit.toString()),
      /* gasUsed */
      UInt64.valueOf(this.gasUsed.toString()),
      /* timestamp */
      UInt64.valueOf(this.timestamp.toString()),
      /* extraData */
      Bytes.wrap(this.extraData),
      /* baseFeePerGas */
      UInt256.valueOf(this.baseFeePerGas),
      /* blockHash */
      Bytes32.wrap(this.blockHash),
      /* transactions */
      this.transactions.map { Bytes.wrap(it) },
      /* withdrawals */
      emptyList(),
    )

  fun ExecutionPayload.toExecutionPayloadV1() =
    ExecutionPayloadV1(
      /* parentHash */
      Bytes32.wrap(this.parentHash),
      /* feeRecipient */
      Bytes20(Bytes.wrap(this.feeRecipient)),
      /* stateRoot */
      Bytes32.wrap(this.stateRoot),
      /* receiptsRoot */
      Bytes32.wrap(this.receiptsRoot),
      /* logsBloom */
      Bytes.wrap(this.logsBloom),
      /* prevRandao */
      Bytes32.wrap(this.prevRandao),
      /* blockNumber */
      UInt64.valueOf(this.blockNumber.toString()),
      /* gasLimit */
      UInt64.valueOf(this.gasLimit.toString()),
      /* gasUsed */
      UInt64.valueOf(this.gasUsed.toString()),
      /* timestamp */
      UInt64.valueOf(this.timestamp.toString()),
      /* extraData */
      Bytes.wrap(this.extraData),
      /* baseFeePerGas */
      UInt256.valueOf(this.baseFeePerGas),
      /* blockHash */
      Bytes32.wrap(this.blockHash),
      /* transactions */
      this.transactions.map { Bytes.wrap(it) },
    )

  fun PayloadAttributes.toPayloadAttributesV1(): PayloadAttributesV1 =
    PayloadAttributesV1(
      UInt64.fromLongBits(this.timestamp.toLong()),
      Bytes32.wrap(this.prevRandao),
      Bytes20(Bytes.wrap(this.suggestedFeeRecipient)),
    )

  fun TekuPayloadStatus.toDomain(): PayloadStatus =
    PayloadStatus(
      ExecutionPayloadStatus.valueOf(this.status.getOrNull().toString()), // TODO: Fix and test
      this.latestValidHash.getOrNull()?.toArray(),
      validationError.getOrNull(),
    )

  fun TekuForkChoiceUpdatedResult.toDomain(): ForkChoiceUpdatedResult {
    val payload = this.asInternalExecutionPayload()
    val parsedPayloadId =
      payload.payloadId
        .getOrNull()
        ?.wrappedBytes
        ?.toArray()
    return ForkChoiceUpdatedResult(payload.payloadStatus.toDomain(), parsedPayloadId)
  }
}
