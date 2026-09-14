/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.QbftConfig
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class MaruAppFactoryTargetGasLimitTest {
  private val factory = MaruAppFactory()
  private val qbftConfig = QbftConfig(feeRecipient = ByteArray(20))

  private fun schedule(vararg forks: Pair<ULong, ElFork>) =
    ForksSchedule(
      chainId = 1337u,
      forks = forks.map { (timestamp, elFork) ->
        ForkSpec(
          timestampSeconds = timestamp,
          blockTimeSeconds = 1u,
          configuration = QbftConsensusConfig(
            DataGenerators.randomValidators(),
            ChainFork(ClFork.QBFT_PHASE0, elFork),
          ),
        )
      },
    )

  @Test
  fun `Amsterdam producer requires explicit target at genesis`() {
    assertThatThrownBy {
      factory.checkTargetGasLimitAndForks(qbftConfig, schedule(0UL to ElFork.Amsterdam))
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("qbft.target-gas-limit must be configured")
  }

  @Test
  fun `producer requires target even before scheduled Amsterdam activation`() {
    assertThatThrownBy {
      factory.checkTargetGasLimitAndForks(
        qbftConfig,
        schedule(0UL to ElFork.Osaka, ULong.MAX_VALUE to ElFork.Amsterdam),
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("qbft.target-gas-limit must be configured")
  }

  @Test
  fun `Amsterdam producer accepts explicit target`() {
    factory.checkTargetGasLimitAndForks(
      qbftConfig.copy(targetGasLimit = 60_000_000UL),
      schedule(0UL to ElFork.Amsterdam),
    )
  }

  @Test
  fun `legacy producer does not require target`() {
    factory.checkTargetGasLimitAndForks(qbftConfig, schedule(0UL to ElFork.Osaka))
  }

  @Test
  fun `Amsterdam follower does not require qbft configuration`() {
    factory.checkTargetGasLimitAndForks(null, schedule(0UL to ElFork.Amsterdam))
  }
}
