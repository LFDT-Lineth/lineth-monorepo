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
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Function-free (write-only) RLP serializer for [BeaconBlockHeader]. This is the single source of truth for
 * the header byte layout. It carries NO `headerHashFunction`, so it can back [HashUtil] without the
 * construction-order cycle that a full [BeaconBlockHeaderSerDe] introduces (a SerDe needs a hash function for
 * its read side, and the default hash function is defined in terms of hashing the header).
 */
class BeaconBlockHeaderRLPSerializer(
  private val validatorSerializer: ValidatorSerDe,
) : RLPSerializer<BeaconBlockHeader> {
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
}
