/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.consensus.ClFork
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.MultiValidatorNetwork
import kotlin.time.Duration.Companion.seconds

class MaruMultiValidatorConvergenceTest {
  companion object {
    /** Number of consecutive round-0 blocks required to declare convergence / stable production. */
    private const val STABLE_BLOCKS = 5
    private val allValidators = 0..3
  }

  private val log = LogManager.getLogger(this.javaClass)
  private lateinit var network: MultiValidatorNetwork

  @BeforeEach
  fun setUp() {
    network = MultiValidatorNetwork()
    network.startBesuNodes()
  }

  @AfterEach
  fun tearDown() {
    network.close()
  }

  @Test
  fun `validators converge to stable block production without round skips`() {
    network.startAllValidators()

    // Wait until 5 consecutive round-0 blocks are observed. During startup, validators run QBFT
    // independently before the P2P mesh is wired, causing round skips on early blocks
    val stableHeight =
      network.waitForConsecutiveRound0Blocks(
        validatorIndex = 0,
        requiredConsecutive = STABLE_BLOCKS,
        timeout = 240.seconds,
      )
    log.info("QBFT convergence achieved at block $stableHeight")

    // Verify STABLE_BLOCKS more blocks after convergence are also round-0.
    // Wait for ALL validators to reach the target so blocks can safely be read from all.
    network.waitForBlockHeight(
      allValidators,
      targetHeight = stableHeight + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )
    val verifyStart = stableHeight - (STABLE_BLOCKS - 1).toULong()
    val verifyCount = (STABLE_BLOCKS * 2).toULong()
    val verifyBlocks = network.beaconChain(0).getSealedBeaconBlocks(verifyStart, verifyCount)
    verifyBlocks.forEach { block ->
      val header = block.beaconBlock.beaconBlockHeader
      assertThat(header.round)
        .withFailMessage { "Block ${header.number} has round ${header.round}, expected 0" }
        .isEqualTo(0u)
    }

    network.checkValidatorsHaveSameBlocks(allValidators, verifyStart, verifyCount)

    network.checkBlockProposersMatchExpectedProposers(
      validatorIndex = 0,
      startBlock = verifyStart,
      endBlock = stableHeight + STABLE_BLOCKS.toULong(),
    )
  }

  @Test
  fun `PHASE1 validators converge on the same chain identity`() {
    // Regression guard for the round-independent chain-identity hash (QBFT_PHASE1). Object equality of
    // BeaconBlockHeader must stay field-based (round-sensitive); if it were hash-based, headers differing
    // only in round/proposer would collapse into one object under PHASE1 and the QBFT engine would fail to
    // converge (this test would time out at waitForConsecutiveRound0Blocks).
    network.startAllValidators(network.factoriesWithClFork(ClFork.QBFT_PHASE1))

    val stableHeight =
      network.waitForConsecutiveRound0Blocks(
        validatorIndex = 0,
        requiredConsecutive = STABLE_BLOCKS,
        timeout = 240.seconds,
      )
    log.info("PHASE1 QBFT convergence achieved at block $stableHeight")

    // Wait for ALL validators to reach the target so blocks can safely be read from all.
    network.waitForBlockHeight(
      allValidators,
      targetHeight = stableHeight + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    val verifyStart = stableHeight - (STABLE_BLOCKS - 1).toULong()
    val verifyCount = (STABLE_BLOCKS * 2).toULong()

    // The "no fork" guarantee under PHASE1 is that all validators agree on the round-independent chain
    // identity (block root) for each height. They may legitimately store byte-different sealed blocks —
    // differing only in round/proposer/committed-seals when they commit the same block at different rounds
    // (this is exactly what QBFT_PHASE1 makes safe, and mirrors Besu's on-chain hash which excludes round +
    // committed seals). So assert identity-hash equality across validators, not raw serialized bytes.
    network.checkValidatorsHaveSameBlocks(allValidators, verifyStart, verifyCount)
  }
}
