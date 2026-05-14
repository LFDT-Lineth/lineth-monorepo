/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.genesis

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.time.Instant

class GenesisFactoryTest {
  val objectMapper = jacksonObjectMapper()
  val validators = listOf(Random.nextBytes(20))
  private lateinit var genesisFactory: GenesisFactory

  @BeforeEach
  fun setup() {
    genesisFactory = GenesisFactory(
      chainId = 13U,
      blockTimeSeconds = 1U,
    )
  }

  @Test
  fun `should create genesis with ttd and all chain forks from Paris to Osaka`() {
    val parisTime = Instant.fromEpochSeconds(0)
    val shanghaiTimestamp = Instant.parse("2025-01-01T00:00:00Z")
    val cancunTimestamp = Instant.parse("2025-02-01T00:00:00Z")
    val cancunTimestamp2 = Instant.parse("2025-02-02T00:00:00Z")
    val pragueTimestamp = Instant.parse("2025-03-01T00:00:00Z")
    val osakaTimestamp = Instant.parse("2025-04-01T00:00:00Z")
    val osakaTimestamp2 = Instant.parse("2025-04-02T00:00:00Z")

    val forks =
      mapOf(
        parisTime to ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
        shanghaiTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
        cancunTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
        cancunTimestamp2 to ChainFork(ClFork.QBFT_PHASE1, ElFork.Cancun),
        pragueTimestamp to ChainFork(ClFork.QBFT_PHASE1, ElFork.Prague),
        osakaTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Osaka),
        osakaTimestamp2 to ChainFork(ClFork.QBFT_PHASE1, ElFork.Osaka),
      )

    genesisFactory.initForkSchedule(
      sequencersAddresses = validators,
      terminalTotalDifficulty = 10UL,
      chainForks = forks,
    )
    genesisFactory.maruForkSchedule().also { forksSchedule ->
      assertThat(
        forksSchedule,
      ).isEqualTo(MaruGenesisFactory().create(13U, validators, terminalTotalDifficulty = 10u, forks = forks))
    }

    val jsonNode = objectMapper.readTree(genesisFactory.besuGenesis())
    val config = jsonNode.get("config")

    assertIsNumberWithValue(config.get("chainId"), 13L)
    assertIsNumberWithValue(
      config.get("terminalTotalDifficulty"),
      10L,
    )
    assertIsNumberWithValue(config.get("shanghaiTime"), shanghaiTimestamp.epochSeconds)
    assertIsNumberWithValue(config.get("cancunTime"), cancunTimestamp.epochSeconds)
    assertIsNumberWithValue(config.get("pragueTime"), pragueTimestamp.epochSeconds)
    assertIsNumberWithValue(config.get("osakaTime"), osakaTimestamp.epochSeconds)
  }

  @Test
  fun `should create genesis without ttd and subset of forks `() {
    val epochTimestamp = Instant.fromEpochSeconds(0)
    val osakaTimestamp = Instant.parse("2025-04-04T00:00:00Z")

    val forks =
      mapOf(
        epochTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
        osakaTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Osaka),
      )

    genesisFactory.initForkSchedule(
      sequencersAddresses = validators,
      chainForks = forks,
    )

    genesisFactory.maruForkSchedule().also { forksSchedule ->
      val expected =
        ForksSchedule(
          chainId = 13U,
          forks = listOf(
            ForkSpec(
              timestampSeconds = epochTimestamp.epochSeconds.toULong(),
              blockTimeSeconds = 1U,
              configuration = QbftConsensusConfig(
                validatorSet = validators.map { Validator(it) }.toSet(),
                fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
              ),
            ),
            ForkSpec(
              timestampSeconds = osakaTimestamp.epochSeconds.toULong(),
              blockTimeSeconds = 1U,
              configuration = QbftConsensusConfig(
                validatorSet = validators.map { Validator(it) }.toSet(),
                fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Osaka),
              ),
            ),
          ),
        )

      assertThat(forksSchedule).isEqualTo(expected)
    }

    val jsonNode = objectMapper.readTree(genesisFactory.besuGenesis())
    val config = jsonNode.get("config")

    assertIsNumberWithValue(config.get("chainId"), 13L)
    assertIsNumberWithValue(config.get("terminalTotalDifficulty"), 0L)
    assertIsNumberWithValue(config.get("shanghaiTime"), epochTimestamp.epochSeconds)
    assertIsNumberWithValue(config.get("cancunTime"), epochTimestamp.epochSeconds)
    assertIsNumberWithValue(config.get("pragueTime"), epochTimestamp.epochSeconds)
    assertIsNumberWithValue(config.get("osakaTime"), osakaTimestamp.epochSeconds)
  }
}
