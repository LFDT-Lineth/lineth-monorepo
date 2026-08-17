/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import linea.testing.besu.BesuTransactionsHelper
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
import org.junit.jupiter.api.io.TempDir
import testutils.SingleNodeNetworkStack
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import java.io.File
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruFollowerNoElTest {
  private lateinit var cluster: Cluster
  private lateinit var validatorStack: SingleNodeNetworkStack

  @TempDir
  private lateinit var maruFollowerDataDir: File
  private lateinit var maruFollower: MaruApp
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()

  private var validatorApiPort: UInt = 0u
  private var followerApiPort: UInt = 0u

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

    validatorStack = SingleNodeNetworkStack(
      cluster = cluster,
    ) { ethereumJsonRpcBaseUrl, engineRpcUrl, tmpDir ->
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = ethereumJsonRpcBaseUrl,
        engineApiRpc = engineRpcUrl,
        dataDir = tmpDir,
        apiPort = 0u,
        startApiServer = true,
      )
    }

    // Start all Besu nodes together for proper peering
    validatorStack.maruApp.start().get()

    // Discover actual validator API port
    validatorApiPort = validatorStack.maruApp.apiPort()

    // Get the validator's p2p port after it's started
    val validatorP2pPort = validatorStack.p2pPort

    maruFollower = maruFactory.buildTestMaruFollowerWithP2pPeering(
      ethereumJsonRpcUrl = null,
      engineApiRpc = null,
      dataDir = maruFollowerDataDir.toPath(),
      validatorPortForStaticPeering = validatorP2pPort,
      syncingConfig = MaruFactory.defaultSyncingConfig,
      enablePayloadValidation = false,
      apiPort = 0u,
      startApiServer = true,
    )

    maruFollower.start().get()

    // Discover actual follower API port
    followerApiPort = maruFollower.apiPort()

    log.info("Nodes are peered")
    maruFollower.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
  }

  @AfterEach
  fun tearDown() {
    maruFollower.stop().get()
    validatorStack.maruApp.stop().get()
    validatorStack.maruApp.close()
    cluster.close()
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
        validatorStack.besuNode.sendTransactionAndAssertExecution(
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
