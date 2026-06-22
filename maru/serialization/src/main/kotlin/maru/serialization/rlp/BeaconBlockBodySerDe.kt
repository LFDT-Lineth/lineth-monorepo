/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.BeaconBlockBody
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class BeaconBlockBodySerDe(
  private val sealSerializer: SealSerDe,
  private val executionPayloadSerializer: ExecutionPayloadSerDe,
) : RLPSerDe<BeaconBlockBody> {
  override fun writeTo(
    value: BeaconBlockBody,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()

    rlpOutput.writeList(value.prevCommitSeals) { prevBlockSeal, output ->
      sealSerializer.writeTo(prevBlockSeal, output)
    }

    executionPayloadSerializer.writeTo(value.executionPayload, rlpOutput)

    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): BeaconBlockBody {
    rlpInput.enterList()

    val prevCommitSeals = rlpInput.readList { sealSerializer.readFrom(rlpInput) }.toSet()
    val executionPayload = executionPayloadSerializer.readFrom(rlpInput)

    rlpInput.leaveList()

    return BeaconBlockBody(prevCommitSeals, executionPayload)
  }
}
