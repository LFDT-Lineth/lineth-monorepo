/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.ExecutionPayload
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput
import java.math.BigInteger

class ExecutionPayloadSerDe : RLPSerDe<ExecutionPayload> {
  override fun writeTo(
    value: ExecutionPayload,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    rlpOutput.writeBytes(Bytes.wrap(value.parentHash))
    rlpOutput.writeBytes(Bytes.wrap(value.feeRecipient))
    rlpOutput.writeBytes(Bytes.wrap(value.stateRoot))
    rlpOutput.writeBytes(Bytes.wrap(value.receiptsRoot))
    rlpOutput.writeBytes(Bytes.wrap(value.logsBloom))
    rlpOutput.writeBytes(Bytes.wrap(value.prevRandao))
    rlpOutput.writeLong(value.blockNumber.toLong())
    rlpOutput.writeLong(value.gasLimit.toLong())
    rlpOutput.writeLong(value.gasUsed.toLong())
    rlpOutput.writeLong(value.timestamp.toLong())
    rlpOutput.writeBytes(Bytes.wrap(value.extraData))
    rlpOutput.writeBytes(Bytes.wrap(value.baseFeePerGas.toByteArray()))
    rlpOutput.writeBytes(Bytes.wrap(value.blockHash))
    rlpOutput.writeList(value.transactions) { transaction, output ->
      output.writeBytes(Bytes.wrap(transaction))
    }
    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): ExecutionPayload {
    rlpInput.enterList()
    val parentHash = rlpInput.readBytes().toArray()
    val feeRecipient = rlpInput.readBytes().toArray()
    val stateRoot = rlpInput.readBytes().toArray()
    val receiptsRoot = rlpInput.readBytes().toArray()
    val logsBloom = rlpInput.readBytes().toArray()
    val prevRandao = rlpInput.readBytes().toArray()
    val blockNumber = rlpInput.readLong().toULong()
    val gasLimit = rlpInput.readLong().toULong()
    val gasUsed = rlpInput.readLong().toULong()
    val timestamp = rlpInput.readLong().toULong()
    val extraData = rlpInput.readBytes().toArray()
    val baseFeePerGas = BigInteger(rlpInput.readBytes().toArray())
    val blockHash = rlpInput.readBytes().toArray()
    val transactions = rlpInput.readList { it.readBytes().toArray() }.toList()
    rlpInput.leaveList()
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
    )
  }
}
