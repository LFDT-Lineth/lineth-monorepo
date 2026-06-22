/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package chaos

import chaos.SetupHelper.getNodesUrlsFromFile
import linea.kotlin.toULong
import linea.log4j.configureLoggers
import linea.web3j.createWeb3jHttpClient
import maru.api.beacon.SignedBeaconBlock
import maru.clients.beacon.Http4kBeaconChainClient
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.testing.filesystem.getPathTo
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.fail
import org.awaitility.kotlin.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class NodesSyncTest {
  private val log = LogManager.getLogger("maru.chaos.NodesSyncTest")

  fun getElNodeChainHead(elApiUrl: String): SafeFuture<ULong> {
    // createEthApiClient(elApiUrl, vertx = null, requestRetryConfig = null)
    //   .findBlockByNumber(B)
    return createWeb3jHttpClient(elApiUrl)
      .ethBlockNumber()
      .sendAsync()
      .toSafeFuture()
      .thenApply { it.blockNumber.toULong() }
  }

  private fun getElNodeChainHeads(nodes: List<NodeInfo<String>>): SafeFuture<List<NodeInfo<ULong>>> =
    nodes
      .map { node ->
        getElNodeChainHead(node.value)
          .thenApply { blockNumber -> node.map(blockNumber) }
      }.let { futures: List<SafeFuture<NodeInfo<ULong>>> ->
        SafeFuture.collectAll(futures.stream())
      }

  private fun getClBeaconChainHead(clApiUrl: String): SafeFuture<SignedBeaconBlock> =
    Http4kBeaconChainClient(clApiUrl)
      .getBlock("head")
      .thenApply { it.data }

  private fun getClBeaconChainHeadBlockNumber(clApiUrl: String): SafeFuture<ULong> =
    getClBeaconChainHead(clApiUrl).thenApply { it.message.slot.toULong() }

  private fun getClNodeChainHeads(nodes: List<NodeInfo<String>>): SafeFuture<List<NodeInfo<ULong>>> =
    nodes
      .map { node ->
        getClBeaconChainHeadBlockNumber(node.value)
          .thenApply { blockNumber -> node.map(blockNumber) }
      }.let { futures: List<SafeFuture<NodeInfo<ULong>>> ->
        SafeFuture.collectAll(futures.stream())
      }

  private fun assertNodesAreInSync(
    nodesHeads: List<NodeInfo<ULong>>,
    outOfSyncLeniency: Int,
  ) {
    val maxHead = nodesHeads.maxOf { it.value }
    val minHead = nodesHeads.minOf { it.value }

    if (maxHead - minHead > outOfSyncLeniency.toULong()) {
      val nodeWithMinHead = nodesHeads.minBy { it.value }
      log.error(
        "Nodes are out of sync: maxHead={}, minHead={}, diff={}, leniency={}, nodeWithMinHead={}",
        maxHead,
        minHead,
        maxHead - minHead,
        outOfSyncLeniency,
        nodeWithMinHead.label,
      )
      logNodesHeads(
        nodesHeads,
        logLevel = Level.INFO,
      )

      fail<Nothing>(
        "Nodes are out of sync: maxHead=$maxHead, minHead=$minHead nodeWithMinHead=${nodeWithMinHead.label}",
      )
    } else {
      logNodesHeads(
        nodesHeads,
        logLevel = Level.DEBUG,
      )
    }
  }

  private fun logNodesHeads(
    nodesHeads: List<NodeInfo<ULong>>,
    logLevel: Level = Level.INFO,
  ) {
    nodesHeads
      .sortedBy { it.value }
      .reversed()
      .forEach { (node, headBlock) ->
        log.log(logLevel, "node={} headBlock={}", node, headBlock)
      }
  }

  @BeforeEach
  fun beforeEach() {
    configureLoggers(
      rootLevel = Level.INFO,
      "maru.chaos.NodesSyncTest" to Level.DEBUG,
    )
  }

  @Test
  fun `el nodes should be in sync`() {
    val nodesUrls =
      getNodesUrlsFromFile(
        getPathTo("tmp/port-forward-besu-8545.txt"),
      ).filterNot { it.label.contains("bootnode") }
    await
      .ignoreExceptions()
      .pollInterval(5.seconds.toJavaDuration())
      .atMost(60.seconds.toJavaDuration())
      .untilAsserted {
        val nodesHeads = getElNodeChainHeads(nodesUrls).get()
        assertNodesAreInSync(nodesHeads, outOfSyncLeniency = 3)
      }
  }

  @Test
  fun `maru nodes should be in sync`() {
    val nodesUrls =
      getNodesUrlsFromFile(
        getPathTo("tmp/port-forward-maru-5060.txt"),
      )
    await
      .ignoreExceptions()
      .pollInterval(5.seconds.toJavaDuration())
      .atMost(60.seconds.toJavaDuration())
      .untilAsserted {
        val nodesHeads = getClNodeChainHeads(nodesUrls).get()
        assertNodesAreInSync(nodesHeads, outOfSyncLeniency = 3)
      }
  }

  @Test
  fun `sequencer should continue to produce blocks`() {
    fun isSequencer(node: NodeInfo<String>): Boolean =
      node.label.contains("sequencer") || node.label.contains("validator")

    val elSequencer =
      getNodesUrlsFromFile(
        getPathTo("tmp/port-forward-besu-8545.txt"),
      ).first(::isSequencer)
    val maruSequencer =
      getNodesUrlsFromFile(
        getPathTo("tmp/port-forward-maru-5060.txt"),
      ).first(::isSequencer)

    val highestMaruBlock = getClBeaconChainHead(maruSequencer.value).get()
    val highestExpectedElBlock =
      highestMaruBlock.message.body.executionPayload.blockNumber
        .toULong() + 1uL

    await
      .atMost(12.seconds.toJavaDuration()) // unlikely to have block time lower than 10 seconds
      .untilAsserted {
        assertThat(getClBeaconChainHeadBlockNumber(maruSequencer.value).get())
          .withFailMessage {
            "Maru sequencer stopped producing beacon blocks at height " +
              highestMaruBlock.message.body.executionPayload.blockNumber
          }.isGreaterThan(highestMaruBlock.message.slot.toULong())
      }

    await
      .atMost(3.seconds.toJavaDuration()) // 3s must be more that enough for Maru -> EL sync
      .untilAsserted {
        assertThat(getElNodeChainHead(elSequencer.value).get()).isGreaterThan(highestExpectedElBlock)
      }
  }

  @Test
  fun `multi-validator consensus should produce blocks from different proposers`() {
    val validatorNodes =
      getNodesUrlsFromFile(
        getPathTo("tmp/port-forward-maru-5060.txt"),
      ).filter { it.label.contains("validator") }

    if (validatorNodes.size < 2) {
      log.info("Skipping multi-validator test: found {} validator nodes", validatorNodes.size)
      return
    }

    val proposerIndices = mutableSetOf<String>()
    val client = Http4kBeaconChainClient(validatorNodes.first().value)

    await
      .pollInterval(2.seconds.toJavaDuration())
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        val block = client.getBlock("head").get().data
        proposerIndices.add(block.message.proposerIndex)
        log.info(
          "Block slot={} proposerIndex={} totalUniqueProposers={}",
          block.message.slot,
          block.message.proposerIndex,
          proposerIndices.size,
        )
        assertThat(proposerIndices.size)
          .withFailMessage {
            "Expected blocks from at least 2 different proposers, but only got proposerIndices=$proposerIndices"
          }.isGreaterThanOrEqualTo(2)
      }
  }
}
