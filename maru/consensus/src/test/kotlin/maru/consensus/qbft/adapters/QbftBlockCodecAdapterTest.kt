/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.ext.DataGenerators
import maru.serialization.rlp.ForkAwareBlockHashing
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.ethereum.rlp.RLP
import org.junit.jupiter.api.Test

class QbftBlockCodecAdapterTest {
  // Decoding a proposal block must inject a fork-aware header hash function, so the codec is fed the
  // node's fork-aware serializer (PHASE0 fork at timestamp 0 → round-inclusive test identity).
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
  fun `can encode and decode same value`() {
    val beaconBlock = DataGenerators.randomBeaconBlock(
      10U,
    )
    val testValue = QbftBlockAdapter(beaconBlock)
    val qbftBlockCodecAdapter = QbftBlockCodecAdapter(blockHashing.beaconBlockSerializer)

    val encodedData = RLP.encode { rlpOutput -> qbftBlockCodecAdapter.writeTo(testValue, rlpOutput) }
    val decodedValue = qbftBlockCodecAdapter.readFrom(RLP.input(encodedData))
    assertThat(decodedValue).isEqualTo(testValue)
  }
}
