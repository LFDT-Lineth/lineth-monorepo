/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.web3j.ethapi.createEthApiClient
import maru.config.SyncingConfig
import maru.p2p.messages.BlockRetrievalStrategy
import maru.p2p.messages.DefaultBlockRetrievalStrategy
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeBuilder
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
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.FourEmptyResponsesStrategy
import testutils.MisbehavingP2PNetwork
import testutils.TimeOutResponsesStrategy
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruPeerScoringTest {
  private lateinit var cluster: MaruCluster
  private val log = LogManager.getLogger(this.javaClass)
  private lateinit var validatorEthApiClient: EthApiClient
  private lateinit var followerEthApiClient: EthApiClient

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
  }

  @AfterEach
  fun tearDown() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  @Test
  fun `node gets in sync with default block retrieval strategy`() {
    setUpNodes(blockRetrievalStrategy = DefaultBlockRetrievalStrategy())

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
    setUpNodes(
      blockRetrievalStrategy = FourEmptyResponsesStrategy(),
      banPeriod = 10.seconds,
      followerCooldownPeriod = 10.minutes,
    )

    // Wait until it is disconnected because of empty responses
    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(250.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(cluster.maruNode("follower").p2pNetwork.peerCount).isEqualTo(0)
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
    setUpNodes(
      blockRangeRequestTimeout = timeout,
      blockRetrievalStrategy = TimeOutResponsesStrategy(delay = delay),
      validatorCooldownPeriod = 20.seconds,
    )

    // Wait for the follower to dial and connect to the validator. Previously this was a fixed
    // `sleep(timeout - 1s)` followed by `assertEquals(1)`; that races on slow CI when peering
    // takes longer than the sleep window, observing peerCount=0 before the connection completes.
    await.atMost(10.seconds.toJavaDuration()).untilAsserted {
      assertThat(cluster.maruNode("follower").p2pNetwork.peerCount).isEqualTo(1)
    }

    // After the request timeout fires, the follower should disconnect the unresponsive validator.
    await.atMost((delay + 5.seconds).toJavaDuration()).untilAsserted {
      assertThat(cluster.maruNode("follower").p2pNetwork.peerCount).isEqualTo(0)
    }
  }

  // Fast peering with the given reputation periods, and syncing one block at a time with the given request timeout
  private fun NodeBuilder.withPeeringTimings(
    banPeriod: Duration,
    cooldownPeriod: Duration,
    blockRangeRequestTimeout: Duration,
  ): NodeBuilder =
    withFastPeering(banPeriod = banPeriod, cooldownPeriod = cooldownPeriod)
      .withSyncingConfig(
        TestSyncingConfigs.validator.copy(
          download = SyncingConfig.Download(blockRangeRequestTimeout = blockRangeRequestTimeout),
        ),
      )

  private fun setUpNodes(
    blockRetrievalStrategy: BlockRetrievalStrategy,
    banPeriod: Duration = 10.seconds,
    validatorCooldownPeriod: Duration = 10.seconds,
    followerCooldownPeriod: Duration = 10.seconds,
    blockRangeRequestTimeout: Duration = 5.seconds,
  ) {
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .maruConfig { it.copy(allowEmptyBlocks = true) }
          .withPeeringTimings(
            banPeriod = 10.seconds,
            cooldownPeriod = validatorCooldownPeriod,
            blockRangeRequestTimeout = 5.seconds,
          )
          .withFastQbft()
          .withP2pNetworkFactory {
              privateKeyBytes,
              p2pConfig,
              chainId,
              blockHashing,
              metricsFacade,
              metricsSystem,
              statusManager,
              chain,
              forkIdHashManager,
              isBlockImportEnabledProvider,
              p2pState,
              timerFactory,
            ->
            MisbehavingP2PNetwork(
              privateKeyBytes = privateKeyBytes,
              p2pConfig = p2pConfig,
              chainId = chainId,
              blockHashing = blockHashing,
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
          }
      }.start()
    val validatorBesuNode = cluster.besuNode("sequencer")
    log.info("Validator ENR: ${cluster.maruNode("sequencer").p2pNetwork.enr}")

    validatorEthApiClient = createEthApiClient(
      rpcUrl = validatorBesuNode.jsonRpcBaseUrl().get(),
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

    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .ignoreExceptions()
      .untilAsserted {
        assertThat(
          validatorEthApiClient.ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get().number,
        ).isGreaterThanOrEqualTo(10UL)
      }

    // adding a node to a running cluster starts it immediately
    cluster.addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
      nodeBuilder
        .payloadValidation(true)
        .maruConfig { it.copy(allowEmptyBlocks = true) }
        .withPeeringTimings(
          banPeriod = banPeriod,
          cooldownPeriod = followerCooldownPeriod,
          blockRangeRequestTimeout = blockRangeRequestTimeout,
        ).bootnodes(listOf("sequencer"))
    }
    val followerBesuNode = cluster.besuNode("follower")

    cluster.assertSameCanonicalElChain(chainHeight = 0UL)

    followerEthApiClient = createEthApiClient(
      rpcUrl = followerBesuNode.jsonRpcBaseUrl().get(),
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
  }
}
