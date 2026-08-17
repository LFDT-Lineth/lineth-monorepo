/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class ForkAwareBlockHashingTest {
  private val validatorSet = setOf(DataGenerators.randomValidator())

  private fun schedule(
    phase0EndTimestamp: ULong,
    phase1StartTimestamp: ULong,
  ): ForksSchedule =
    ForksSchedule(
      1337u,
      listOf(
        ForkSpec(
          phase0EndTimestamp,
          1u,
          QbftConsensusConfig(
            validatorSet = validatorSet,
            fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
          ),
        ),
        ForkSpec(
          phase1StartTimestamp,
          1u,
          QbftConsensusConfig(
            validatorSet = validatorSet,
            fork = ChainFork(ClFork.QBFT_PHASE1, ElFork.Prague),
          ),
        ),
      ),
    )

  @Test
  fun `PHASE0 identity is round-inclusive, changes with round`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 1000UL))
    val header =
      DataGenerators.randomBeaconBlockHeader(
        1UL,
      ).copy(timestamp = 10UL, beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction)
    val sameContentDifferentRound = header.copy(
      round = header.round + 1u,
      beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
    )

    assertThat(header.beaconBlockIdHash).isEqualTo(HashUtil.headerHash(header))
    assertThat(sameContentDifferentRound.beaconBlockIdHash).isNotEqualTo(header.beaconBlockIdHash)
  }

  @Test
  fun `PHASE1 identity is round-independent, same content different round and proposer produce same identity`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 100UL))
    val timestamp = 200UL // past the PHASE1 activation timestamp
    val header =
      DataGenerators.randomBeaconBlockHeader(1UL).copy(
        timestamp = timestamp,
        round = 0u,
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )
    val sameContentDifferentRoundAndProposer =
      header.copy(
        round = 7u,
        proposer = Validator(DataGenerators.randomValidator().address),
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )

    assertThat(sameContentDifferentRoundAndProposer.beaconBlockIdHash).isEqualTo(header.beaconBlockIdHash)
    assertThat(header.beaconBlockIdHash).isEqualTo(HashUtil.roundIndependentHeaderHash(header))
  }

  /**
   * Reproduces the chain fork reported when one validator commits a block at round 0 while the rest
   * round-change and commit the re-proposed block (identical content) at round 1: both are valid, quorum-signed
   * commits of the same underlying block, but PHASE0's round-inclusive identity hash makes them
   * hash-irreconcilable, forking the chain. PHASE1's round-independent identity resolves this by giving both
   * commits the same chain identity.
   */
  private fun assertBlockCommittedAtDifferentRoundsHasOneChainIdentity(
    blockHashing: ForkAwareBlockHashing,
    timestamp: ULong,
  ) {
    val committedAtRound0 =
      DataGenerators.randomBeaconBlockHeader(1UL).copy(
        timestamp = timestamp,
        round = 0u,
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )
    val committedAtRound1 =
      committedAtRound0.copy(
        round = 1u,
        proposer = Validator(DataGenerators.randomValidator().address),
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )

    assertThat(committedAtRound1.beaconBlockIdHash)
      .withFailMessage {
        "validators forked: the same block content committed at round 0 vs round 1 produced different chain identities"
      }.isEqualTo(committedAtRound0.beaconBlockIdHash)
  }

  @Test
  fun `PHASE0 bug reproduction - commits of the same block at different rounds fork the chain`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 1000UL))

    assertThrows<AssertionError> {
      assertBlockCommittedAtDifferentRoundsHasOneChainIdentity(blockHashing, timestamp = 10UL)
    }
  }

  @Test
  fun `PHASE1 fix - commits of the same block at different rounds converge on one chain identity`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 100UL))

    assertBlockCommittedAtDifferentRoundsHasOneChainIdentity(blockHashing, timestamp = 200UL)
  }

  @Test
  fun `PHASE1 identity still changes when real content changes`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 100UL))
    val timestamp = 200UL
    val header =
      DataGenerators.randomBeaconBlockHeader(1UL).copy(
        timestamp = timestamp,
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )
    val differentBodyRoot =
      header.copy(
        bodyRoot = DataGenerators.randomBeaconBlockHeader(2UL).bodyRoot,
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )

    assertThat(differentBodyRoot.beaconBlockIdHash).isNotEqualTo(header.beaconBlockIdHash)
  }

  @Test
  fun `fork activation boundary is resolved by the header's own timestamp`() {
    val phase1Timestamp = 1000UL
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = phase1Timestamp))

    val justBeforeActivation =
      DataGenerators.randomBeaconBlockHeader(1UL).copy(
        timestamp = phase1Timestamp - 1UL,
        round = 3u,
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )
    val atActivation =
      justBeforeActivation.copy(
        timestamp = phase1Timestamp,
        beaconBlockIdHashFunction = blockHashing.beaconBlockIdHashFunction,
      )

    // Before activation: round-inclusive identity.
    assertThat(justBeforeActivation.beaconBlockIdHash).isEqualTo(HashUtil.headerHash(justBeforeActivation))
    // At activation: round-independent identity.
    assertThat(atActivation.beaconBlockIdHash).isEqualTo(HashUtil.roundIndependentHeaderHash(atActivation))
  }

  @Test
  fun `PHASE0 state root is round-inclusive`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 1000UL))
    val state = DataGenerators.randomBeaconState(number = 1UL, timestamp = 10UL)

    assertThat(blockHashing.stateRoot(state)).isEqualTo(HashUtil.stateRoot(state))
  }

  @Test
  fun `PHASE1 state root is round-independent`() {
    val blockHashing = ForkAwareBlockHashing(schedule(phase0EndTimestamp = 0UL, phase1StartTimestamp = 100UL))
    val state = DataGenerators.randomBeaconState(number = 1UL, timestamp = 200UL)

    assertThat(blockHashing.stateRoot(state)).isEqualTo(HashUtil.roundIndependentStateRoot(state))
  }
}
