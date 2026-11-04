/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import io.libp2p.etc.types.fromHex
import io.micrometer.core.instrument.DistributionSummary
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.distribution.HistogramSnapshot
import io.vertx.micrometer.backends.BackendRegistries
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastPeering
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test
import testutils.maru.awaitTillMaruHasPeers
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

/**
 * Benchmark measuring QBFT consensus latency with 4 validators on the local JVM — no containers.
 *
 * All phase latencies are recorded by [ConsensusMetrics] inside [MaruApp] into Micrometer histograms,
 * labeled by role (proposer vs non_proposer) and nodeid. These are the same metrics scraped by
 * Prometheus in K8S. This test collects no data itself — it only reads from the Micrometer registry.
 *
 * Topology: full mesh — all 6 bidirectional connections among 4 validators.
 * Gossipsub requires D≥4 peers to form a proper MESH and forward received messages.
 *
 * Run with:
 *   ./gradlew :app:integrationTest --tests "maru.app.QbftConsensus4ValidatorBenchmarkTest"
 */
@Disabled
class QbftConsensus4ValidatorBenchmarkTest {
  private val key0 = "080212201dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae".fromHex()
  private val key1 = "0802122100abb81ba53518eb0a206dfe80f2a973182e5d66c98cd31d00bf7471fcd5514157".fromHex()
  private val key2 = "080212202fec0750fe3edc7e8272d4814a36b632921fc5e835d20a2de874471e8ad9ad0b".fromHex()
  private val key3 = "080212207a19c01ce2246b94b48ed778d9bfb3b76eaabe8193c468d6751f4e4d1adf98a8".fromHex()

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
  fun `measure QBFT consensus latency on local JVM with 4 validators`() {
    // Start all 4 validators. The cluster builds the QBFT validator set from their node keys.
    // Phase latencies are recorded automatically by ConsensusMetrics
    // inside each MaruApp into Micrometer histograms labeled by role and nodeid.
    val chainId = 1337u
    val validatorKeys = listOf(key0, key1, key2, key3)
    cluster = MaruCluster(chainId = chainId)
    validatorKeys.forEachIndexed { index, key ->
      cluster.addNode("validator-$index", addBesu = true) { nodeBuilder ->
        nodeBuilder
          .withRole(NodeRole.Sequencer)
          // otherwise the 4 Besu nodes would share the default signer key, hence the same enode
          .besuValidator(false)
          .withNodeKey(key)
          .withFastQbft()
          .withFastPeering()
          .withSyncingConfig(TestSyncingConfigs.default)
          // Wire full mesh: 6 bidirectional connections for 4 nodes (n*(n-1)/2 = 6).
          .staticPeers((0 until index).map { "validator-$it" })
          .maruConfig {
            it.copy(
              allowEmptyBlocks = true,
              observability = it.observability.copy(prometheusMetricsEnabled = true, jvmMetricsEnabled = true),
            )
          }
      }
    }
    cluster.start()
    val app0 = cluster.maruNode("validator-0")
    val app1 = cluster.maruNode("validator-1")
    val app2 = cluster.maruNode("validator-2")
    val app3 = cluster.maruNode("validator-3")

    app0.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    app1.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    app2.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    app3.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    log.info("All 4 validators peered in full mesh — starting measurement")

    val blocksToMeasure = 100
    val latch = CountDownLatch(blocksToMeasure)

    app0.onBlockCommitted = { sealedBlock ->
      if (sealedBlock.beaconBlock.beaconBlockHeader.number > 0UL) {
        latch.countDown()
      }
    }

    assertThat(latch.await(blocksToMeasure * 3L, TimeUnit.SECONDS))
      .withFailMessage("Timed out waiting for $blocksToMeasure blocks — check validator logs")
      .isTrue()

    // Map nodeIds to human-readable names for the output.
    val nodeIdToName =
      mapOf(
        app0.p2pNetwork.nodeId to "validator-0",
        app1.p2pNetwork.nodeId to "validator-1",
        app2.p2pNetwork.nodeId to "validator-2",
        app3.p2pNetwork.nodeId to "validator-3",
      )

    val registry = BackendRegistries.getDefaultNow()
    if (registry != null) {
      printMetrics(registry, blocksToMeasure, nodeIdToName)
    } else {
      log.warn("No Micrometer registry available — metrics not recorded")
    }
  }

