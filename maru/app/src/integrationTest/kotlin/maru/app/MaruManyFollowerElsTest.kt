/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.ethGetBlockByNumber
import maru.config.ApiEndpointConfig
import maru.config.FollowersConfig
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastPeering
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks
import testutils.maru.awaitTillMaruHasPeers
import java.net.URI

/**
 * Integration tests for Maru with multiple follower execution layer nodes configured via FollowersConfig.
 * Tests verify that blocks are correctly propagated to all follower EL nodes.
 *
 * Test Architecture:
 * - 3 Besu nodes total:
 *   - 1 Besu for the Sequencer node (payload validator + sequencer combined)
 *   - 2 Besu for the Follower node:
 *     a) Payload Validator Besu (connected to Maru Follower)
 *     b) Follower Besu (configured in FollowersConfig)
 * - 2 Maru nodes: Sequencer, Follower
 */
class MaruManyFollowerElsTest {
  private lateinit var cluster: MaruCluster
  private lateinit var followerBesu: BesuNode
  private lateinit var followersConfig: FollowersConfig
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  /**
   * Adds the Maru follower, with its payload validator Besu, to the running cluster, which starts it immediately.
   */
  private fun startMaruFollower() {
    cluster.addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
      nodeBuilder
        .payloadValidation(true)
        .withFastPeering()
        .withSyncingConfig(TestSyncingConfigs.default)
        .staticPeers(listOf("sequencer"))
        .maruConfig { it.copy(allowEmptyBlocks = false, followers = followersConfig) }
    }

    // Wait for nodes to peer
    log.info("Waiting for Maru nodes to peer")
    cluster.maruNode("follower").awaitTillMaruHasPeers(1u)
    cluster.maruNode("sequencer").awaitTillMaruHasPeers(1u)
  }

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    transactionsHelper = BesuTransactionsHelper()

    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .withFastQbft()
          .withFastPeering()
          .withSyncingConfig(TestSyncingConfigs.validator)
          .maruConfig { it.copy(allowEmptyBlocks = false) }
      }.start()

    // The extra follower EL must use the cluster genesis, resolved when the sequencer Besu was started
    val clusterGenesis = cluster.besuNode("sequencer").configuration.genesisConfig.orElseThrow()
    followerBesu = BesuFactory.buildTestBesu(
      genesisFile = clusterGenesis,
      validator = false,
      nodeName = "extra-follower-besu",
    )
    cluster.besuCluster.addNodeAndStart(followerBesu)
    followersConfig =
      FollowersConfig(
        mapOf(
          "follower-besu" to
            ApiEndpointConfig(
              URI.create(followerBesu.engineRpcUrl().get()).toURL(),
            ),
        ),
      )
    // Note: We don't start the follower here - each test decides when to start it
  }

  @AfterEach
  fun tearDown() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  @Test
  fun `Maru follower with FollowersConfig imports blocks when started simultaneously with validator`() {
    startMaruFollower()

    val validatorGenesis = cluster.besuNode("sequencer").ethGetBlockByNumber("earliest", false)
    val followerGenesis = cluster.besuNode("follower").ethGetBlockByNumber("earliest", false)
    val followerBesuGenesis = followerBesu.ethGetBlockByNumber("earliest", false)

    assertThat(followerGenesis).isEqualTo(validatorGenesis)
    assertThat(followerBesuGenesis).isEqualTo(validatorGenesis)

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("recipient-account-$it"),
          amount = Amount.ether(100),
        )
      }
    }

    Checks.checkAllNodesHaveSameBlocks(
      blocksToProduce,
      cluster.besuNode("sequencer"),
      cluster.besuNode("follower"),
      followerBesu,
    )
  }

  @Test
  fun `Maru follower with FollowersConfig imports blocks when started after validator has produced blocks`() {
    val initialBlocksToProduce = 10
    log.info("Producing $initialBlocksToProduce blocks before starting Maru follower")
    repeat(initialBlocksToProduce) {
      transactionsHelper.run {
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("initial-recipient-$it"),
          amount = Amount.ether(50),
        )
      }
    }

    Checks.checkAllNodesHaveSameBlocks(
      initialBlocksToProduce,
      cluster.besuNode("sequencer"),
    )

    log.info("Starting Maru follower after $initialBlocksToProduce blocks have been produced")
    startMaruFollower()

    log.info("Waiting for Maru follower to sync initial $initialBlocksToProduce blocks")
    Checks.checkAllNodesHaveSameBlocks(
      initialBlocksToProduce,
      cluster.besuNode("sequencer"),
      cluster.besuNode("follower"),
      followerBesu,
    )

    val additionalBlocksToProduce = 5
    val totalBlocks = initialBlocksToProduce + additionalBlocksToProduce
    log.info("Producing $additionalBlocksToProduce additional blocks")
    repeat(additionalBlocksToProduce) {
      transactionsHelper.run {
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("additional-recipient-$it"),
          amount = Amount.ether(75),
        )
      }
    }

    Checks.checkAllNodesHaveSameBlocks(
      totalBlocks,
      cluster.besuNode("sequencer"),
      cluster.besuNode("follower"),
      followerBesu,
    )
  }
}
