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
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
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
      sendTransactionWhenElReady(validatorBesuNode)
    }

    validatorBesuNode.assertMinedBlocks(blocksToProduce)

    // adding a node to a running cluster starts it immediately
    cluster.addNode(NodeRole.Follower, withBesuEl = true) {
      it.staticPeers(listOf("sequencer"))
    }
    val followerBesuNode = cluster.besuNode("follower")

    followerBesuNode.assertMinedBlocks(blocksToProduce)

    repeat(blocksToProduce) {
      sendTransactionWhenElReady(followerBesuNode)
    }

    validatorBesuNode.assertMinedBlocks(2 * blocksToProduce)
    followerBesuNode.assertMinedBlocks(2 * blocksToProduce)
  }

  /**
   * Besu's txpool stays disabled while the node reports "not in sync", and under CPU contention from
   * parallel test forks the sync flag can flap even after blocks are imported — so every send is
   * retried until the EL accepts it. Sends rejected with "Transaction pool not enabled" never enter
   * the pool, so retrying them cannot duplicate a transaction.
   */
  private fun sendTransactionWhenElReady(node: BesuNode) {
    await
      .atMost(120.seconds.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        transactionsHelper.run {
          node.sendTransactionAndAssertExecution(
            logger = log,
            recipient = createAccount("another account"),
            amount = Amount.ether(100),
          )
        }
      }
  }
}
