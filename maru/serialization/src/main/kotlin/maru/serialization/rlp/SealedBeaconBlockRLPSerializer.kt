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
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Function-free (write-only) RLP serializer for [SealedBeaconBlock]. Single source of truth for the sealed
 * block byte layout, matching [SealedBeaconBlockSerDe.writeTo] exactly: the block (delegated to
 * [beaconBlockRLPSerializer]) followed by the commit seals list. Carries no `headerHashFunction`.
 */
class SealedBeaconBlockRLPSerializer(
  private val beaconBlockRLPSerializer: BeaconBlockRLPSerializer,
  private val sealSerializer: SealSerDe,
) : RLPSerializer<SealedBeaconBlock> {
  override fun writeTo(
    value: SealedBeaconBlock,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    beaconBlockRLPSerializer.writeTo(value.beaconBlock, rlpOutput)
    rlpOutput.writeList(value.commitSeals) { commitSeal, output ->
      sealSerializer.writeTo(commitSeal, output)
    }

    rlpOutput.endList()
  }
}
