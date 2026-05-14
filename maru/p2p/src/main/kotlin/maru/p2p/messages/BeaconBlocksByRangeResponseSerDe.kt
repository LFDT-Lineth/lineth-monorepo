/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.core.SealedBeaconBlock
import maru.serialization.rlp.RLPSerDe
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class BeaconBlocksByRangeResponseSerDe(
  private val sealedBeaconBlockSerDe: RLPSerDe<SealedBeaconBlock>,
) : RLPSerDe<BeaconBlocksByRangeResponse> {
  override fun writeTo(
    value: BeaconBlocksByRangeResponse,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()
    rlpOutput.writeList(value.blocks) { block, output ->
      sealedBeaconBlockSerDe.writeTo(block, output)
    }
    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BeaconBlocksByRangeResponse {
    rlpInput.enterList()
    val blocks = rlpInput.readList { sealedBeaconBlockSerDe.readFrom(it) }
    rlpInput.leaveList()
    return BeaconBlocksByRangeResponse(blocks)
  }
}
