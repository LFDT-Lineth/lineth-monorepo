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
import maru.core.BeaconBlockHeader
import maru.core.BeaconState
import maru.core.Validator
import maru.crypto.Hashing
import maru.serialization.Serializer

object HashUtil {
  private fun <T> rootHash(
    t: T,
    serializer: Serializer<T>,
  ): ByteArray = Hashing.keccak(serializer.serialize(t))

  fun headerHash(header: BeaconBlockHeader): ByteArray = rootHash(header, RLPSerializers.BeaconBlockHeaderRLPSerializer)

  fun roundIndependentHeaderHash(header: BeaconBlockHeader): ByteArray =
    rootHash(header.copy(round = 0u, proposer = Validator.ZERO), RLPSerializers.BeaconBlockHeaderRLPSerializer)

  fun bodyRoot(body: BeaconBlockBody): ByteArray = rootHash(body, RLPSerializers.BeaconBlockBodySerializer)

  fun stateRoot(state: BeaconState): ByteArray = rootHash(state, RLPSerializers.BeaconStateRLPSerializer)

  fun roundIndependentStateRoot(state: BeaconState): ByteArray =
    rootHash(
      state.copy(beaconBlockHeader = state.beaconBlockHeader.copy(round = 0u, proposer = Validator.ZERO)),
      RLPSerializers.BeaconStateRLPSerializer,
    )
}
