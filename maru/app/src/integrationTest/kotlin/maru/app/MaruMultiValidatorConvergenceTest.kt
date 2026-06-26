/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

/**
 * Multi-validator QBFT convergence + liveness scenarios. Split out from the restart-recovery
 * scenarios (see [MaruMultiValidatorTestBase]) so the two groups run in parallel Gradle forks.
 */
class MaruMultiValidatorConvergenceTest : MaruMultiValidatorTestBase() {
  @Test
  fun `validators converge to stable block production without round skips`() {
    startAllValidators()

    // Wait until 5 consecutive round-0 blocks are observed. During startup, validators run QBFT
    // independently before the P2P mesh is wired, causing round skips on early blocks
    val stableHeight =
      waitForConsecutiveRound0Blocks(
        stack0.maruApp.beaconChain,
        requiredConsecutive = STABLE_BLOCKS,
        timeout = 240.seconds,
      )
    log.info("QBFT convergence achieved at block $stableHeight")

    // Verify STABLE_BLOCKS more blocks after convergence are also round-0.
    // Wait for ALL validators to reach the target so blocks can safely be read from all.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      stack3.maruApp.beaconChain,
      targetHeight = stableHeight + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )
    val verifyStart = stableHeight - (STABLE_BLOCKS - 1).toULong()
    val verifyCount = (STABLE_BLOCKS * 2).toULong()
    val verifyBlocks = stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount)
    verifyBlocks.forEach { block ->
      val header = block.beaconBlock.beaconBlockHeader
      assertThat(header.round)
        .withFailMessage { "Block ${header.number} has round ${header.round}, expected 0" }
        .isEqualTo(0u)
    }

    // Verify blocks consistent across all 4 validators
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
        { stack3.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )

    // Verify proposers match expected
    checkBlockProposersMatchExpectedProposers(
      beaconChain = stack0.maruApp.beaconChain,
      startBlock = verifyStart,
      endBlock = stableHeight + STABLE_BLOCKS.toULong(),
    )
  }

  @Test
  fun `block production continues with 1 node offline`() {
    startAllValidators()

    // Wait for convergence before stopping a node
    waitForConsecutiveRound0Blocks(
      stack0.maruApp.beaconChain,
      requiredConsecutive = STABLE_BLOCKS,
      timeout = 240.seconds,
    )

    // Stop validator 3
    log.info("Stopping validator 3")
    stopValidator(stack3)

    // Record current height and wait for STABLE_BLOCKS more blocks
    val heightAfterStop = currentBlockHeight(stack0)
    log.info("Height after stopping validator 3: $heightAfterStop")
    // Wait for all 3 remaining validators so getSealedBeaconBlocks doesn't throw on any of them.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      targetHeight = heightAfterStop + STABLE_BLOCKS.toULong(),
      timeout = 240.seconds,
    )

    // Verify blocks consistent across the 3 remaining validators
    val verifyStart = heightAfterStop + 1uL
    val count = STABLE_BLOCKS.toULong()
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )
    log.info("Block production continued successfully with 3 of 4 validators")
  }
}
