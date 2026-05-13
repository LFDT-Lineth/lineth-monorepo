/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import java.math.BigInteger

/**
 * Execution Payload for the Engine API and Beacon Block
 * https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md#executionpayloadv1
 */
data class ExecutionPayload(
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
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ExecutionPayload

    if (!parentHash.contentEquals(other.parentHash)) return false
    if (!stateRoot.contentEquals(other.stateRoot)) return false
    if (!receiptsRoot.contentEquals(other.receiptsRoot)) return false
    if (!logsBloom.contentEquals(other.logsBloom)) return false
    if (!prevRandao.contentEquals(other.prevRandao)) return false
    if (blockNumber != other.blockNumber) return false
    if (gasLimit != other.gasLimit) return false
    if (gasUsed != other.gasUsed) return false
    if (timestamp != other.timestamp) return false
    if (!extraData.contentEquals(other.extraData)) return false
    if (baseFeePerGas != other.baseFeePerGas) return false
    if (!blockHash.contentEquals(other.blockHash)) return false
    if (!transactions.zip(other.transactions).all { it.first.contentEquals(it.second) }) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentHash.contentHashCode()
    result = 31 * result + stateRoot.contentHashCode()
    result = 31 * result + receiptsRoot.contentHashCode()
    result = 31 * result + logsBloom.contentHashCode()
    result = 31 * result + prevRandao.contentHashCode()
    result = 31 * result + blockNumber.hashCode()
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + gasUsed.hashCode()
    result = 31 * result + timestamp.hashCode()
    result = 31 * result + extraData.contentHashCode()
    result = 31 * result + baseFeePerGas.hashCode()
    result = 31 * result + blockHash.contentHashCode()
    result = 31 * result + transactions.hashCode()
    return result
  }
}

val GENESIS_EXECUTION_PAYLOAD =
  ExecutionPayload(
    parentHash = EMPTY_HASH,
    feeRecipient = EMPTY_HASH.copyOf(20), // 20 bytes for address
    stateRoot = EMPTY_HASH,
    receiptsRoot = EMPTY_HASH,
    logsBloom = ByteArray(256), // Ethereum logs bloom is 256 bytes
    prevRandao = EMPTY_HASH,
    blockNumber = 0UL,
    gasLimit = 0UL,
    gasUsed = 0UL,
    timestamp = 0UL,
    extraData = EMPTY_HASH,
    baseFeePerGas = BigInteger.ZERO,
    blockHash = EMPTY_HASH,
    transactions = emptyList(),
  )
