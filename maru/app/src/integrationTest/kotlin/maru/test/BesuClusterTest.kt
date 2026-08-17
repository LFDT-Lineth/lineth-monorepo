/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test

import linea.testing.besu.BesuFactory
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.test.cluster.BesuCluster
import maru.test.extensions.latestBlockNumber
import maru.test.extensions.nodeHeads
import maru.test.genesis.BesuGenesisFactory
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.fail
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.net.ConnectException
import java.util.Optional
import kotlin.random.Random
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class BesuClusterTest {
  private lateinit var genesis: String

  @BeforeEach
  fun beforeEach() {
    genesis = BesuGenesisFactory()
      .apply {
        setForkSchedule(
          ForksSchedule(
            chainId = Random.nextInt(1, Int.MAX_VALUE).toUInt(),
            forks = listOf(
              ForkSpec(
                timestampSeconds = 0UL,
                blockTimeSeconds = 1U,
                configuration = DifficultyAwareQbftConfig(
                  terminalTotalDifficulty = UInt.MAX_VALUE.toULong(),
                  postTtdConfig = QbftConsensusConfig(
                    validatorSet = setOf(Validator(Random.nextBytes(20))),
                    fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
                  ),
                ),
              ),
            ),
          ),
        )
      }.create()
  }

  fun createBesu(
    label: String,
    validator: Boolean = false,
    jsonRpcPort: Int? = null,
  ): BesuNode =
    BesuFactory.buildTestBesu(
      genesis,
      nodeName = label,
      validator = validator,
      jsonRpcPort = Optional.ofNullable(jsonRpcPort),
    )

  private lateinit var cluster: BesuCluster

  @AfterEach
  fun afterEach() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  @Test
  fun `should allow to add nodes to existing cluster and sync`() {
    cluster = BesuCluster()
      .apply {
        addNode(createBesu("besu-0"))
        addNode(createBesu("besu-1", validator = true))
        addNode(createBesu("besu-2"))
        start(false)
      }

    await
      .atMost(30.seconds.toJavaDuration())
      .untilAsserted {
        cluster.assertNodesAreSyncedUpTo(3UL)
      }

    val node2 = cluster.nodes["besu-2"]!!
    cluster.stopNode("besu-2")
    // it should throw if node was effectively stopped
    assertThrows<ConnectException> { node2.latestBlockNumber() }

    val newBesu = createBesu("besu-new-0")
    cluster.addNodeAndStart(newBesu, awaitPeerDiscovery = true)
    await
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(newBesu.latestBlockNumber()).isGreaterThanOrEqualTo(5UL)
      }
  }

  @Test
  fun `should allow to start nodes 1 by 1 and sync`() {
    cluster = BesuCluster()
      .apply {
        addNodeAndStart(createBesu("besu-0"))
        addNodeAndStart(createBesu("besu-1", validator = true))
        addNodeAndStart(createBesu("besu-2"))
      }

    await
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        cluster.assertNodesAreSyncedUpTo(3UL)
      }

    val newBesu = createBesu("besu-extra", validator = false)
    cluster.addNodeAndStart(newBesu)
    await
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(newBesu.latestBlockNumber()).isGreaterThanOrEqualTo(5UL)
      }
  }

  @Test
  fun `should remove,stop and add back nodes to the cluster`() {
    cluster = BesuCluster().apply {
      addNode(createBesu("sequencer", validator = true))
      addNode(createBesu("follower-1"))
      start(false)
    }

    await
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        cluster.assertNodesAreSyncedUpTo(3UL)
      }

    val sequencer = cluster.nodes["sequencer"]!!
    val lastMinedBlock = sequencer.latestBlockNumber()
    cluster.stopNode("sequencer")

    // it should throw if node was effectively stopped
    assertThrows<ConnectException> { sequencer.latestBlockNumber() }
    cluster.addNodeAndStart(sequencer)

    await
      .atMost(120.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(cluster.nodes["follower-1"]!!.latestBlockNumber()).isGreaterThanOrEqualTo(lastMinedBlock + 3UL)
      }
  }

  fun BesuCluster.assertNodesAreSyncedUpTo(targetBlockNumber: ULong) {
    val nodesHeadBlockNumbers = this.nodeHeads()
    val inSync = nodesHeadBlockNumbers.values.all { besuHead -> besuHead >= targetBlockNumber }

    if (!inSync) {
      fail<Unit>("Nodes did not sync to block $targetBlockNumber nodes heads: $nodesHeadBlockNumbers")
    }
  }
}
