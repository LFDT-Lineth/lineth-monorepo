/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.ethGetBlockByNumber
import maru.config.SyncingConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import testutils.Checks.checkAllNodesHaveSameBlocks
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * One Maru validator and one Maru follower, each backed by its own Besu node, peered over P2P.
 *
 * Shared by the MaruFollower*Test classes, which are split by scenario so that Gradle runs them in parallel forks.
 */
class ValidatorFollowerNetwork {
  private val log = LogManager.getLogger(this.javaClass)
  private val transactionsHelper = BesuTransactionsHelper()
  val maruFactory = MaruFactory()
  val cluster =
    Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )
  val validatorStack = PeeringNodeNetworkStack()
  val followerStack =
    PeeringNodeNetworkStack(
      besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
    )

  fun startBesuNodes() {
    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack, followerStack)
  }

  fun close() {
    followerStack.maruApp.stop().get()
    validatorStack.maruApp.stop().get()
    followerStack.maruApp.close()
    validatorStack.maruApp.close()
    cluster.close()
  }

  fun startMaruNodes(
    syncingConfig: SyncingConfig = MaruFactory.defaultSyncingConfig,
    payloadValidationEnabled: Boolean = true,
    validatorP2pPort: UInt = 0u,
  ) {
    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        syncingConfig = syncingConfig,
        p2pPort = validatorP2pPort,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
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

  fun stopFollowerMaru() {
    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()
  }

  fun startFollowerMaru(syncingConfig: SyncingConfig = MaruFactory.defaultSyncingConfig) {
    followerStack.setMaruApp(
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
        syncingConfig = syncingConfig,
      ),
    )
    followerStack.maruApp.start().get()
  }

  fun produceBlocks(blockCount: Int) {
    repeat(blockCount) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
  }

  fun checkValidatorAndFollowerBlocks(
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

  fun checkNetworkStacksBlocksProduced(
    blocksProduced: Int,
    vararg stacks: PeeringNodeNetworkStack,
  ) {
    checkAllNodesHaveSameBlocks(blocksProduced, *stacks.map { it.besuNode }.toTypedArray())
  }
}
