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
import maru.core.BeaconBlockIdHashFunction
import org.hyperledger.besu.ethereum.rlp.RLPInput

class BeaconBlockHeaderSerDe(
  beaconBlockHeaderRLPSerializer: BeaconBlockHeaderRLPSerializer,
  private val validatorSerializer: ValidatorSerDe,
  private val beaconBlockIdHashFunction: BeaconBlockIdHashFunction,
) : RLPSerDe<BeaconBlockHeader>,
  RLPSerializer<BeaconBlockHeader> by beaconBlockHeaderRLPSerializer {
  override fun readFrom(rlpInput: RLPInput): BeaconBlockHeader {
    rlpInput.enterList()

    val number = rlpInput.readLong().toULong()
    val round = rlpInput.readInt().toUInt()
    val timestamp = rlpInput.readLong().toULong()
    val proposer = validatorSerializer.readFrom(rlpInput)
    val parentRoot = rlpInput.readBytes().toArray()
    val stateRoot = rlpInput.readBytes().toArray()
    val bodyRoot = rlpInput.readBytes().toArray()

    rlpInput.leaveList()

    return BeaconBlockHeader(
      number = number,
      round = round,
      timestamp = timestamp,
      proposer = proposer,
      parentRoot = parentRoot,
      stateRoot = stateRoot,
      bodyRoot = bodyRoot,
      beaconBlockIdHashFunction = beaconBlockIdHashFunction,
    )
  }
}