  private fun percentileFromBuckets(
    snapshot: HistogramSnapshot,
    percentile: Double,
  ): Double {
    val buckets = snapshot.histogramCounts()
    if (buckets.isEmpty()) return 0.0
    val total = snapshot.count().toDouble()
    if (total == 0.0) return 0.0
    val target = percentile * total
    for (bucket in buckets.sortedBy { it.bucket() }) {
      if (bucket.bucket().isInfinite()) continue
      if (bucket.count() >= target) return bucket.bucket()
    }
    return snapshot.max()
  }

  private fun printMetrics(
    registry: MeterRegistry,
    blocksToMeasure: Int,
    nodeIdToName: Map<String, String>,
  ) {
    log.info("==========================================================")
    log.info("  QBFT Consensus Latency — local JVM, real libp2p, 4 validators, 1 tx/block")
    log.info("  Target blocks: {}", blocksToMeasure)
    log.info("")

    // Print histograms grouped by validator, then by role within each validator.
    fun printHistogram(
      nameSuffix: String,
      label: String,
      description: String,
    ) {
      val meters =
        registry.meters.filter { meter ->
          meter.id.name.endsWith(nameSuffix) && meter is DistributionSummary
        }
      if (meters.isEmpty()) {
        log.info("  {}: not found", label)
        return
      }

      // Group by nodeid → role
      for (meter in meters.sortedWith(
        compareBy({ nodeIdToName[it.id.getTag("nodeid")!!] ?: it.id.getTag("nodeid") ?: "" }, {
          it.id.getTag("role")
            ?: ""
        }),
      )) {
        val summary = meter as DistributionSummary
        val snapshot = summary.takeSnapshot()
        if (snapshot.count() == 0L) continue
        val nodeLabel = nodeIdToName[summary.id.getTag("nodeid")!!] ?: summary.id.getTag("nodeid") ?: "?"
        val role = summary.id.getTag("role") ?: "all"

        val p50 = percentileFromBuckets(snapshot, 0.5)
        val p95 = percentileFromBuckets(snapshot, 0.95)
        val p99 = percentileFromBuckets(snapshot, 0.99)

        log.info(
          "  {} [{}] [{}] (n={}): mean={} p50={} p95={} p99={} max={}",
          label,
          nodeLabel,
          role,
          snapshot.count(),
          "%.1fms".format(snapshot.mean()),
          "%.1fms".format(p50),
          "%.1fms".format(p95),
          "%.1fms".format(p99),
          "%.1fms".format(snapshot.max()),
        )
      }
      log.info("    ^ {}", description)
    }

    fun printCounter(
      nameSuffix: String,
      label: String,
    ) {
      val meters =
        registry.meters.filter { meter ->
          meter.id.name.endsWith(nameSuffix) && meter is io.micrometer.core.instrument.Counter
        }
      for (meter in meters.sortedWith(
        compareBy({ nodeIdToName[it.id.getTag("nodeid")!!] ?: it.id.getTag("nodeid") ?: "" }, {
          it.id.getTag("role")
            ?: ""
        }),
      )) {
        val counter = meter as io.micrometer.core.instrument.Counter
        if (counter.count() == 0.0) continue
        val nodeLabel = nodeIdToName[counter.id.getTag("nodeid")!!] ?: counter.id.getTag("nodeid") ?: "?"
        val role = counter.id.getTag("role") ?: "all"
        log.info("  {} [{}] [{}] = {}", label, nodeLabel, role, counter.count().toLong())
      }
    }

    log.info("  ── Total consensus latency ──")
    printHistogram("block.latency", "block.latency", "timer-fire → block committed")
    log.info("")

    log.info("  ── Phase breakdown ──")
    printHistogram("phase.proposal", "phase.proposal", "timer → PROPOSAL received (non-proposer only)")
    printHistogram("phase.prepare.first", "phase.prepare.first", "start → first PREPARE received")
    printHistogram(
      "phase.prepare.spread",
      "phase.prepare.spread",
      "first → last PREPARE (parallel validation spread)",
    )
    printHistogram("phase.commit.first", "phase.commit.first", "last PREPARE → first COMMIT")
    printHistogram("phase.commit.spread", "phase.commit.spread", "first → last COMMIT (gossip jitter)")
    printHistogram(
      "phase.import",
      "phase.import",
      "last COMMIT → block committed (queue wait + seal verify + state transition + DB write + setHead)",
    )
    log.info("")

    log.info("  ── Counters ──")
    printCounter("blocks.committed", "blocks.committed")
  }
}
