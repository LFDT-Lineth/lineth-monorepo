/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.SealedBeaconBlock
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class SealedBeaconBlockSerDe(
  private val beaconBlockSerializer: BeaconBlockSerDe,
  private val sealSerializer: SealSerDe,
) : RLPSerDe<SealedBeaconBlock> {
  override fun writeTo(
    value: SealedBeaconBlock,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    beaconBlockSerializer.writeTo(value.beaconBlock, rlpOutput)
    rlpOutput.writeList(value.commitSeals) { commitSeal, output ->
      sealSerializer.writeTo(commitSeal, output)
    }

    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): SealedBeaconBlock {
    rlpInput.enterList()

    val beaconBlock = beaconBlockSerializer.readFrom(rlpInput)
    val commitSeals = rlpInput.readList { sealSerializer.readFrom(rlpInput) }.toSet()

    rlpInput.leaveList()

    return SealedBeaconBlock(beaconBlock, commitSeals)
  }
}
