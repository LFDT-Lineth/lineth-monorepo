/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import linea.kotlin.decodeHex
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random

class ForkSpecDigesterTest {
  private val qbftConfig =
    QbftConsensusConfig(
      validatorSet = setOf(Validator("0x0000000000000000000000000000000000000001".decodeHex())),
      fork = ChainFork(
        clFork = ClFork.QBFT_PHASE0,
        elFork = ElFork.Paris,
      ),
    )
  private val qbftTtdAwareConfig =
    DifficultyAwareQbftConfig(
      postTtdConfig = qbftConfig,
      terminalTotalDifficulty = 1000uL,
    )

  private val forkSpec =
    ForkSpec(
      timestampSeconds = 1000uL,
      blockTimeSeconds = 4U,
      configuration = qbftTtdAwareConfig,
    )

  private fun digest(spec: ForkSpec): ByteArray = ForkSpecDigester.serialize(spec)

  @Test
  fun `should ignore blockTime`() {
    assertThat(digest(forkSpec.copy(blockTimeSeconds = forkSpec.blockTimeSeconds + 1U)))
      .isEqualTo(digest(forkSpec))
  }

  @Test
  fun `should take timestamp into account`() {
    assertThat(digest(forkSpec.copy(timestampSeconds = forkSpec.timestampSeconds + 1U)))
      .isNotEqualTo(digest(forkSpec))
  }

  @Test
  fun `should take qbft config into account`() {
    val forkSpecQbftA = forkSpec.copy(configuration = qbftConfig)
    val forkSpecQbftB =
      forkSpec.copy(configuration = qbftConfig.copy(validatorSet = setOf(Validator(Random.nextBytes(20)))))
    assertThat(digest(forkSpecQbftA))
      .isNotEqualTo(digest(forkSpecQbftB))

    // assert consistency
    assertThat(digest(forkSpecQbftA))
      .isEqualTo(digest(forkSpecQbftA.copy()))
    assertThat(digest(forkSpecQbftB))
      .isEqualTo(digest(forkSpecQbftB.copy()))
  }

  @Test
  fun `should take ttd aware qbft config into account`() {
    // only changing terminalTotalDifficulty will gurantee correctenss
    // if we change postTtdConfig, then the digest will be different but
    // if implementation is refactored and delegates to QbftConsensusConfigDigester will have a bug
    val forkSpecQbftA = forkSpec.copy(configuration = qbftTtdAwareConfig)
    val forkSpecQbftB =
      forkSpec.copy(
        configuration = qbftTtdAwareConfig.copy(
          terminalTotalDifficulty = qbftTtdAwareConfig.terminalTotalDifficulty + 1uL,
        ),
      )
    assertThat(digest(forkSpecQbftA))
      .isNotEqualTo(digest(forkSpecQbftB))

    // assert consistency
    assertThat(digest(forkSpecQbftA))
      .isEqualTo(digest(forkSpecQbftA.copy()))
    assertThat(digest(forkSpecQbftB))
      .isEqualTo(digest(forkSpecQbftB.copy()))
  }
}
