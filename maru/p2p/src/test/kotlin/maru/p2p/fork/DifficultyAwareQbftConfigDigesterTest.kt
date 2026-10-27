/*
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
import java.nio.ByteBuffer

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

  @Test
  fun `should keep a stable byte layout`() {
    // Layout: terminalTotalDifficulty (8 bytes) followed by postTtdConfig bytes.
    // Must stay byte-identical so fork ids of existing networks (e.g. Linea mainnet/sepolia) don't change.
    val postTtdConfigBytes = QbftConsensusConfigDigester.serialize(config.postTtdConfig)
    val legacyBytes =
      ByteBuffer
        .allocate(8 + postTtdConfigBytes.size)
        .putLong(config.terminalTotalDifficulty.toLong())
        .put(postTtdConfigBytes)
        .array()

    assertThat(digest(config)).isEqualTo(legacyBytes)
  }
}
