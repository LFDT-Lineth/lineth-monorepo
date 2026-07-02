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
 * Write-only RLP serializer for [SealedBeaconBlock]: the single source of truth for the sealed block byte
 * layout (the block, delegated to [beaconBlockRLPSerializer], followed by the commit seals list), matching
 * [SealedBeaconBlockSerDe.writeTo].
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
