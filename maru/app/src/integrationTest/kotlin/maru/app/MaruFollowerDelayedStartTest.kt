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
import org.apache.logging.log4j.LogManager
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

class MaruFollowerDelayedStartTest {
  private lateinit var cluster: Cluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
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
  fun `follower can sync and accept transactions after delayed start`() {
    validatorStack = PeeringNodeNetworkStack()
    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack)

    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    val blocksToProduce = 10
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

    followerStack = PeeringNodeNetworkStack(
      besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
    )
    cluster.addNode(followerStack.besuNode)

    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
      )
    followerStack.setMaruApp(followerMaruApp)
    followerStack.maruApp.start().get()

    followerStack.besuNode.assertMinedBlocks(blocksToProduce)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        followerStack.besuNode.sendTransactionAndAssertExecution(
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
