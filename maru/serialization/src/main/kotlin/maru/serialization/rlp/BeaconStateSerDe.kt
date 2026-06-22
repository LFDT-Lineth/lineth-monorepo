/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.BeaconState
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class BeaconStateSerDe(
  private val beaconBlockHeaderSerializer: BeaconBlockHeaderSerDe,
  private val validatorSerializer: ValidatorSerDe,
) : RLPSerDe<BeaconState> {
  override fun writeTo(
    value: BeaconState,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    beaconBlockHeaderSerializer.writeTo(value.beaconBlockHeader, rlpOutput)
    rlpOutput.writeList(value.validators) { validator, output ->
      validatorSerializer.writeTo(validator, output)
    }

    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BeaconState {
    rlpInput.enterList()

    val latestBeaconBlockHeader = beaconBlockHeaderSerializer.readFrom(rlpInput)
    val validators = rlpInput.readList { validatorSerializer.readFrom(it) }.toSortedSet()

    rlpInput.leaveList()

    return BeaconState(
      beaconBlockHeader = latestBeaconBlockHeader,
      validators = validators,
    )
  }
}
