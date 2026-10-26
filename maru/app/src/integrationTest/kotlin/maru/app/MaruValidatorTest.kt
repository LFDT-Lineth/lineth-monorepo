/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import maru.config.SyncingConfig
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.assertSameCanonicalElChain
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastPeering
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.assertMinedBlocks
import testutils.Checks.getMinedBlocks
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruValidatorTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  private val validatorBesuNode: BesuNode get() = cluster.besuNode("sequencer")
  private val followerBesuNode: BesuNode get() = cluster.besuNode("follower")

  private fun setupCluster(syncingConfig: SyncingConfig = TestSyncingConfigs.default) {
    // The follower is added first so that its Besu node is the Besu cluster's bootnode.
    // This way in the tests where the sequencer's Besu node is restarted, it can still peer within the cluster.
    // As the follower is started before the sequencer, it is the sequencer that statically peers with the follower.
    cluster = MaruCluster()
      .addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .payloadValidation(true)
          .maruConfig { it.copy(allowEmptyBlocks = false) }
          .withFastPeering()
          .withSyncingConfig(syncingConfig)
      }.addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = false) }
        nodeBuilder.withFastPeering().withFastQbft().withSyncingConfig(syncingConfig)
        nodeBuilder.staticPeers(listOf("follower"))
      }.start()

    log.info("Nodes are peered")
    cluster.maruNode("follower").awaitTillMaruHasPeers(1u)
    cluster.maruNode("sequencer").awaitTillMaruHasPeers(1u)
    cluster.assertSameCanonicalElChain(chainHeight = 0UL)
  }

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

  /**
   * Stops the sequencer's Besu node and builds a new one from scratch, on the same RPC ports so that the sequencer's
   * Maru node keeps using it. The new Besu node isn't started.
   */
  private fun stopSequencerBesuAndBuildNewOne(): BesuNode {
    val oldBesuSequencer = validatorBesuNode
    val genesis = oldBesuSequencer.genesisConfig.get()
    val engineRpcPort = oldBesuSequencer.engineJsonRpcPort
    val jsonRpcPort = oldBesuSequencer.jsonRpcPort
    log.info("Old sequencer jsonRpcPort=$jsonRpcPort, engineRpcPort=$engineRpcPort")
    cluster.besuCluster.stopNode(oldBesuSequencer.name)
    oldBesuSequencer.close()

    return BesuFactory.buildTestBesu(
      genesisFile = genesis,
      jsonRpcPort = jsonRpcPort,
      engineRpcPort = engineRpcPort,
    )
  }

  @Test
  fun `Besu sequencer restarted from scratch is able to sync state`() {
    setupCluster()
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

    val newBesuSequencer = stopSequencerBesuAndBuildNewOne()

    cluster.besuCluster.addNodeAndStart(newBesuSequencer)

    await
      .pollDelay(1.seconds.toJavaDuration())
      .timeout(30.seconds.toJavaDuration())
      .untilAsserted {
        val newSequencerBlocks = newBesuSequencer.getMinedBlocks(blocksToProduce)
        assertThat(newSequencerBlocks).hasSize(blocksToProduce)
      }

    repeat(blocksToProduce) {
      transactionsHelper.run {
        newBesuSequencer.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    newBesuSequencer.assertMinedBlocks(blocksToProduce * 2)
    followerBesuNode.assertMinedBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Besu sequencer restarted from scratch is able to sync state while follower accepts transactions`() {
    setupCluster()
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        followerBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    validatorBesuNode.assertMinedBlocks(blocksToProduce)
    followerBesuNode.assertMinedBlocks(blocksToProduce)

    val newBesuSequencer = stopSequencerBesuAndBuildNewOne()

    repeat(3) {
      transactionsHelper.run {
        followerBesuNode.sendTransaction(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    cluster.besuCluster.addNodeAndStart(newBesuSequencer)

    await
      .pollDelay(1.seconds.toJavaDuration())
      .timeout(30.seconds.toJavaDuration())
      .untilAsserted {
        val newSequencerBlocks = newBesuSequencer.getMinedBlocks(blocksToProduce + 1)
        assertThat(newSequencerBlocks).hasSize(blocksToProduce + 1)
      }

    repeat(blocksToProduce) {
      transactionsHelper.run {
        followerBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    newBesuSequencer.assertMinedBlocks(blocksToProduce * 2 + 1)
    followerBesuNode.assertMinedBlocks(blocksToProduce * 2 + 1)
  }
}
