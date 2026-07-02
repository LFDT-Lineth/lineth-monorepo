/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.ext.DataGenerators
import maru.serialization.rlp.ForkAwareBlockHashing
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class BeaconBlocksByRangeResponseSerDeTest {
  // Round-trip (deserialize) → the sealed block SerDe must inject a fork-aware header hash function.
  private val blockHashing =
    ForkAwareBlockHashing(
      ForksSchedule(
        chainId = 1u,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1u,
            configuration = QbftConsensusConfig(
              validatorSet = emptySet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
            ),
          ),
        ),
      ),
    )

  @Test
  fun `response serDe serializes and deserializes correctly`() {
    val serDe =
      BeaconBlocksByRangeResponseSerDe(
        blockHashing.sealedBeaconBlockSerializer,
      )

    val response =
      BeaconBlocksByRangeResponse(
        blocks = listOf(
          DataGenerators.randomSealedBeaconBlock(
            number = 5UL,
          ),
        ),
      )

    val serialized = serDe.serialize(response)
    val deserialized = serDe.deserialize(serialized)

    assertThat(deserialized).isEqualTo(response)
  }
}
