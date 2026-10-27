/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test

import linea.kotlin.toULong
import linea.testing.besu.ethGetBlockByNumber
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.assertNodesAreSyncedUpTo
import maru.test.extensions.headBeaconBlockNumber
import maru.test.extensions.latestBlockNumber
import org.apache.logging.log4j.Level
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Order
import org.junit.jupiter.api.Test
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.times
import kotlin.time.toJavaDuration

class MaruClusterTest {
  private lateinit var cluster: MaruCluster

  @BeforeEach
  fun beforeEach() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf(
        "maru" to Level.INFO,
        "maru.clients" to Level.DEBUG,
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
  fun `should allow to retrieve nodes by label`() {
    cluster = MaruCluster()
      .addNode(NodeRole.Follower)
      .addNode(NodeRole.Sequencer, withBesuEl = true)
      .addNode("follower-special")
      .start()

    assertThat(cluster.node("follower").nodeRole).isEqualTo(NodeRole.Follower)
    assertThat(cluster.node("follower-special").nodeRole).isEqualTo(NodeRole.Follower)
    assertThat(cluster.node("sequencer").nodeRole).isEqualTo(NodeRole.Sequencer)
  }

  @Test
  @Order(2)
  fun `should create network starting at prague`() {
    cluster = MaruCluster(
      chainForks = mapOf(
        // Genesis at Prague
        Instant.fromEpochSeconds(0L) to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Prague,
          ),
      ),
    ).addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
      nodeBuilder.withLabel("sequencer")
    }.start()

    await()
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        cluster.assertNodesAreSyncedUpTo(targetBlockNumber = 5UL)
      }
  }

  @Test
  @Order(3)
  fun `should create network starting with all forks and switch post ttd`() {
    val now = Clock.System.now()
    val terminalTotalDifficulty = 20UL
    // QBFT assigns every block difficulty 1 (genesis difficulty is 0x1), so cumulative totalDifficulty reaches
    // terminalTotalDifficulty exactly at block (terminalTotalDifficulty - 1), the terminal block.
    val terminalBlockNumber = terminalTotalDifficulty - 1UL
    // Timestamp-based forks start well after the terminal block is reached, so that the first PoS block can only
    // have been produced through the DifficultyAwareQbft -> post-TTD QBFT transition, not a timestamp fork switch.
    val firstTimestampForkTime = now.plus(30.seconds)
    val forkTimeGap = 10.seconds
    cluster = MaruCluster(
      terminalTotalDifficulty = terminalTotalDifficulty,
      chainForks = mapOf(
        Instant.fromEpochSeconds(0) to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Paris,
          ),
        firstTimestampForkTime to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Shanghai,
          ),
        firstTimestampForkTime.plus(forkTimeGap) to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Cancun,
          ),
        firstTimestampForkTime.plus(2 * forkTimeGap) to
          ChainFork(
            ClFork.QBFT_PHASE0,
            ElFork.Prague,
          ),
      ),
    ).addNode("sequencer", addBesu = true)
      .start()

    val sequencerBesu = cluster.besuNode("sequencer")
    val firstPosBlockNumber = terminalBlockNumber + 1UL
    await()
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(sequencerBesu.latestBlockNumber())
          .withFailMessage { "Sequencer did not produce first PoS block=$firstPosBlockNumber" }
          .isGreaterThanOrEqualTo(firstPosBlockNumber)
      }

    // terminal block is the last block produced by Besu's QBFT (pre-merge, difficulty 1)
    val terminalBlock = sequencerBesu.ethGetBlockByNumber(terminalBlockNumber)
    assertThat(terminalBlock.difficulty.toLong()).isEqualTo(1L)

    // the next block is the first PoS block (difficulty 0), driven by Maru via Engine API
    val firstPosBlock = sequencerBesu.ethGetBlockByNumber(firstPosBlockNumber)
    assertThat(firstPosBlock.difficulty.toLong()).isEqualTo(0L)
    assertThat(firstPosBlock.timestamp.toULong())
      .withFailMessage {
        "First PoS block timestamp=${firstPosBlock.timestamp} is not before the first timestamp fork " +
          "at ${firstTimestampForkTime.epochSeconds}, so it was not produced by the post-TTD transition"
      }.isLessThan(firstTimestampForkTime.epochSeconds.toULong())
  }

  @Test
  @Order(4)
  fun `should instantiate multiple nodes in the cluster with static peering and sync`() {
    cluster = MaruCluster()
      .addNode("sequencer", addBesu = true)
      .addNode("follower-internal-0") { nodeBuilder ->
        nodeBuilder
          .staticPeers(listOf("sequencer"))
      }.addNode("follower-internal-1") { nodeBuilder ->
        nodeBuilder
          .staticPeers(listOf("sequencer"))
      }.start()

    await()
      .apply { }
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        cluster.assertNodesAreSyncedUpTo(targetBlockNumber = 3UL)
      }
  }

  @Test
  @Order(5)
  fun `should instantiate multiple nodes in the cluster with discovery`() {
    cluster = MaruCluster()
      .addNode("bootnode-1")
      .addNode("sequencer", addBesu = true)
      .addNode("follower-internal-0")
      .start()
    val followers = cluster.nodes(NodeRole.Follower)

    await()
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        followers
          .forEach { assertThat(it.maru.p2pNetwork.peerCount).isGreaterThanOrEqualTo(cluster.nodeCount() - 1) }
      }

    await()
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        cluster.assertNodesAreSyncedUpTo(targetBlockNumber = 5UL)
      }
  }

  @Test
  @Order(6)
  fun `should allow to add nodes after cluster is has started`() {
    cluster = MaruCluster()
      .addNode("bootnode-0")
      .addNode("sequencer", addBesu = true)
      .addNode("follower-internal-0")
      .start()

    await()
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(90.seconds.toJavaDuration())
      .untilAsserted { cluster.assertNodesAreSyncedUpTo(targetBlockNumber = 2UL) }

    // add new node and wait for sync
    cluster.addNode("follower-new-node")
    await()
      .pollInterval(1.seconds.toJavaDuration())
      // besu peering takes time sometimes, necessary for back sync
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        cluster.node("follower-new-node").let { newNode ->
          assertThat(newNode.maru.p2pNetwork.peerCount).isGreaterThanOrEqualTo(cluster.nodeCount() - 1)
          assertThat(newNode.maru.headBeaconBlockNumber()).isGreaterThanOrEqualTo(5UL)
        }
      }
  }

  @Test
  @Order(7)
  fun `should fail when nodes with duplicated labels are added`() {
    cluster = MaruCluster()
      .addNode("node-1")
      .addNode("node-2")

    try {
      cluster.addNode("node-1")
      throw AssertionError("Expected exception was not thrown")
    } catch (e: IllegalArgumentException) {
      assertThat(e.message).isEqualTo("Node labels must be unique: label=node-1 already exists in the cluster")
    }
  }
}
