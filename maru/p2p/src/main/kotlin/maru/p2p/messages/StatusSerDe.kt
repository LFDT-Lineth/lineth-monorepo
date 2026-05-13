/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.serialization.rlp.RLPSerDe
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class StatusSerDe : RLPSerDe<Status> {
  override fun writeTo(
    value: Status,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    rlpOutput.writeBytes(Bytes.wrap(value.forkIdHash))
    rlpOutput.writeBytes(Bytes.wrap(value.latestStateRoot))
    rlpOutput.writeLong(value.latestBlockNumber.toLong())

    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): Status {
    rlpInput.enterList()

    val forkId = rlpInput.readBytes().toArray()
    val headStateRoot = rlpInput.readBytes().toArray()
    val headBlockNumber = rlpInput.readLong().toULong()

    rlpInput.leaveList()

    return Status(
      forkIdHash = forkId,
      latestStateRoot = headStateRoot,
      latestBlockNumber = headBlockNumber,
    )
  }
}
