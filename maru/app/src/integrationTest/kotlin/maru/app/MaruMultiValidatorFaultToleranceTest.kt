/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.MultiValidatorNetwork
import kotlin.time.Duration.Companion.seconds

class MaruMultiValidatorFaultToleranceTest {
  companion object {
    /** Number of consecutive round-0 blocks required to declare convergence / stable production. */
    private const val STABLE_BLOCKS = 5
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
  fun `block production continues with 1 node offline`() {
    network.startAllValidators()

    // Wait for convergence before stopping a node
    network.waitForConsecutiveRound0Blocks(
      validatorIndex = 0,
      requiredConsecutive = STABLE_BLOCKS,
      timeout = 240.seconds,
    )

    log.info("Stopping validator 3")
    network.stopValidator(3)

    // Record current height and wait for STABLE_BLOCKS more blocks
    val heightAfterStop = network.currentBlockHeight(0)
    log.info("Height after stopping validator 3: $heightAfterStop")
    // Wait for all 3 remaining validators so getSealedBeaconBlocks doesn't throw on any of them.
    network.waitForBlockHeight(
      0..2,
      targetHeight = heightAfterStop + STABLE_BLOCKS.toULong(),
      timeout = 240.seconds,
    )

    network.checkValidatorsHaveSameBlocks(0..2, heightAfterStop + 1uL, STABLE_BLOCKS.toULong())
    log.info("Block production continued successfully with 3 of 4 validators")
  }

  @Test
  fun `block production recovers after 2 nodes offline and 1 returns`() {
    network.startAllValidators()

    // Wait for convergence before stopping nodes
    network.waitForConsecutiveRound0Blocks(
      validatorIndex = 0,
      requiredConsecutive = STABLE_BLOCKS,
      timeout = 240.seconds,
    )

    // Stop validators 2 and 3 -- only 2 of 4 remain, below quorum (need 3)
    log.info("Stopping validators 2 and 3")
    network.stopValidator(2)
    network.stopValidator(3)

    val heightAfterStop = network.currentBlockHeight(0)
    log.info("Height after stopping 2 validators: $heightAfterStop")

    // Wait 5 seconds and verify no new blocks were produced
    Thread.sleep(5000)
    val heightAfterWait = network.currentBlockHeight(0)
    assertThat(heightAfterWait)
      .withFailMessage {
        "Expected no new blocks (height $heightAfterStop) but got height $heightAfterWait"
      }.isEqualTo(heightAfterStop)
    log.info("Confirmed: no blocks produced without quorum")

    // Restart validator 2 -- quorum restored (3 of 4)
    log.info("Restarting validator 2")
    network.restartValidator(2, peerIndexes = listOf(0, 1))

    // Wait for all 3 active validators before reading their blocks.
    network.waitForBlockHeight(
      0..2,
      targetHeight = heightAfterStop + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    network.checkValidatorsHaveSameBlocks(0..2, heightAfterStop + 1uL, STABLE_BLOCKS.toULong())
    log.info("Block production recovered after quorum was restored")
  }
}
