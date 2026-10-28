/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import linea.testing.besu.BesuTransactionsHelper
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruFollowerNoElTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  private var validatorApiPort: UInt = 0u
  private var followerApiPort: UInt = 0u

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    transactionsHelper = BesuTransactionsHelper()

    // The follower has no EL node attached
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = false) }
      }.addNode(NodeRole.Follower) { nodeBuilder ->
        nodeBuilder
          .staticPeers(listOf("sequencer"))
      }.start()

    validatorApiPort = cluster.maruNode("sequencer").apiPort()
    followerApiPort = cluster.maruNode("follower").apiPort()

    log.info("Waiting for nodes to peer")
    await
      .atMost(30.seconds.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .until {
        cluster.maruNode("follower").peersConnected() >= 1u &&
          cluster.maruNode("sequencer").peersConnected() >= 1u
      }
  }

  @AfterEach
  fun tearDown() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  // TODO: Replace with a proper Beacon REST API client
  private val httpClient: HttpClient = HttpClient.newHttpClient()
  private val objectMapper = ObjectMapper()

  private data class ClBlockMetadata(
    val slot: ULong,
    val blockHash: String?,
  )

  private fun readHead(apiPort: UInt): ClBlockMetadata {
    val req =
      HttpRequest
        .newBuilder(URI.create("http://127.0.0.1:${apiPort.toInt()}/eth/v2/beacon/blocks/head"))
        .GET()
        .build()
    val resp = httpClient.send(req, HttpResponse.BodyHandlers.ofString())
    require(resp.statusCode() == 200) { "Unexpected status ${resp.statusCode()}: ${resp.body()}" }
    val root: JsonNode = objectMapper.readTree(resp.body())
    val message =
      root
        .path("data")
        .path("message")
    val slotStr =
      message
        .path("slot")
        .asText()
    require(slotStr.isNotBlank()) { "slot not found in response: ${resp.body()}" }
    val slot = slotStr.toULong()

    val blockHashStr =
      message
        .path("body")
        .path("attestations")
        .find { it.path("data").path("slot").asInt() == slot.toInt() }
        ?.path("data")
        ?.path("body_root")
        ?.asText()

    return ClBlockMetadata(slot, blockHashStr)
  }

  @Test
  fun `Maru follower is able to import blocks without EL`() {
    val blocksToProduce = 4 // Less than desync tolerance

    val initialFollowerHead = readHead(followerApiPort)
    val initialValidatorHead = readHead(validatorApiPort)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // Await until both validator and follower advanced and heads match
    await
      .pollInterval(1.seconds.toJavaDuration())
      .timeout(20.seconds.toJavaDuration())
      .untilAsserted {
        val validatorHead = readHead(validatorApiPort)
        val followerHead = readHead(followerApiPort)

        assertThat(validatorHead.blockHash).isNotNull
        assertThat(validatorHead.slot).isGreaterThanOrEqualTo(initialValidatorHead.slot + blocksToProduce.toULong())
        assertThat(followerHead.slot).isGreaterThanOrEqualTo(initialFollowerHead.slot + blocksToProduce.toULong())
        assertThat(followerHead).isEqualTo(validatorHead)
      }
  }
}
