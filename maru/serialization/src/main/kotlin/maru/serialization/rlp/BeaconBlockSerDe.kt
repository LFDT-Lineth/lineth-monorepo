/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.BeaconBlock
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class BeaconBlockSerDe(
  private val beaconBlockHeaderSerializer: BeaconBlockHeaderSerDe,
  private val beaconBlockBodySerializer: BeaconBlockBodySerDe,
) : RLPSerDe<BeaconBlock> {
  override fun writeTo(
    value: BeaconBlock,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    beaconBlockHeaderSerializer.writeTo(value.beaconBlockHeader, rlpOutput)
    beaconBlockBodySerializer.writeTo(value.beaconBlockBody, rlpOutput)

    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BeaconBlock {
    rlpInput.enterList()

    val beaconBLockHeader = beaconBlockHeaderSerializer.readFrom(rlpInput)
    val beaconBlockBody = beaconBlockBodySerializer.readFrom(rlpInput)

    rlpInput.leaveList()

    return BeaconBlock(beaconBLockHeader, beaconBlockBody)
  }
}
