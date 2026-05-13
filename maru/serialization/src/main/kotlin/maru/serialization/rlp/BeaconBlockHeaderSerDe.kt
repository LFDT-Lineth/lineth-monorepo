/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.BeaconBlockHeader
import maru.core.Hasher
import maru.core.HeaderHashFunction
import maru.serialization.SerDe
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class BeaconBlockHeaderSerDe(
  private val validatorSerializer: ValidatorSerDe,
  private val hasher: Hasher,
  private val headerHashFunction: (SerDe<BeaconBlockHeader>, Hasher) -> HeaderHashFunction,
) : RLPSerDe<BeaconBlockHeader> {
  override fun writeTo(
    value: BeaconBlockHeader,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    rlpOutput.writeLong(value.number.toLong())
    rlpOutput.writeInt(value.round.toInt())
    rlpOutput.writeLong(value.timestamp.toLong())
    validatorSerializer.writeTo(value.proposer, rlpOutput)
    rlpOutput.writeBytes(Bytes.wrap(value.parentRoot))
    rlpOutput.writeBytes(Bytes.wrap(value.stateRoot))
    rlpOutput.writeBytes(Bytes.wrap(value.bodyRoot))

    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BeaconBlockHeader {
    rlpInput.enterList()

    val number = rlpInput.readLong().toULong()
    val round = rlpInput.readInt().toUInt()
    val timestamp = rlpInput.readLong().toULong()
    val proposer = validatorSerializer.readFrom(rlpInput)
    val parentRoot = rlpInput.readBytes().toArray()
    val stateRoot = rlpInput.readBytes().toArray()
    val bodyRoot = rlpInput.readBytes().toArray()

    rlpInput.leaveList()

    return BeaconBlockHeader(
      number = number,
      round = round,
      timestamp = timestamp,
      proposer = proposer,
      parentRoot = parentRoot,
      stateRoot = stateRoot,
      bodyRoot = bodyRoot,
      headerHashFunction(this, hasher),
    )
  }
}
