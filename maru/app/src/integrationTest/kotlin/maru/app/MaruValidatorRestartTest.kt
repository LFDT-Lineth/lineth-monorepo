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
import maru.test.util.NetworkUtil.findFreePorts
import org.apache.logging.log4j.LogManager
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.assertMinedBlocks
import testutils.PeeringNodeNetworkStack
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruValidatorRestartTest {
  private lateinit var cluster: Cluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private val maruFactory = MaruFactory()
  private val log = LogManager.getLogger(this.javaClass)

  @BeforeEach
  fun setup() {
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )
    transactionsHelper = BesuTransactionsHelper()
    validatorStack = PeeringNodeNetworkStack()
    followerStack = PeeringNodeNetworkStack(
      besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
    )
  }

  @AfterEach
  fun tearDown() {
    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()
    validatorStack.maruApp.stop().get()
    validatorStack.maruApp.close()
    cluster.close()
  }

  @Test
  fun `Maru validator restarted from scratch is able to sync state`() {
    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack, followerStack)
    val freePorts = findFreePorts(6)
    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithDiscovery(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        p2pPort = freePorts[0],
        discoveryPort = freePorts[1],
      )
    followerStack.setMaruApp(followerMaruApp)
    followerStack.maruApp.start().get()

    val followerENR =
      followerStack.maruApp.p2pNetwork.localNodeRecord
        ?.asEnr()

    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithDiscovery(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        p2pPort = freePorts[2],
        discoveryPort = freePorts[3],
        bootnode = followerENR,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    log.info(
      "Follower: ${followerStack.maruApp.p2pNetwork.nodeId}, validator: ${validatorStack.maruApp.p2pNetwork.nodeId}",
    )

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    validatorStack.besuNode.assertMinedBlocks(blocksToProduce)
    followerStack.besuNode.assertMinedBlocks(blocksToProduce)

    validatorStack.maruApp.stop().get()
    validatorStack.maruApp.close()

    await
      .timeout(30.seconds.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .until { followerStack.maruApp.peersConnected() == 0u }

    val newValidatorMaruApp =
      maruFactory.buildTestMaruValidatorWithDiscovery(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        p2pPort = freePorts[4],
        discoveryPort = freePorts[5],
        bootnode = followerENR,
      )
    validatorStack.setMaruApp(newValidatorMaruApp)
    validatorStack.maruApp.start().get()

    log.info("Restarted validator: ${newValidatorMaruApp.p2pNetwork.nodeId}")

    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
    followerStack.maruApp.awaitTillMaruHasPeers(1u)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    validatorStack.besuNode.assertMinedBlocks(2 * blocksToProduce)
    followerStack.besuNode.assertMinedBlocks(2 * blocksToProduce)
  }
}
