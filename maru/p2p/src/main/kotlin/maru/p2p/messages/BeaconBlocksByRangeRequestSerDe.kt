/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.serialization.rlp.RLPSerDe
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class BeaconBlocksByRangeRequestSerDe : RLPSerDe<BeaconBlocksByRangeRequest> {
  override fun writeTo(
    value: BeaconBlocksByRangeRequest,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()
    rlpOutput.writeLong(value.startBlockNumber.toLong())
    rlpOutput.writeLong(value.count.toLong())
    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BeaconBlocksByRangeRequest {
    rlpInput.enterList()
    val startBlockNumber = rlpInput.readLong().toULong()
    val count = rlpInput.readLong().toULong()
    rlpInput.leaveList()

    return BeaconBlocksByRangeRequest(
      startBlockNumber = startBlockNumber,
      count = count,
    )
  }
}
