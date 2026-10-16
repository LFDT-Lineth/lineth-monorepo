/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuTransactionsHelper
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.assertMinedBlocks

class MaruFollowerDelayedStartTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    transactionsHelper = BesuTransactionsHelper()
  }

  @AfterEach
  fun tearDown() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  @Test
  fun `follower can sync and accept transactions after delayed start`() {
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true)
      .start()
    val validatorBesuNode = cluster.besuNode("sequencer")

    val blocksToProduce = 10
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    validatorBesuNode.assertMinedBlocks(blocksToProduce)

    // adding a node to a running cluster starts it immediately
    cluster.addNode(NodeRole.Follower, withBesuEl = true) {
      it.staticPeers(listOf("sequencer"))
    }
    val followerBesuNode = cluster.besuNode("follower")

    followerBesuNode.assertMinedBlocks(blocksToProduce)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        followerBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    validatorBesuNode.assertMinedBlocks(2 * blocksToProduce)
    followerBesuNode.assertMinedBlocks(2 * blocksToProduce)
  }
}
