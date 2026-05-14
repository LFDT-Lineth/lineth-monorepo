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
import linea.timer.JvmTimerFactory
import linea.timer.TimerFactory
import linea.web3j.ethapi.createEthApiClient
import maru.config.P2PConfig
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.database.P2PState
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.messages.BlockRetrievalStrategy
import maru.p2p.messages.DefaultBlockRetrievalStrategy
import maru.p2p.messages.StatusManager
import maru.serialization.SerDe
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.plugin.services.MetricsSystem
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import testutils.FourEmptyResponsesStrategy
import testutils.MisbehavingP2PNetwork
import testutils.PeeringNodeNetworkStack
import testutils.TimeOutResponsesStrategy
import testutils.besu.BesuFactory
import testutils.besu.BesuTransactionsHelper
import testutils.besu.ethGetBlockByNumber
import testutils.maru.MaruFactory
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruPeerScoringTest {
  private lateinit var besuCluster: Cluster
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()
  private lateinit var validatorEthApiClient: EthApiClient
  private lateinit var followerEthApiClient: EthApiClient
  private val timerFactory = JvmTimerFactory()

  @AfterEach
  fun tearDown() {
    if (::followerStack.isInitialized) {
      followerStack.maruApp.stop().get()
      followerStack.maruApp.close()
    }
    if (::validatorStack.isInitialized) {
      validatorStack.maruApp.stop().get()
      validatorStack.maruApp.close()
    }
    if (::besuCluster.isInitialized) {
      besuCluster.close()
    }
  }

  @Test
  fun `node gets in sync with default block retrieval strategy`() {
    setUpNodes(blockRetrievalStrategy = DefaultBlockRetrievalStrategy(), timerFactory = timerFactory)

    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(15UL)
      }
  }

  @Test
  fun `node disconnects validator when receiving too many empty responses`() {
    val maruNodeSetup =
      setUpNodes(
        blockRetrievalStrategy = FourEmptyResponsesStrategy(),
        banPeriod = 10.seconds,
        followerCooldownPeriod = 10.minutes,
        timerFactory = timerFactory,
      )

    // Wait until it is disconnected because of empty responses
    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(250.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(maruNodeSetup.followerMaruApp.p2pNetwork.peerCount).isEqualTo(0)
      }
    // reconnects after ban period and finishes syncing
    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(18UL)
      }
  }

  @Test
  fun `node disconnects validator when requests time out`() {
    val timeout = 3.seconds
    val delay = timeout + 2.seconds
    val maruNodeSetup =
      setUpNodes(
        blockRangeRequestTimeout = timeout,
        blockRetrievalStrategy = TimeOutResponsesStrategy(delay = delay),
        validatorCooldownPeriod = 20.seconds,
        timerFactory = timerFactory,
      )

    // Wait for the follower to dial and connect to the validator. Previously this was a fixed
    // `sleep(timeout - 1s)` followed by `assertEquals(1)`; that races on slow CI when peering
    // takes longer than the sleep window, observing peerCount=0 before the connection completes.
    await.atMost(10.seconds.toJavaDuration()).untilAsserted {
      assertThat(maruNodeSetup.followerMaruApp.p2pNetwork.peerCount).isEqualTo(1)
    }

    // After the request timeout fires, the follower should disconnect the unresponsive validator.
    await.atMost((delay + 5.seconds).toJavaDuration()).untilAsserted {
      assertThat(maruNodeSetup.followerMaruApp.p2pNetwork.peerCount).isEqualTo(0)
    }
  }

  data class MaruNodeSetup(
    val validatorMaruApp: MaruApp,
    val followerMaruApp: MaruApp,
  )

  fun setUpNodes(
    blockRetrievalStrategy: BlockRetrievalStrategy,
    banPeriod: Duration = 10.seconds,
    validatorCooldownPeriod: Duration = 10.seconds,
    followerCooldownPeriod: Duration = 10.seconds,
    blockRangeRequestTimeout: Duration = 5.seconds,
    timerFactory: TimerFactory,
  ): MaruNodeSetup {
    transactionsHelper = BesuTransactionsHelper()
    besuCluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

    validatorStack = PeeringNodeNetworkStack()
    followerStack = PeeringNodeNetworkStack(
      besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
    )

    PeeringNodeNetworkStack.startBesuNodes(besuCluster, validatorStack, followerStack)

    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithDiscovery(
        allowEmptyBlocks = true,
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        p2pPort = 0u,
        discoveryPort = 0u,
        cooldownPeriod = validatorCooldownPeriod,
        p2pNetworkFactory = {
            privateKeyBytes: ByteArray,
            p2pConfig: P2PConfig,
            chainId: UInt,
            serDe: SerDe<SealedBeaconBlock>,
            metricsFacade: MetricsFacade,
            metricsSystem: MetricsSystem,
            statusManager: StatusManager,
            chain: BeaconChain,
            forkIdHashManager: ForkPeeringManager,
            isBlockImportEnabledProvider: () -> Boolean,
            p2pState: P2PState,
            timerFactory: TimerFactory,
          ->
          MisbehavingP2PNetwork(
            privateKeyBytes = privateKeyBytes,
            p2pConfig = p2pConfig,
            chainId = chainId,
            serDe = serDe,
            metricsFacade = metricsFacade,
            metricsSystem = metricsSystem,
            statusManager = statusManager,
            chain = chain,
            forkIdHashManager = forkIdHashManager,
            isBlockImportEnabledProvider = isBlockImportEnabledProvider,
            p2pState = p2pState,
            blockRetrievalStrategy = blockRetrievalStrategy,
            timerFactory = timerFactory,
          ).p2pNetwork
        },
      )

    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    val bootnodeEnr =
      validatorStack.maruApp.p2pNetwork.localNodeRecord
        ?.asEnr()
    log.info("Validator ENR: $bootnodeEnr")

    validatorEthApiClient = createEthApiClient(
      rpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
      log = LogManager.getLogger("clients.l2.test.validator"),
      requestRetryConfig = null,
      vertx = null,
    )

    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          validatorEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(0UL)
      }

    val validatorGenesis = validatorStack.besuNode.ethGetBlockByNumber("earliest", false)
    val followerGenesis = followerStack.besuNode.ethGetBlockByNumber("earliest", false)
    assertThat(validatorGenesis).isEqualTo(followerGenesis)

    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithDiscovery(
        allowEmptyBlocks = true,
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        bootnode = bootnodeEnr,
        p2pPort = 0u,
        discoveryPort = 0u,
        banPeriod = banPeriod,
        cooldownPeriod = followerCooldownPeriod,
        blockRangeRequestTimeout = blockRangeRequestTimeout,
      )
    followerStack.setMaruApp(followerMaruApp)

    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          validatorEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(10UL)
      }

    followerStack.maruApp.start().get()

    followerEthApiClient = createEthApiClient(
      rpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
      log = LogManager.getLogger("clients.l2.test.follower"),
      requestRetryConfig = null,
      vertx = null,
    )
    // wait for Besu to be fully started,
    // to avoid CI flakiness due to low resources sometimes
    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          followerEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(0UL)
      }
    return MaruNodeSetup(validatorMaruApp = validatorMaruApp, followerMaruApp = followerMaruApp)
  }
}
