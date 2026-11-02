/*
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
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.assertMinedBlocks
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

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

    // The follower's Besu EL can hold the blocks while still reporting "not in sync" (txpool stays
    // disabled until the sync flag flips), and that lag grows under CPU contention from parallel test
    // forks. Retry the first send until the EL accepts it; failed sends never enter the pool.
    await
      .atMost(120.seconds.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        transactionsHelper.run {
          followerBesuNode.sendTransactionAndAssertExecution(
            logger = log,
            recipient = createAccount("another account"),
            amount = Amount.ether(100),
          )
        }
      }
    repeat(blocksToProduce - 1) {
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
