/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import linea.kotlin.decodeHex
import maru.core.ExecutionPayload
import org.apache.tuweni.bytes.Bytes
import org.bouncycastle.math.ec.custom.sec.SecP256K1Curve
import org.hyperledger.besu.crypto.SECPSignature
import org.hyperledger.besu.datatypes.AccessListEntry
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.core.Transaction
import org.hyperledger.besu.ethereum.core.encoding.EncodingContext
import org.hyperledger.besu.ethereum.core.encoding.TransactionEncoder
import org.web3j.protocol.core.methods.response.EthBlock
import java.math.BigInteger

/** Maps eth API blocks to execution payloads, so tests can build on top of the blocks of a running EL node. */
object EthBlockMappers {
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
      parentHash = this.parentHash.decodeHex(),
      feeRecipient = this.miner.decodeHex(),
      stateRoot = this.stateRoot.decodeHex(),
      receiptsRoot = this.receiptsRoot.decodeHex(),
      logsBloom = this.logsBloom.decodeHex(),
      prevRandao = this.mixHash.decodeHex(),
      blockNumber = this.number.toLong().toULong(),
      gasLimit = this.gasLimit.toLong().toULong(),
      gasUsed = this.gasUsed.toLong().toULong(),
      timestamp = this.timestamp.toLong().toULong(),
      extraData = this.extraData.decodeHex(),
      baseFeePerGas = this.baseFeePerGas,
      blockHash = this.hash.decodeHex(),
      transactions = transactions,
    )
  }
}
