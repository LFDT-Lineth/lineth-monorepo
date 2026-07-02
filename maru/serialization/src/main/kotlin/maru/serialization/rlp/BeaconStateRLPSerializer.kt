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
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Function-free (write-only) RLP serializer for [BeaconState]. Single source of truth for the state byte
 * layout, matching [BeaconStateSerDe.writeTo] exactly: the embedded header (delegated to
 * [beaconBlockHeaderRLPSerializer]) followed by the validators list. Carries no `headerHashFunction`, so it
 * can back [HashUtil.stateRoot] without a construction-order cycle.
 */
class BeaconStateRLPSerializer(
  private val beaconBlockHeaderRLPSerializer: BeaconBlockHeaderRLPSerializer,
  private val validatorSerializer: ValidatorSerDe,
) : RLPSerializer<BeaconState> {
  override fun writeTo(
    value: BeaconState,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    beaconBlockHeaderRLPSerializer.writeTo(value.beaconBlockHeader, rlpOutput)
    rlpOutput.writeList(value.validators) { validator, output ->
      validatorSerializer.writeTo(validator, output)
    }

    rlpOutput.endList()
  }
}
