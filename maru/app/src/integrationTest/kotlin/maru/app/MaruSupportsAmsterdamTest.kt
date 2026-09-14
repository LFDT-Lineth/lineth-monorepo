/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.kotlin.encodeHex
import maru.config.ApiEndpointConfig
import maru.config.QbftConfig
import maru.config.ValidatorElNode
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeBuilder
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.headBeaconBlockNumber
import maru.test.extensions.headElBlock
import org.apache.logging.log4j.Level
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.DefaultBlockParameter
import java.net.URI
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

class MaruSupportsAmsterdamTest {
  private lateinit var cluster: MaruCluster

  @BeforeEach
  fun beforeEach() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf(
        "maru" to Level.INFO,
        "maru.clients" to Level.DEBUG,
        "org.hyperledger.besu.ethereum.api.jsonrpc.internal.methods.engine" to Level.DEBUG,
      ),
    )
  }

  @AfterEach
  fun afterEach() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  @Test
  fun `should run network starting from Amsterdam`() {
    cluster = MaruCluster(
      blockTimeSeconds = 1u,
      chainForks = mapOf(
        Instant.fromEpochSeconds(0) to ChainFork(clFork = ClFork.QBFT_PHASE0, elFork = ElFork.Amsterdam),
      ),
    ).addNode(NodeRole.Sequencer, withBesuEl = true, configurator = ::configureSequencer)
      .addNode(NodeRole.Follower, withBesuEl = true) {
        it.staticPeers(listOf("sequencer"))
      }.start()

    await().atMost(60.seconds.toJavaDuration()).untilAsserted {
      assertThat(cluster.node("sequencer").maru.headBeaconBlockNumber()).isGreaterThan(5UL)
    }
    await().atMost(60.seconds.toJavaDuration()).untilAsserted {
      assertThat(cluster.node("follower").maru.headBeaconBlockNumber()).isGreaterThan(5UL)
    }
    assertExecutionLayersAgree()
  }

  @Test
  fun `should switch from Osaka to Amsterdam`() {
    val amsterdamTimestamp = Clock.System.now().plus(20.seconds)
    cluster = MaruCluster(
      chainForks = mapOf(
        Instant.fromEpochSeconds(0) to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Osaka,
          ),
        amsterdamTimestamp to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Amsterdam,
          ),
      ),
    ).addNode(NodeRole.Sequencer, withBesuEl = true, configurator = ::configureSequencer)
      .addNode(NodeRole.Follower, withBesuEl = true) {
        it.staticPeers(listOf("sequencer"))
      }.start()

    await().atMost(60.seconds.toJavaDuration()).untilAsserted {
      assertThat(
        cluster
          .node("sequencer")
          .maru
          .headElBlock()
          .timestamp,
      ).isGreaterThanOrEqualTo(amsterdamTimestamp.epochSeconds.toULong() + 5UL)
    }
    await().atMost(60.seconds.toJavaDuration()).untilAsserted {
      assertThat(
        cluster
          .node("follower")
          .maru
          .headElBlock()
          .timestamp,
      ).isGreaterThanOrEqualTo(amsterdamTimestamp.epochSeconds.toULong() + 5UL)
    }
    assertExecutionLayersAgree()
  }

  /** The cluster replaces the template endpoint and fee recipient after starting Besu. */
  private fun configureSequencer(node: NodeBuilder) {
    node.maruConfig { config ->
      config.copy(
        qbft = QbftConfig(feeRecipient = ByteArray(20), targetGasLimit = 30_000_000UL),
        validatorElNode = ValidatorElNode(
          engineApiEndpoint = ApiEndpointConfig(URI.create("http://localhost:8551").toURL()),
          payloadValidationEnabled = true,
        ),
      )
    }
  }

  private fun assertExecutionLayersAgree() {
    val payload = cluster.node("sequencer").maru.headElBlock()
    assertThat(payload.slotNumber).isNotNull()
    assertThat(payload.blockAccessList).isNotEmpty()
    assertThat(payload.gasLimit).isEqualTo(30_000_000UL)
    await().atMost(60.seconds.toJavaDuration()).untilAsserted {
      for (label in listOf("sequencer", "follower")) {
        val block = cluster.besuNode(label).nodeRequests().eth()
          .ethGetBlockByNumber(DefaultBlockParameter.valueOf(payload.blockNumber.toString().toBigInteger()), false)
          .send().block
        assertThat(block).isNotNull()
        assertThat(block.hash).isEqualTo(payload.blockHash.encodeHex())
      }
    }
  }
}
