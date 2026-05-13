/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.web3j.ethapi.createEthApiClient
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
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
import testutils.FakeL1JsonRpcServer
import testutils.PeeringNodeNetworkStack
import testutils.besu.BesuFactory
import testutils.besu.BesuTransactionsHelper
import testutils.besu.ethGetBlockByNumber
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruLineaFinalizationTest {
  private lateinit var cluster: Cluster
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()
  private lateinit var fakeL1: FakeL1JsonRpcServer
  private lateinit var validatorEthApiClient: EthApiClient
  private lateinit var followerEthApiClient: EthApiClient

  @BeforeEach
  fun setUp() {
    fakeL1 = FakeL1JsonRpcServer().start()
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

    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack, followerStack)

    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        l1EthApiEndpoint = fakeL1.url(),
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    val validatorP2pPort = validatorStack.p2pPort

    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorP2pPort,
        l1EthApiEndpoint = fakeL1.url(),
      )
    followerStack.setMaruApp(followerMaruApp)
    followerStack.maruApp.start().get()

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
    val validatorGenesis = validatorStack.besuNode.ethGetBlockByNumber("earliest", false)
    val followerGenesis = followerStack.besuNode.ethGetBlockByNumber("earliest", false)

    assertThat(validatorGenesis).isEqualTo(followerGenesis)

    validatorEthApiClient = createEthApiClient(
      rpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
      log = LogManager.getLogger("clients.l2.test.validator"),
      requestRetryConfig = null,
      vertx = null,
    )
    followerEthApiClient = createEthApiClient(
      rpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
      log = LogManager.getLogger("clients.l2.test.follower"),
      requestRetryConfig = null,
      vertx = null,
    )
    // wait for Besu to be fully started and synced,
    // to avoid CI flakiness due low resources sometimes
    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          validatorEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(0UL)
        assertThat(
          followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(0UL)
      }
  }

  @AfterEach
  fun tearDown() {
    followerStack.maruApp.stop().get()
    validatorStack.maruApp.stop().get()
    followerStack.maruApp.close()
    validatorStack.maruApp.close()
    cluster.close()
    fakeL1.stop()
  }

  @Test
  fun `should finalize current block right away when syncing and behind finalized on L1`() {
    fakeL1.setFinalizedL2BlockNumber(4UL)

    repeat(3) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(1),
        )
      }
    }

    await
      .atMost(5.seconds.toJavaDuration())
      .ignoreExceptions() // sometimes besu fails to return the latest block ¯\_(ツ)_/¯
      .until {
        followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number == 3UL
      }

    await
      .atMost(15.seconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          validatorEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get().number,
        ).isBetween(2UL, 3UL)
        assertThat(
          followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get().number,
        ).isBetween(2UL, 3UL)
      }

    // Propagating the Head of the chain further than the Finalization height
    repeat(4) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(1),
        )
      }
    }

    await
      .atMost(20.seconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number)
          .isGreaterThan(6UL)
      }

    await
      .atMost(20.seconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          validatorEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get().number,
        ).isEqualTo(4UL)
        assertThat(
          followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get().number,
        ).isEqualTo(4UL)
      }
  }
}
