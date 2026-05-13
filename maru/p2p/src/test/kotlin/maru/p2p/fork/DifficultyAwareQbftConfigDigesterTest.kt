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
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class DifficultyAwareQbftConfigDigesterTest {
  private val config =
    DifficultyAwareQbftConfig(
      QbftConsensusConfig(
        validatorSet = setOf(Validator("0x0000000000000000000000000000000000000001".decodeHex())),
        fork = ChainFork(
          clFork = ClFork.QBFT_PHASE0,
          elFork = ElFork.Paris,
        ),
      ),
      terminalTotalDifficulty = 1000uL,
    )

  private fun digest(config: DifficultyAwareQbftConfig): ByteArray = DifficultyAwareQbftConfigDigester.serialize(config)

  @Test
  fun `should take terminalTotalDifficulty into account`() {
    assertThat(digest(config.copy(terminalTotalDifficulty = 1001uL)))
      .isNotEqualTo(digest(config))
  }

  @Test
  fun `should digest deterministically`() {
    assertThat(digest(config.copy())).isEqualTo(digest(config))
  }
}
