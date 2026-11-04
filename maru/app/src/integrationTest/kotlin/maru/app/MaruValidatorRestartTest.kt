/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuTransactionsHelper
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeBuilder
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastPeering
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.assertMinedBlocks
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruValidatorRestartTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  @BeforeEach
  fun setup() {
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

  // Fast peering with 10s reputation periods, and syncing one block at a time
  private fun NodeBuilder.withTestDefaults(): NodeBuilder =
    withFastPeering(banPeriod = 10.seconds, cooldownPeriod = 10.seconds)
      .withSyncingConfig(TestSyncingConfigs.validator)
      .maruConfig { it.copy(allowEmptyBlocks = false) }

  @Test
  fun `Maru validator restarted from scratch is able to sync state`() {
    // The follower is started first, as it is the validator's discovery bootnode
    cluster = MaruCluster()
      .addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .payloadValidation(true)
          .withTestDefaults()
      }.addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder.withTestDefaults().withFastQbft()
        nodeBuilder.bootnodes(listOf("follower"))
      }.start()
    val validatorBesuNode = cluster.besuNode("sequencer")
    val followerBesuNode = cluster.besuNode("follower")
    val followerMaruApp = cluster.maruNode("follower")
    val validatorMaruApp = cluster.maruNode("sequencer")

    log.info(
      "Follower: ${followerMaruApp.p2pNetwork.nodeId}, validator: ${validatorMaruApp.p2pNetwork.nodeId}",
    )

    followerMaruApp.awaitTillMaruHasPeers(1u)
    validatorMaruApp.awaitTillMaruHasPeers(1u)

    val blocksToProduce = 5
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
    followerBesuNode.assertMinedBlocks(blocksToProduce)

    cluster.stopMaruNode("sequencer")

    await
      .timeout(30.seconds.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .until { followerMaruApp.peersConnected() == 0u }

    // Same node key, data directory and bootnode, new p2p and discovery ports
    val newValidatorMaruApp = cluster.startMaruNode("sequencer")

    log.info("Restarted validator: ${newValidatorMaruApp.p2pNetwork.nodeId}")

    newValidatorMaruApp.awaitTillMaruHasPeers(1u)
    followerMaruApp.awaitTillMaruHasPeers(1u)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorBesuNode.sendTransactionAndAssertExecution(
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
