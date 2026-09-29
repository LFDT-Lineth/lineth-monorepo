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
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.MultiValidatorNetwork
import kotlin.time.Duration.Companion.seconds

class MaruMultiValidatorRestartTest {
  companion object {
    /** Number of blocks to wait for before and after the restart. */
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
  fun `block production resumes after all 4 nodes restart`() {
    network.startAllValidators()

    // Wait for STABLE_BLOCKS blocks before recording the checkpoint (validator 0 only; all are synced).
    network.waitForBlockHeight(0..0, targetHeight = STABLE_BLOCKS.toULong(), timeout = 90.seconds)
    val heightBeforeRestart = network.currentBlockHeight(0)
    log.info("Height before full restart: $heightBeforeRestart")

    log.info("Stopping all 4 validators")
    allValidators.forEach { network.stopValidator(it) }

    Thread.sleep(2000)

    log.info("Restarting all 4 validators")
    network.startAllValidators()

    // Wait for ALL 4 validators to reach the target before reading their blocks.
    network.waitForBlockHeight(
      allValidators,
      targetHeight = heightBeforeRestart + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    network.checkValidatorsHaveSameBlocks(allValidators, heightBeforeRestart + 1uL, STABLE_BLOCKS.toULong())
    log.info("Block production resumed successfully after full restart")
  }
}
