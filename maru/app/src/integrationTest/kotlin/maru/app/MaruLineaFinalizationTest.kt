/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.testing.besu.BesuTransactionsHelper
import linea.web3j.ethapi.createEthApiClient
import maru.config.ApiEndpointConfig
import maru.config.LineaConfig
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
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.FakeL1JsonRpcServer
import testutils.maru.awaitTillMaruHasPeers
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruLineaFinalizationTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private lateinit var fakeL1: FakeL1JsonRpcServer
  private lateinit var validatorEthApiClient: EthApiClient
  private lateinit var followerEthApiClient: EthApiClient

  // Linea finalization against the fake L1, reading L2 blocks from the node's own EL
  private fun NodeBuilder.withLineaFinalization(): NodeBuilder =
    maruConfig { config ->
      config.copy(
        linea = LineaConfig(
          contractAddress = ByteArray(20), // dummy address — fake L1 ignores it
          l1EthApiEndpoint = ApiEndpointConfig(URI.create(fakeL1.url()).toURL()),
          l1PollingInterval = 100.milliseconds,
          l2EthApiEndpoint = config.forkTransition.l2EthApiEndpoint!!,
        ),
      )
    }

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    fakeL1 = FakeL1JsonRpcServer().start()
    transactionsHelper = BesuTransactionsHelper()

    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .withFastQbft()
          .withFastPeering()
          .withSyncingConfig(TestSyncingConfigs.validator)
          .withLineaFinalization()
          .maruConfig { it.copy(allowEmptyBlocks = false) }
      }.addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .withFastPeering()
          .withSyncingConfig(TestSyncingConfigs.default)
          .staticPeers(listOf("sequencer"))
          .payloadValidation(true)
          .withLineaFinalization()
          .maruConfig { it.copy(allowEmptyBlocks = false) }
      }.start()

    cluster.maruNode("follower").awaitTillMaruHasPeers(1u)
    cluster.maruNode("sequencer").awaitTillMaruHasPeers(1u)
    cluster.assertSameCanonicalElChain(chainHeight = 0UL)

    validatorEthApiClient = createEthApiClient(
      rpcUrl = cluster.besuNode("sequencer").jsonRpcBaseUrl().get(),
      log = LogManager.getLogger("clients.l2.test.validator"),
      requestRetryConfig = null,
      vertx = null,
    )
    followerEthApiClient = createEthApiClient(
      rpcUrl = cluster.besuNode("follower").jsonRpcBaseUrl().get(),
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
    if (::cluster.isInitialized) {
      cluster.stop()
    }
    if (::fakeL1.isInitialized) {
      fakeL1.stop()
    }
  }

  @Test
  fun `should finalize current block right away when syncing and behind finalized on L1`() {
    fakeL1.setFinalizedL2BlockNumber(4UL)

    repeat(3) {
      transactionsHelper.run {
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
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
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
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
