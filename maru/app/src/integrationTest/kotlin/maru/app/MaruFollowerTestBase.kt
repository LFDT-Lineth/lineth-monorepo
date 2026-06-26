/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.ethGetBlockByNumber
import maru.config.SyncingConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import testutils.Checks.checkAllNodesHaveSameBlocks
import testutils.PeeringNodeNetworkStack
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * Shared validator+follower fixture for the Maru follower integration tests.
 *
 * The concrete suites are split across classes (one per scenario family) so they run in separate
 * Gradle forks in parallel instead of sequentially in a single fork: the "import blocks" scenarios
 * live in [MaruFollowerTest] and the longer syncing scenarios in [MaruFollowerSyncingTest]. Each
 * test method gets a fresh cluster + stacks via [setUp].
 */
abstract class MaruFollowerTestBase {
  protected lateinit var cluster: Cluster
  protected lateinit var validatorStack: PeeringNodeNetworkStack
  protected lateinit var followerStack: PeeringNodeNetworkStack
  protected lateinit var transactionsHelper: BesuTransactionsHelper
  protected val log = LogManager.getLogger(this.javaClass)
  protected val maruFactory = MaruFactory()

  protected fun setupMaruHelper(
    syncingConfig: SyncingConfig = MaruFactory.defaultSyncingConfig,
    payloadValidationEnabled: Boolean = true,
  ) {
    // Create and start validator Maru app first
    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        syncingConfig = syncingConfig,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    // Get the validator's p2p port after it's started
    val validatorP2pPort = validatorStack.p2pPort

    // Create follower Maru app with the validator's p2p port for static peering
    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorP2pPort,
        syncingConfig = syncingConfig,
        enablePayloadValidation = payloadValidationEnabled,
      )
    followerStack.setMaruApp(followerMaruApp)
    followerStack.maruApp.start().get()

    log.info("Nodes are peered")
    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
    val validatorGenesis = validatorStack.besuNode.ethGetBlockByNumber("earliest", false)
    val followerGenesis = followerStack.besuNode.ethGetBlockByNumber("earliest", false)

    assertThat(validatorGenesis).isEqualTo(followerGenesis)
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

    // Start all Besu nodes together for proper peering
    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack, followerStack)
  }

  @AfterEach
  fun tearDown() {
    followerStack.maruApp.stop().get()
    validatorStack.maruApp.stop().get()
    followerStack.maruApp.close()
    validatorStack.maruApp.close()
    cluster.close()
  }

  protected fun checkValidatorAndFollowerBlocks(
    blocksToProduce: Int,
    timeout: Duration = 30.seconds,
  ) {
    checkAllNodesHaveSameBlocks(
      expectedBlockCount = blocksToProduce,
      validatorStack.besuNode,
      followerStack.besuNode,
      timeout = timeout,
    )
  }

  protected fun checkNetworkStacksBlocksProduced(
    blocksProduced: Int,
    vararg stacks: PeeringNodeNetworkStack,
  ) {
    checkAllNodesHaveSameBlocks(blocksProduced, *stacks.map { it.besuNode }.toTypedArray())
  }
}
