/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.topics

import maru.serialization.rlp.RLPSerDe
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput
import org.hyperledger.besu.ethereum.p2p.rlpx.wire.MessageData as BesuMessageData

class BesuMessageDataSerDe : RLPSerDe<BesuMessageData> {
  override fun writeTo(
    value: BesuMessageData,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()
    rlpOutput.writeInt(value.code)
    rlpOutput.writeBytes(value.data)
    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BesuMessageData {
    rlpInput.enterList()
    val code = rlpInput.readInt()
    val data = rlpInput.readBytes()
    rlpInput.leaveList()
    return MaruMessageData(code, data)
  }

  internal data class MaruMessageData(
    private val code: Int,
    private val data: Bytes,
  ) : BesuMessageData {
    override fun getData(): Bytes = data

    override fun getSize(): Int = data.size()

    override fun getCode(): Int = code

    override fun toString(): String = "MessageData(code=$code, size=$size, data=$data)"
  }
}
