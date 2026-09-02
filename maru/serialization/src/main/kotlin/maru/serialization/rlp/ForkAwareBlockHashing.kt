/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.consensus.ClFork
import maru.consensus.ForksSchedule
import maru.core.BeaconBlockIdHashFunction
import maru.core.BeaconState
import maru.core.SealedBeaconBlock

/**
 * Builds the node's chain-identity hashing functions from its [ForksSchedule].
 *
 * The chain-identity hash ([beaconBlockIdHashFunction]) and the state root are round-inclusive (include `round`
 * and `proposer`) up to and including QBFT_PHASE0, and become round-independent from QBFT_PHASE1 onward, so
 * that multiple rounds/proposers for the same block content settle on the same identity. The active fork is
 * resolved from the header's own timestamp.
 *
 * This is separate from the round-inclusive primitives on [HashUtil] (`HashUtil.headerHash`/`HashUtil.stateRoot`),
 * which remain used for commit seals and the QBFT engine (see `SCEP256SealVerifier` and
 * `QbftBlockHeaderAdapter`) regardless of fork.
 */
class ForkAwareBlockHashing(
  private val forksSchedule: ForksSchedule,
) {
  private fun clForkAt(timestamp: ULong): ClFork = forksSchedule.getForkByTimestamp(timestamp).configuration.fork.clFork

  private fun isRoundIndependent(timestamp: ULong): Boolean =
    clForkAt(timestamp).version >= ClFork.QBFT_PHASE1.version

  /**
   * Fork-aware chain-identity hash. This is the header hash function a node injects into every header it
   * builds or deserializes, so the header's `beaconBlockIdHash` carries the fork-appropriate identity
   * (round-inclusive through QBFT_PHASE0, round-independent from QBFT_PHASE1).
   */
  val beaconBlockIdHashFunction: BeaconBlockIdHashFunction = { header ->
    if (isRoundIndependent(header.timestamp)) {
      HashUtil.roundIndependentHeaderHash(header)
    } else {
      HashUtil.headerHash(header)
    }
  }

  /**
   * Fork-aware state root, parallel to [beaconBlockIdHashFunction]: zeroes `round`/`proposer` in the embedded
   * header before hashing the state from QBFT_PHASE1 onward.
   */
  fun stateRoot(beaconState: BeaconState): ByteArray =
    if (isRoundIndependent(beaconState.beaconBlockHeader.timestamp)) {
      HashUtil.roundIndependentStateRoot(beaconState)
    } else {
      HashUtil.stateRoot(beaconState)
    }

  /**
   * Node-scoped serializers that deserialize headers (and anything containing a header) with
   * [beaconBlockIdHashFunction]. They reuse the shared function-free write-only serializers for the byte layout, so
   * serialized bytes are identical to the static [RLPSerializers]; only the read-side injected hash function
   * differs.
   */
  val beaconBlockHeaderSerializer: BeaconBlockHeaderSerDe =
    BeaconBlockHeaderSerDe(
      beaconBlockHeaderRLPSerializer = RLPSerializers.BeaconBlockHeaderRLPSerializer,
      validatorSerializer = RLPSerializers.ValidatorSerializer,
      beaconBlockIdHashFunction = beaconBlockIdHashFunction,
    )

  val beaconBlockSerializer: BeaconBlockSerDe =
    BeaconBlockSerDe(
      beaconBlockRLPSerializer = RLPSerializers.BeaconBlockRLPSerializer,
      beaconBlockHeaderSerializer = beaconBlockHeaderSerializer,
      beaconBlockBodySerializer = RLPSerializers.BeaconBlockBodySerializer,
    )

  val sealedBeaconBlockSerializer: SealedBeaconBlockSerDe =
    SealedBeaconBlockSerDe(
      sealedBeaconBlockRLPSerializer = RLPSerializers.SealedBeaconBlockRLPSerializer,
      beaconBlockSerializer = beaconBlockSerializer,
      sealSerializer = RLPSerializers.SealSerializer,
    )

  val sealedBeaconBlockCompressorSerializer: MaruCompressorRLPSerDe<SealedBeaconBlock> =
    MaruCompressorRLPSerDe(serDe = sealedBeaconBlockSerializer)

  val beaconStateSerializer: BeaconStateSerDe =
    BeaconStateSerDe(
      beaconStateRLPSerializer = RLPSerializers.BeaconStateRLPSerializer,
      beaconBlockHeaderSerializer = beaconBlockHeaderSerializer,
      validatorSerializer = RLPSerializers.ValidatorSerializer,
    )
}
