/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.core.ext.DataGenerators
import maru.database.BeaconChain
import maru.database.InMemoryBeaconChain
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random

class RollingForwardForkIdDigestCalculatorTest {
  private fun createCalculator(
    chainId: UInt = 123u,
    beaconChain: BeaconChain = inMemoryBeaconChainFromGenesis(),
  ): RollingForwardForkIdDigestCalculator =
    RollingForwardForkIdDigestCalculator(
      chainId = chainId,
      beaconChain = beaconChain,
      digester = ForkIdDigester(),
    )

  private val validatorSet =
    setOf(
      Validator(Random.nextBytes(20)),
      Validator(Random.nextBytes(20)),
    )

  private fun forkSpec(
    timestamp: ULong,
    elFork: ElFork,
  ): ForkSpec = ForkSpec(timestamp, 1u, QbftConsensusConfig(validatorSet, fork = ChainFork(ClFork.QBFT_PHASE0, elFork)))

  private val forks =
    listOf(
      forkSpec(0UL, ElFork.Paris),
      forkSpec(10UL, ElFork.Shanghai),
      forkSpec(20UL, ElFork.Cancun),
      forkSpec(30UL, ElFork.Prague),
    )

  @Test
  fun `should return 4 bytes digests`() {
    assertThat(
      createCalculator()
        .calculateForkDigests(forks),
    ).withFailMessage("digests should be fixed size of 4 bytes")
      .allSatisfy { assertThat(it.forkIdDigest).hasSize(4) }
  }

  @Test
  fun `should take genesis block hash into account`() {
    assertThat(
      createCalculator(beaconChain = inMemoryBeaconChainFromGenesis(genesisTimestampSeconds = 1u))
        .calculateForkDigests(forks.take(1)),
    ).isNotEqualTo(
      createCalculator(beaconChain = inMemoryBeaconChainFromGenesis(genesisTimestampSeconds = 2u))
        .calculateForkDigests(forks.take(1)),
    ).withFailMessage("digests should take genesis block hash into account")
  }

  @Test
  fun `should take genesis chainId into account`() {
    val beaconChain = inMemoryBeaconChainFromGenesis()
    assertThat(
      createCalculator(chainId = 1u, beaconChain = beaconChain)
        .calculateForkDigests(forks.take(1)),
    ).isNotEqualTo(
      createCalculator(chainId = 2u, beaconChain = beaconChain)
        .calculateForkDigests(forks.take(1)),
    ).withFailMessage("digests should take chainId into account")
  }

  @Test
  fun `should be deterministic`() {
    val beaconChain = inMemoryBeaconChainFromGenesis()
    assertThat(
      createCalculator(chainId = 1u, beaconChain = beaconChain)
        .calculateForkDigests(forks),
    ).isEqualTo(
      createCalculator(chainId = 1u, beaconChain = beaconChain)
        .calculateForkDigests(forks.shuffled()),
    ).withFailMessage("digests should be deterministic")
  }

  @Test
  fun `should take parent fork into account`() {
    val forksA =
      listOf(
        forkSpec(0UL, ElFork.Paris),
        forkSpec(10UL, ElFork.Shanghai),
        forkSpec(20UL, ElFork.Cancun),
        forkSpec(30UL, ElFork.Prague),
      )
    val forksB =
      listOf(
        forkSpec(0UL, ElFork.Paris),
        forkSpec(20UL, ElFork.Cancun),
        forkSpec(30UL, ElFork.Prague),
      )
    val calculator = createCalculator()
    assertThat(
      calculator.calculateForkDigests(forksA).firstOrNull { it.forkSpec.configuration.fork.elFork == ElFork.Prague },
    ).isNotEqualTo(
      calculator.calculateForkDigests(forksB).firstOrNull { it.forkSpec.configuration.fork.elFork == ElFork.Prague },
    ).withFailMessage("digests should take parent fork into account")
  }

  private fun inMemoryBeaconChainFromGenesis(
    genesisTimestampSeconds: ULong = 0UL,
  ): InMemoryBeaconChain {
    val (beaconState, sealedBlock) = DataGenerators.genesisState(
      genesisTimestamp = genesisTimestampSeconds,
    )
    return InMemoryBeaconChain(beaconState, sealedBlock)
  }
}
