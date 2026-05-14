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
import maru.consensus.ElFork
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class QbftConsensusConfigDigesterTest {
  private val config =
    QbftConsensusConfig(
      validatorSet = setOf(Validator("0x0000000000000000000000000000000000000001".decodeHex())),
      fork = ChainFork(
        clFork = ClFork.QBFT_PHASE0,
        elFork = ElFork.Paris,
      ),
    )

  private fun digest(config: QbftConsensusConfig): ByteArray = QbftConsensusConfigDigester.serialize(config)

  @Test
  fun `should take elFork into account`() {
    assertThat(digest(config.copy(fork = config.fork.copy(elFork = ElFork.Prague))))
      .isNotEqualTo(digest(config))
  }

  @Test
  fun `should take clFork into account`() {
    assertThat(digest(config.copy(fork = config.fork.copy(clFork = ClFork.QBFT_PHASE1))))
      .isNotEqualTo(digest(config))
  }

  @Test
  fun `should digest deterministically`() {
    val validatorSetOrderA =
      setOf(
        Validator("0x0000000000000000000000000000000000000001".decodeHex()),
        Validator("0x0000000000000000000000000000000000000002".decodeHex()),
      )
    val validatorSetOrderB =
      setOf(
        Validator("0x0000000000000000000000000000000000000002".decodeHex()),
        Validator("0x0000000000000000000000000000000000000001".decodeHex()),
      )
    assertThat(digest(config.copy(validatorSet = validatorSetOrderA)))
      .isEqualTo(digest(config.copy(validatorSet = validatorSetOrderB)))
  }
}
