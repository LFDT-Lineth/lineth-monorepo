/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ConsensusConfig
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

class MaruAppFactoryCheckEthApiTest {
  private fun baseQbftConfig() =
    QbftConsensusConfig(
      DataGenerators.randomValidators(),
      ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
    )

  private fun difficultyAware(base: QbftConsensusConfig) =
    DifficultyAwareQbftConfig(
      postTtdConfig = base,
      terminalTotalDifficulty = 42u,
    )

  private fun forkSpec(
    timestampSeconds: ULong,
    config: ConsensusConfig,
  ) = ForkSpec(
    timestampSeconds = timestampSeconds,
    blockTimeSeconds = 1u,
    configuration = config,
  )

  // Creates a fork Schedule with 2 forks. Timestamps are set as offsets relative to the `nowTs`
  private fun createForksSchedule(
    nowTs: ULong,
    currentOffsetSeconds: Long,
    futureOneOffsetSeconds: Long,
    futureOneIsDifficultyAware: Boolean,
  ): ForksSchedule {
    val firstTs = (nowTs.toLong() + currentOffsetSeconds).toULong()
    val secondTs = (nowTs.toLong() + futureOneOffsetSeconds).toULong()
    require(firstTs < secondTs) { "First timestamp has to be lower than the second one!" }

    val first = forkSpec(firstTs, baseQbftConfig())
    val second =
      forkSpec(
        secondTs,
        if (futureOneIsDifficultyAware) difficultyAware(baseQbftConfig()) else baseQbftConfig(),
      )

    return ForksSchedule(chainId = 1337u, forks = listOf(first, second))
  }

  private fun futureSchedule(
    nowTs: ULong,
    futureDifficultyAware: Boolean,
  ) = createForksSchedule(nowTs, 0L, 10L, futureDifficultyAware)

  private fun currentSchedule(
    nowTs: ULong,
    currentDifficultyAware: Boolean,
  ) = createForksSchedule(nowTs, -100L, 0L, currentDifficultyAware)

  @Test
  fun `throws when future DifficultyAwareQbft exists and l2EthWeb3j is null`() {
    val nowTs = 1_000_000UL
    val schedule = futureSchedule(nowTs = nowTs, futureDifficultyAware = true)
    val fixedClock = Clock.fixed(Instant.ofEpochSecond(nowTs.toLong()), ZoneOffset.UTC)

    assertThatThrownBy {
      MaruAppFactory().checkL2EthApiEndpointAndForks(fixedClock, schedule, null)
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("future fork enables DifficultyAwareQbft")
  }

  @Test
  fun `does not throw when future DifficultyAwareQbft exists in the future and l2EthWeb3j is provided`() {
    val nowTs = 1_000_000UL
    val schedule = futureSchedule(nowTs = nowTs, futureDifficultyAware = true)
    val fixedClock = Clock.fixed(Instant.ofEpochSecond(nowTs.toLong()), ZoneOffset.UTC)

    MaruAppFactory().checkL2EthApiEndpointAndForks(fixedClock, schedule, Any())
  }

  @Test
  fun `does not throw when no future DifficultyAwareQbft exists`() {
    val nowTs = 1_000_000UL
    val schedule = futureSchedule(nowTs = nowTs, futureDifficultyAware = false)
    val fixedClock = Clock.fixed(Instant.ofEpochSecond(nowTs.toLong()), ZoneOffset.UTC)

    MaruAppFactory().checkL2EthApiEndpointAndForks(fixedClock, schedule, null)
  }

  @Test
  fun `throws when current fork is DifficultyAwareQbft and l2EthWeb3j is null`() {
    val nowTs = 1_000_000UL
    val schedule = currentSchedule(nowTs = nowTs, currentDifficultyAware = true)
    val fixedClock = Clock.fixed(Instant.ofEpochSecond(nowTs.toLong()), ZoneOffset.UTC)

    assertThatThrownBy {
      MaruAppFactory().checkL2EthApiEndpointAndForks(fixedClock, schedule, null)
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("DifficultyAwareQbft")
  }

  @Test
  fun `does not throw when current fork is DifficultyAwareQbft and l2EthWeb3j is provided`() {
    val nowTs = 1_000_000UL
    val schedule = currentSchedule(nowTs = nowTs, currentDifficultyAware = true)
    val fixedClock = Clock.fixed(Instant.ofEpochSecond(nowTs.toLong()), ZoneOffset.UTC)

    MaruAppFactory().checkL2EthApiEndpointAndForks(fixedClock, schedule, Any())
  }
}
