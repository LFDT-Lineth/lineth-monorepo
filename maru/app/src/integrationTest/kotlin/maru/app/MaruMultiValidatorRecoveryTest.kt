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
 * Multi-validator QBFT restart-recovery scenarios (quorum loss/recovery, full restart). Split out
 * from the convergence/liveness scenarios (see [MaruMultiValidatorTestBase]) so the two groups run
 * in parallel Gradle forks.
 */
class MaruMultiValidatorRecoveryTest : MaruMultiValidatorTestBase() {
  @Test
  fun `block production recovers after 2 nodes offline and 1 returns`() {
    startAllValidators()

    // Wait for convergence before stopping nodes
    waitForConsecutiveRound0Blocks(
      stack0.maruApp.beaconChain,
      requiredConsecutive = STABLE_BLOCKS,
      timeout = 240.seconds,
    )

    // Stop validators 2 and 3 -- only 2 of 4 remain, below quorum (need 3)
    log.info("Stopping validators 2 and 3")
    stopValidator(stack2)
    stopValidator(stack3)

    val heightAfterStop = currentBlockHeight(stack0)
    log.info("Height after stopping 2 validators: $heightAfterStop")

    // Wait 5 seconds and verify no new blocks were produced
    Thread.sleep(5000)
    val heightAfterWait = currentBlockHeight(stack0)
    assertThat(heightAfterWait)
      .withFailMessage {
        "Expected no new blocks (height $heightAfterStop) but got height $heightAfterWait"
      }.isEqualTo(heightAfterStop)
    log.info("Confirmed: no blocks produced without quorum")

    // Restart validator 2 -- quorum restored (3 of 4)
    log.info("Restarting validator 2")
    restartValidator(stack2, maruFactory2, listOf(stack0.maruApp, stack1.maruApp))

    // Wait for all 3 active validators before reading their blocks.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      targetHeight = heightAfterStop + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    // Verify consistency across the 3 active validators
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
    log.info("Block production recovered after quorum was restored")
  }

  @Test
  fun `block production resumes after all 4 nodes restart`() {
    startAllValidators()

    // Wait for STABLE_BLOCKS blocks before recording the checkpoint (stack0 only; all are synced).
    waitForBlockHeight(stack0.maruApp.beaconChain, targetHeight = STABLE_BLOCKS.toULong(), timeout = 90.seconds)
    val heightBeforeRestart = currentBlockHeight(stack0)
    log.info("Height before full restart: $heightBeforeRestart")

    // Stop all 4 validators
    log.info("Stopping all 4 validators")
    stopValidator(stack0)
    stopValidator(stack1)
    stopValidator(stack2)
    stopValidator(stack3)

    Thread.sleep(2000)

    // Restart all 4 and re-establish full mesh
    log.info("Restarting all 4 validators")
    startAllValidators()

    // Wait for ALL 4 validators to reach the target before reading their blocks.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      stack3.maruApp.beaconChain,
      targetHeight = heightBeforeRestart + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    // Verify consistency across all 4 validators
    val verifyStart = heightBeforeRestart + 1uL
    val count = STABLE_BLOCKS.toULong()
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack3.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )
    log.info("Block production resumed successfully after full restart")
  }
}
