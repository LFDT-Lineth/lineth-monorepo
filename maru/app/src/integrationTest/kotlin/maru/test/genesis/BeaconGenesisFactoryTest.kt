/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.genesis

import linea.kotlin.decodeHex
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class BeaconGenesisFactoryTest {
  private val validatorAddresses =
    listOf(
      "0x0000000000000000000000000000000000000001".decodeHex(),
    )
  private val validators = validatorAddresses.map { Validator(it) }
  private lateinit var factory: MaruGenesisFactory

  @BeforeEach
  fun setUp() {
    factory = MaruGenesisFactory()
  }

  @Test
  fun `should create genesis with default configuration`() {
    val chainId = 1337U

    val result =
      factory.create(
        chainId = chainId,
        validators = validatorAddresses,
      )

    assertThat(result).isEqualTo(
      ForksSchedule(
        chainId = chainId,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
            ),
          ),
        ),
      ),
    )
  }

  @Test
  fun `should create genesis with specific blocktime`() {
    val result =
      factory.create(
        chainId = 1337U,
        validators = validatorAddresses,
        blockTimeSeconds = 5U,
      )

    assertThat(result.forks.first.blockTimeSeconds).isEqualTo(5U)
  }

  @Test
  fun `should create genesis with single fork`() {
    val forkTimestamp = Instant.fromEpochSeconds(0)

    val result =
      factory.create(
        chainId = 1337U,
        validators = validatorAddresses,
        forks = mapOf(forkTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai)),
      )

    assertThat(result).isEqualTo(
      ForksSchedule(
        chainId = 1337U,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
            ),
          ),
        ),
      ),
    )
  }

  @Test
  fun `should create genesis with single fork and ttd`() {
    val forkTimestamp = Instant.fromEpochSeconds(0)

    val result =
      factory.create(
        chainId = 1337U,
        validators = validatorAddresses,
        terminalTotalDifficulty = 50UL,
        forks = mapOf(forkTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai)),
      )

    assertThat(result).isEqualTo(
      ForksSchedule(
        chainId = 1337U,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1U,
            configuration = DifficultyAwareQbftConfig(
              terminalTotalDifficulty = 50UL,
              postTtdConfig = QbftConsensusConfig(
                validatorSet = validators.toSet(),
                fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
              ),
            ),
          ),
        ),
      ),
    )
  }

  @Test
  fun `should create genesis with all forks in chronological order with ttd`() {
    val forks =
      mapOf(
        Instant.fromEpochSeconds(0) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
        Instant.fromEpochSeconds(1000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
        Instant.fromEpochSeconds(2000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
        Instant.fromEpochSeconds(3000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
        Instant.fromEpochSeconds(4000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Osaka),
        Instant.fromEpochSeconds(5000) to ChainFork(ClFork.QBFT_PHASE1, ElFork.Osaka),
      )

    val result =
      factory.create(
        chainId = 1337U,
        validators = validatorAddresses,
        blockTimeSeconds = 1U,
        terminalTotalDifficulty = 50UL,
        forks = forks,
      )

    assertThat(result).isEqualTo(
      ForksSchedule(
        chainId = 1337U,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1U,
            configuration = DifficultyAwareQbftConfig(
              terminalTotalDifficulty = 50UL,
              postTtdConfig = QbftConsensusConfig(
                validatorSet = validators.toSet(),
                fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
              ),
            ),
          ),
          ForkSpec(
            timestampSeconds = 1000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
            ),
          ),
          ForkSpec(
            timestampSeconds = 2000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
            ),
          ),
          ForkSpec(
            timestampSeconds = 3000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
            ),
          ),
          ForkSpec(
            timestampSeconds = 4000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Osaka),
            ),
          ),
          ForkSpec(
            timestampSeconds = 5000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE1, ElFork.Osaka),
            ),
          ),
        ),
      ),
    )
  }

  @Test
  fun `should create genesis with subset of forks in chronological order with ttd`() {
    val forks =
      mapOf(
        Instant.fromEpochSeconds(0) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
        Instant.fromEpochSeconds(100_000_000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
      )

    val result =
      factory.create(
        chainId = 1337U,
        validators = validatorAddresses,
        blockTimeSeconds = 1U,
        terminalTotalDifficulty = 50UL,
        forks = forks,
      )

    assertThat(result).isEqualTo(
      ForksSchedule(
        chainId = 1337U,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1U,
            configuration = DifficultyAwareQbftConfig(
              terminalTotalDifficulty = 50UL,
              postTtdConfig = QbftConsensusConfig(
                validatorSet = validators.toSet(),
                fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
              ),
            ),
          ),
          ForkSpec(
            timestampSeconds = 100_000_000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
            ),
          ),
        ),
      ),
    )
  }

  @Test
  fun `should create genesis with subset of forks in chronological order without ttd`() {
    val forks =
      mapOf(
        Instant.fromEpochSeconds(0) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
        Instant.fromEpochSeconds(100_000_000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
      )

    val result =
      factory.create(
        chainId = 1337U,
        validators = validatorAddresses,
        blockTimeSeconds = 1U,
        forks = forks,
      )

    assertThat(result).isEqualTo(
      ForksSchedule(
        chainId = 1337U,
        forks = listOf(
          ForkSpec(
            timestampSeconds = 0UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
            ),
          ),
          ForkSpec(
            timestampSeconds = 100_000_000UL,
            blockTimeSeconds = 1U,
            configuration = QbftConsensusConfig(
              validatorSet = validators.toSet(),
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
            ),
          ),
        ),
      ),
    )
  }

  @Nested
  inner class InputValidationTests {
    @Test
    fun `should fail when forks don't follow correct order`() {
      val forks =
        mapOf(
          Instant.fromEpochSeconds(1000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
          Instant.fromEpochSeconds(0) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
          Instant.fromEpochSeconds(2000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
          Instant.fromEpochSeconds(3000) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
        )

      assertThatThrownBy {
        factory.create(
          chainId = 1337U,
          validators = validatorAddresses,
          blockTimeSeconds = 1U,
          forks = forks,
        )
      }.isInstanceOf(java.lang.IllegalArgumentException::class.java)
        .hasMessageContaining(
          "EL forks don't follow the correct order: found [Shanghai, Paris, Cancun, Prague], " +
            "expected [Paris, Shanghai, Cancun, Prague]",
        )
    }

    @Test
    fun `should fail with duplicate validator addresses`() {
      val duplicateValidators =
        listOf(
          validatorAddresses[0],
          validatorAddresses[0], // duplicate
        )

      assertThatThrownBy {
        factory.create(
          chainId = 1337U,
          validators = duplicateValidators,
          blockTimeSeconds = 1U,
        )
      }.isInstanceOf(java.lang.IllegalArgumentException::class.java)
        .hasMessageContaining("Validators are duplicated")
    }

    @Test
    fun `should fail with zero block time`() {
      assertThatThrownBy {
        factory.create(
          chainId = 1337U,
          validators = validatorAddresses,
          blockTimeSeconds = 0U,
        )
      }.isInstanceOf(IllegalArgumentException::class.java)
    }

    @Test
    fun `should fail with non sense big block time`() {
      assertThatThrownBy {
        factory.create(
          chainId = 1337U,
          validators = validatorAddresses,
          blockTimeSeconds = 61u,
        )
      }.isInstanceOf(IllegalArgumentException::class.java)
    }
  }
}
