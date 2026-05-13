/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.serialization.SerDe
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLP
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import java.nio.charset.StandardCharsets

class RpcExceptionSerDe : SerDe<RpcException> {
  override fun serialize(value: RpcException): ByteArray =
    RLP
      .encode { rlpOutput ->
        writeTo(value, rlpOutput)
      }.toArray()

  override fun deserialize(bytes: ByteArray): RpcException = readFrom(RLP.input(Bytes.wrap(bytes)))

  private fun writeTo(
    value: RpcException,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()
    rlpOutput.writeByte(value.responseCode)
    rlpOutput.writeBytes(Bytes.wrap(value.errorMessage.toString().toByteArray(StandardCharsets.UTF_8)))
    rlpOutput.endList()
  }

  private fun readFrom(rlpInput: RLPInput): RpcException {
    rlpInput.enterList()
    val responseCode = rlpInput.readByte()
    val errorMessage = rlpInput.readBytes().toArray().toString(StandardCharsets.UTF_8)
    rlpInput.leaveList()

    return RpcException(responseCode, errorMessage)
  }
}
