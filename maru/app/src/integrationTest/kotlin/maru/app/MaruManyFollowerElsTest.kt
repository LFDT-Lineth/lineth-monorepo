/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.ApiEndpointConfig
import maru.config.FollowersConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks
import testutils.PeeringNodeNetworkStack
import testutils.besu.BesuFactory
import testutils.besu.BesuTransactionsHelper
import testutils.besu.ethGetBlockByNumber
import testutils.besu.startWithRetry
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import java.net.URI

/**
 * Integration tests for Maru with multiple follower execution layer nodes configured via FollowersConfig.
 * Tests verify that blocks are correctly propagated to all follower EL nodes.
 *
 * Test Architecture:
 * - 3 Besu nodes total:
 *   - 1 Besu for Validator Stack (payload validator + sequencer combined)
 *   - 2 Besu for Follower Stack:
 *     a) Payload Validator Besu (connected to Maru Follower)
 *     b) Follower Besu (configured in FollowersConfig)
 * - 2 Maru nodes: Validator, Follower
 */
class MaruManyFollowerElsTest {
  private lateinit var cluster: Cluster
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private lateinit var followerBesu: BesuNode
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()

  /**
   * Starts the Maru follower that was already instantiated in setUp().
   */
  private fun startMaruFollower() {
    followerStack.maruApp.start().get()

    // Wait for nodes to peer
    log.info("Waiting for Maru nodes to peer")
    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
  }

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

    validatorStack = PeeringNodeNetworkStack()

    followerStack = PeeringNodeNetworkStack(
      besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
    )

    followerBesu = BesuFactory.buildTestBesu(validator = false)

    // Start all 3 Besu nodes together
    cluster.startWithRetry(
      validatorStack.besuNode,
      followerStack.besuNode,
      followerBesu,
    )
    val followersConfig =
      FollowersConfig(
        mapOf(
          "follower-besu" to
            ApiEndpointConfig(
              URI.create(followerBesu.engineRpcUrl().get()).toURL(),
            ),
        ),
      )

    val validatorMaruApp =
      maruFactory.buildSwitchableTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()
    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
        followers = followersConfig,
      )
    followerStack.setMaruApp(followerMaruApp)
    // Note: We don't start the follower here - each test decides when to start it
  }

  @AfterEach
  fun tearDown() {
    followerStack.maruApp.stop().get()
    validatorStack.maruApp.stop().get()
    followerStack.maruApp.close()
    validatorStack.maruApp.close()
    cluster.close()
  }

  @Test
  fun `Maru follower with FollowersConfig imports blocks when started simultaneously with validator`() {
    startMaruFollower()

    val validatorGenesis = validatorStack.besuNode.ethGetBlockByNumber("earliest", false)
    val followerGenesis = followerStack.besuNode.ethGetBlockByNumber("earliest", false)
    val followerBesuGenesis = followerBesu.ethGetBlockByNumber("earliest", false)

    assertThat(followerGenesis).isEqualTo(validatorGenesis)
    assertThat(followerBesuGenesis).isEqualTo(validatorGenesis)

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("recipient-account-$it"),
          amount = Amount.ether(100),
        )
      }
    }

    Checks.checkAllNodesHaveSameBlocks(
      blocksToProduce,
      validatorStack.besuNode,
      followerStack.besuNode,
      followerBesu,
    )
  }

  @Test
  fun `Maru follower with FollowersConfig imports blocks when started after validator has produced blocks`() {
    val initialBlocksToProduce = 10
    log.info("Producing $initialBlocksToProduce blocks before starting Maru follower")
    repeat(initialBlocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("initial-recipient-$it"),
          amount = Amount.ether(50),
        )
      }
    }

    Checks.checkAllNodesHaveSameBlocks(
      initialBlocksToProduce,
      validatorStack.besuNode,
    )

    log.info("Starting Maru follower after $initialBlocksToProduce blocks have been produced")
    startMaruFollower()

    log.info("Waiting for Maru follower to sync initial $initialBlocksToProduce blocks")
    Checks.checkAllNodesHaveSameBlocks(
      initialBlocksToProduce,
      validatorStack.besuNode,
      followerStack.besuNode,
      followerBesu,
    )

    val additionalBlocksToProduce = 5
    val totalBlocks = initialBlocksToProduce + additionalBlocksToProduce
    log.info("Producing $additionalBlocksToProduce additional blocks")
    repeat(additionalBlocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("additional-recipient-$it"),
          amount = Amount.ether(75),
        )
      }
    }

    Checks.checkAllNodesHaveSameBlocks(
      totalBlocks,
      validatorStack.besuNode,
      followerStack.besuNode,
      followerBesu,
    )
  }
}
