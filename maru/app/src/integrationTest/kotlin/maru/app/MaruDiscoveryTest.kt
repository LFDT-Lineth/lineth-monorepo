/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.withFastPeering
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.getBlockNumber
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

/**
 * Test suite for Maru peer discovery with multiple nodes.
 * Tests that multiple Maru nodes can discover each other using discovery protocol.
 */
class MaruDiscoveryTest {
  private lateinit var cluster: MaruCluster
  private val log = LogManager.getLogger(this.javaClass)

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
  fun `ten nodes discover each other via bootnode`() {
    testMultiNodeDiscovery(numberOfNodes = 10, expectedPeers = 9)
  }

  /**
   * Tests peer discovery with multiple Maru nodes.
   *
   * Setup:
   * - Creates N Besu nodes (EL) and N Maru nodes (CL)
   * - First Maru node is the sequencer and acts as the bootnode
   * - All other nodes use the bootnode's ENR for discovery
   *
   * @param numberOfNodes Total number of Maru nodes to create
   * @param expectedPeers Number of peers each node should discover
   */
  private fun testMultiNodeDiscovery(
    numberOfNodes: Int,
    expectedPeers: Int,
  ) {
    require(numberOfNodes >= 2) { "Need at least 2 nodes for discovery test" }

    log.info("Starting peer discovery test with $numberOfNodes nodes")

    // Start the bootnode (sequencer) alone first
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = true) }
        nodeBuilder.withFastPeering(banPeriod = 10.seconds, cooldownPeriod = 10.seconds)
      }.start()
    val bootnodeBesu = cluster.besuNode("sequencer")
    log.info("Bootnode ENR: ${cluster.maruNode("sequencer").p2pNetwork.enr}")

    // Wait for some blocks to be produced
    await
      .atMost(20.seconds.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(bootnodeBesu.getBlockNumber().toLong()).isGreaterThanOrEqualTo(5L)
      }
    log.info("Validator is producing blocks")

    // Add the follower nodes to the running cluster, discovering peers through the bootnode only
    for (i in 1 until numberOfNodes) {
      cluster.addNode("follower-$i", addBesu = true) { nodeBuilder ->
        nodeBuilder.bootnodes(listOf("sequencer"))
        nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = true) }
        nodeBuilder.withFastPeering(banPeriod = 10.seconds, cooldownPeriod = 10.seconds)
      }
    }
    log.info("All $numberOfNodes Maru nodes started")

    // Wait for all nodes to discover each other
    log.info("Waiting for all nodes to discover $expectedPeers peers")
    await
      .atMost(60.seconds.toJavaDuration())
      .pollInterval(2.seconds.toJavaDuration())
      .untilAsserted {
        cluster.nodes.forEach { node ->
          val peerCount = node.maru.peersConnected()
          log.info("Node ${node.label} has $peerCount peers (expected: $expectedPeers)")
          assertThat(peerCount).isGreaterThanOrEqualTo(expectedPeers.toUInt())
        }
      }
    log.info("All nodes have discovered their peers!")

    log.info("Verifying followers sync EL blocks")
    val validatorBlockHeight = bootnodeBesu.getBlockNumber()

    await
      .atMost(30.seconds.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .untilAsserted {
        cluster.nodes.forEach { node ->
          val besuBlockHeight = cluster.besuNode(node.label).getBlockNumber()
          log.info("Node ${node.label} EL block height: $besuBlockHeight (validator: $validatorBlockHeight)")
          assertThat(besuBlockHeight).isGreaterThanOrEqualTo(validatorBlockHeight)
        }
      }

    log.info("All followers have synced EL blocks successfully!")
  }
}
