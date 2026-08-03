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
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Write-only RLP serializer for [BeaconBlock]: the single source of truth for the block byte layout (header,
 * delegated to [beaconBlockHeaderRLPSerializer], followed by the body), matching [BeaconBlockSerDe.writeTo].
 * Serialization has no dependency on a header hash function, so this is exposed as a plain static instance
 * in [RLPSerializers].
 */
class BeaconBlockRLPSerializer(
  private val beaconBlockHeaderRLPSerializer: BeaconBlockHeaderRLPSerializer,
  private val beaconBlockBodySerializer: BeaconBlockBodySerDe,
) : RLPSerializer<BeaconBlock> {
  override fun writeTo(
    value: BeaconBlock,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    beaconBlockHeaderRLPSerializer.writeTo(value.beaconBlockHeader, rlpOutput)
    beaconBlockBodySerializer.writeTo(value.beaconBlockBody, rlpOutput)

    rlpOutput.endList()
  }
}
