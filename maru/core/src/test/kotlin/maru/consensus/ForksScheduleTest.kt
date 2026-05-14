/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.Validator
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import kotlin.random.Random

class ForksScheduleTest {
  private val consensusConfig =
    object : ConsensusConfig {
      override val fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague)
    }
  private val qbftConsensusConfig =
    object : ConsensusConfig {
      override val fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai)
    }
  private val otherConsensusConfig =
    object : ConsensusConfig {
      override val fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun)
    }
  private val expectedChainId = 1337u

  @Test
  fun `throws exception on duplicate timestamps`() {
    val fork1 = ForkSpec(timestampSeconds = 1UL, blockTimeSeconds = 1u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 1UL, blockTimeSeconds = 2u, configuration = consensusConfig)
    val fork3 = ForkSpec(timestampSeconds = 3UL, blockTimeSeconds = 3u, configuration = consensusConfig)
    val forks = listOf(fork1, fork2, fork3)

    val exception = assertThrows<IllegalArgumentException> { ForksSchedule(expectedChainId, forks) }
    assertThat(exception).hasMessageContaining("Fork timestamps must be unique")
  }

  @Test
  fun `test getForkByTimestamp returns correct fork`() {
    val fork1 = ForkSpec(timestampSeconds = 1UL, blockTimeSeconds = 1u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 3UL, blockTimeSeconds = 2u, configuration = consensusConfig)
    val fork3 = ForkSpec(timestampSeconds = 5UL, blockTimeSeconds = 3u, configuration = consensusConfig)
    val forks = listOf(fork1, fork2, fork3)

    val schedule = ForksSchedule(expectedChainId, forks)

    assertThat(schedule.getForkByTimestamp(1UL)).isEqualTo(fork1)
    assertThat(schedule.getForkByTimestamp(2UL)).isEqualTo(fork1)
    assertThat(schedule.getForkByTimestamp(3UL)).isEqualTo(fork2)
    assertThat(schedule.getForkByTimestamp(4UL)).isEqualTo(fork2)
    assertThat(schedule.getForkByTimestamp(5UL)).isEqualTo(fork3)
    assertThat(schedule.getForkByTimestamp(6UL)).isEqualTo(fork3)
    assertThat(schedule.getForkByTimestamp(700UL)).isEqualTo(fork3)
  }

  @Test
  fun `test getPreviousForkByTimestamp returns correct fork`() {
    val fork1 = ForkSpec(timestampSeconds = 1UL, blockTimeSeconds = 1u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 3UL, blockTimeSeconds = 2u, configuration = consensusConfig)
    val fork3 = ForkSpec(timestampSeconds = 5UL, blockTimeSeconds = 3u, configuration = consensusConfig)
    val fork4 = ForkSpec(timestampSeconds = 7UL, blockTimeSeconds = 3u, configuration = consensusConfig)
    val forks = listOf(fork1, fork2, fork3, fork4)

    val schedule = ForksSchedule(expectedChainId, forks)

    assertThat(schedule.getPreviousForkByTimestamp(1UL)).isNull()
    assertThat(schedule.getPreviousForkByTimestamp(2UL)).isNull()
    assertThat(schedule.getPreviousForkByTimestamp(3UL)).isEqualTo(fork1)
    assertThat(schedule.getPreviousForkByTimestamp(4UL)).isEqualTo(fork1)
    assertThat(schedule.getPreviousForkByTimestamp(5UL)).isEqualTo(fork2)
    assertThat(schedule.getPreviousForkByTimestamp(6UL)).isEqualTo(fork2)
    assertThat(schedule.getPreviousForkByTimestamp(7UL)).isEqualTo(fork3)
    assertThat(schedule.getPreviousForkByTimestamp(600UL)).isEqualTo(fork3)
  }

  @Test
  fun `test getNextForkByTimestamp returns correct fork`() {
    val fork1 = ForkSpec(timestampSeconds = 1UL, blockTimeSeconds = 1u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 3UL, blockTimeSeconds = 2u, configuration = consensusConfig)
    val fork3 = ForkSpec(timestampSeconds = 5UL, blockTimeSeconds = 3u, configuration = consensusConfig)
    val fork4 = ForkSpec(timestampSeconds = 7UL, blockTimeSeconds = 3u, configuration = consensusConfig)
    val forks = listOf(fork1, fork2, fork3, fork4)

    val schedule = ForksSchedule(expectedChainId, forks)

    assertThat(schedule.getNextForkByTimestamp(1UL)).isEqualTo(fork2)
    assertThat(schedule.getNextForkByTimestamp(2UL)).isEqualTo(fork2)
    assertThat(schedule.getNextForkByTimestamp(3UL)).isEqualTo(fork3)
    assertThat(schedule.getNextForkByTimestamp(4UL)).isEqualTo(fork3)
    assertThat(schedule.getNextForkByTimestamp(5UL)).isEqualTo(fork4)
    assertThat(schedule.getNextForkByTimestamp(6UL)).isEqualTo(fork4)
    assertThat(schedule.getNextForkByTimestamp(7UL)).isNull()
    assertThat(schedule.getNextForkByTimestamp(800UL)).isNull()
  }

  @Test
  fun `getForkByTimestamp throws if timestamp is before all forks`() {
    val fork1 = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 2000UL, blockTimeSeconds = 20u, configuration = consensusConfig)
    val forks = listOf(fork1, fork2)

    val schedule = ForksSchedule(expectedChainId, forks)

    val exception =
      assertThrows<IllegalArgumentException> {
        schedule.getForkByTimestamp(500UL)
      }
    assertThat(exception).hasMessageContaining("No fork found")
  }

  @Test
  fun `ForkSpec initialization with invalid blockTimeSeconds`() {
    val exception =
      assertThrows<IllegalArgumentException> {
        ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 0u, configuration = consensusConfig)
      }
    assertThat(exception).hasMessage("blockTimeSeconds must be greater or equal to 1 second")
  }

  @Test
  fun `getForkByConfigType throws exception when config class not found`() {
    val qbftFork = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = qbftConsensusConfig)
    val forks = listOf(qbftFork)

    val schedule = ForksSchedule(expectedChainId, forks)

    val exception =
      assertThrows<IllegalArgumentException> {
        schedule.getForkByConfigType(otherConsensusConfig::class)
      }
    assertThat(exception).hasMessageContaining("No fork found for config type")
  }

  @Test
  fun `getForkByConfigType returns first matching fork`() {
    val otherFork1 = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = consensusConfig)
    val qbftFork1 = ForkSpec(timestampSeconds = 2000UL, blockTimeSeconds = 20u, configuration = qbftConsensusConfig)
    val otherFork2 = ForkSpec(timestampSeconds = 3000UL, blockTimeSeconds = 30u, configuration = consensusConfig)
    val qbftFork2 = ForkSpec(timestampSeconds = 4000UL, blockTimeSeconds = 40u, configuration = qbftConsensusConfig)
    val qbftFork3 = ForkSpec(timestampSeconds = 5000UL, blockTimeSeconds = 50u, configuration = qbftConsensusConfig)
    val forks = listOf(otherFork1, qbftFork1, otherFork2, qbftFork2, qbftFork3)

    val schedule = ForksSchedule(expectedChainId, forks)

    // Should return the first one found (note: forks are sorted by timestamp in reverse order internally)
    val result = schedule.getForkByConfigType(qbftConsensusConfig::class)
    assertThat(result).isEqualTo(qbftFork1)
  }

  @Test
  fun `getNextForkByTimestamp returns next fork when one exists`() {
    val fork1 = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 2000UL, blockTimeSeconds = 20u, configuration = qbftConsensusConfig)
    val fork3 = ForkSpec(timestampSeconds = 3000UL, blockTimeSeconds = 30u, configuration = otherConsensusConfig)
    val forks = listOf(fork1, fork2, fork3)

    val schedule = ForksSchedule(expectedChainId, forks)

    // Test getting next fork from before first fork
    assertThat(schedule.getNextForkByTimestamp(500UL)).isEqualTo(fork1)

    // Test getting next fork from first fork timestamp
    assertThat(schedule.getNextForkByTimestamp(1000UL)).isEqualTo(fork2)

    // Test getting next fork from between first and second fork
    assertThat(schedule.getNextForkByTimestamp(1500UL)).isEqualTo(fork2)

    // Test getting next fork from second fork timestamp
    assertThat(schedule.getNextForkByTimestamp(2000UL)).isEqualTo(fork3)

    // Test getting next fork from between second and third fork
    assertThat(schedule.getNextForkByTimestamp(2500UL)).isEqualTo(fork3)
  }

  @Test
  fun `getNextForkByTimestamp returns null when no next fork exists`() {
    val fork1 = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 2000UL, blockTimeSeconds = 20u, configuration = qbftConsensusConfig)
    val fork3 = ForkSpec(timestampSeconds = 3000UL, blockTimeSeconds = 30u, configuration = otherConsensusConfig)
    val forks = listOf(fork1, fork2, fork3)

    val schedule = ForksSchedule(expectedChainId, forks)

    // Test getting next fork from last fork timestamp
    assertThat(schedule.getNextForkByTimestamp(3000UL)).isNull()

    // Test getting next fork from after last fork
    assertThat(schedule.getNextForkByTimestamp(4000UL)).isNull()
  }

  @Test
  fun `getNextForkByTimestamp works with single fork`() {
    val fork1 = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = consensusConfig)
    val forks = listOf(fork1)

    val schedule = ForksSchedule(expectedChainId, forks)

    // Test getting next fork from before the only fork
    assertThat(schedule.getNextForkByTimestamp(500UL)).isEqualTo(fork1)

    // Test getting next fork from the only fork timestamp
    assertThat(schedule.getNextForkByTimestamp(1000UL)).isNull()

    // Test getting next fork from after the only fork
    assertThat(schedule.getNextForkByTimestamp(1500UL)).isNull()
  }

  @Test
  fun `getNextForkByTimestamp with edge case timestamps`() {
    val fork1 = ForkSpec(timestampSeconds = 1000UL, blockTimeSeconds = 10u, configuration = consensusConfig)
    val fork2 = ForkSpec(timestampSeconds = 2000UL, blockTimeSeconds = 20u, configuration = qbftConsensusConfig)
    val forks = listOf(fork1, fork2)

    val schedule = ForksSchedule(expectedChainId, forks)

    // Test with timestamp exactly one less than first fork
    assertThat(schedule.getNextForkByTimestamp(999UL)).isEqualTo(fork1)

    // Test with timestamp exactly one more than first fork
    assertThat(schedule.getNextForkByTimestamp(1001UL)).isEqualTo(fork2)

    // Test with timestamp exactly one less than second fork
    assertThat(schedule.getNextForkByTimestamp(1999UL)).isEqualTo(fork2)

    // Test with timestamp exactly one more than second fork
    assertThat(schedule.getNextForkByTimestamp(2001UL)).isNull()
  }

  @Nested
  inner class EqualsAndHashCode {
    val validators = setOf(Validator(Random.nextBytes(20)))
    val difficultyAwareQbftFork =
      ForkSpec(
        timestampSeconds = 1000UL,
        blockTimeSeconds = 10u,
        configuration = DifficultyAwareQbftConfig(
          postTtdConfig = QbftConsensusConfig(
            validatorSet = validators,
            fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
          ),
          terminalTotalDifficulty = 1000UL,
        ),
      )
    val qbftFork =
      ForkSpec(
        timestampSeconds = 2000UL,
        blockTimeSeconds = 10u,
        configuration = QbftConsensusConfig(
          validatorSet = validators,
          fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
        ),
      )

    @Test
    fun `equals should return false with different consensus config`() {
      val schedule1 = ForksSchedule(expectedChainId, listOf(qbftFork))
      val schedule1Instance2 = ForksSchedule(expectedChainId, listOf(qbftFork))
      val schedule2 = ForksSchedule(expectedChainId, listOf(difficultyAwareQbftFork))
      val schedule2Instance2 = ForksSchedule(expectedChainId, listOf(difficultyAwareQbftFork))

      assertThat(schedule1).isNotEqualTo(schedule2)
      assertThat(schedule1).isEqualTo(schedule1Instance2)
      assertThat(schedule2).isEqualTo(schedule2Instance2)

      assertThat(schedule1.hashCode()).isNotEqualTo(schedule2.hashCode())
      assertThat(schedule1.hashCode()).isEqualTo(schedule1Instance2.hashCode())
      assertThat(schedule2.hashCode()).isEqualTo(schedule2Instance2.hashCode())
    }

    @Test
    fun `equals should return take multiple forks into correct order`() {
      val forkList =
        listOf(
          difficultyAwareQbftFork,
          qbftFork.copy(timestampSeconds = difficultyAwareQbftFork.timestampSeconds + 1000UL),
        )
      val forkList2 =
        listOf(
          difficultyAwareQbftFork,
          qbftFork.copy(
            timestampSeconds = difficultyAwareQbftFork.timestampSeconds + 1000UL,
            configuration = QbftConsensusConfig(
              validatorSet = validators,
              fork = ChainFork(ClFork.QBFT_PHASE1, ElFork.Prague),
            ),
          ),
        )

      val schedule1 = ForksSchedule(expectedChainId, forkList)
      val schedule1Instance2 = ForksSchedule(expectedChainId, forkList.reversed())
      val schedule2 = ForksSchedule(expectedChainId, forkList2)
      val schedule2Instance2 = ForksSchedule(expectedChainId, forkList2.reversed())

      assertThat(schedule1).isNotEqualTo(schedule2)
      assertThat(schedule1).isEqualTo(schedule1Instance2)
      assertThat(schedule2).isEqualTo(schedule2Instance2)

      assertThat(schedule1.hashCode()).isNotEqualTo(schedule2.hashCode())
      assertThat(schedule1.hashCode()).isEqualTo(schedule1Instance2.hashCode())
      assertThat(schedule2.hashCode()).isEqualTo(schedule2Instance2.hashCode())
    }
  }
}
