/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.core.SealedBeaconBlock
import maru.metrics.MaruMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.hyperledger.besu.consensus.qbft.core.messagedata.QbftV1
import java.util.concurrent.ConcurrentHashMap

/**
 * Micrometer-based consensus metrics that record QBFT phase latencies as histograms,
 * labeled by [role] (proposer vs non-proposer).
 *
 * Role detection: if a PROPOSAL was received via P2P for a given block, this node was a non-proposer;
 * otherwise it was the proposer (proposers create the PROPOSAL locally and never receive it back via gossipsub).
 *
 * Lifecycle: create once per [MaruApp] init (when `config.qbft != null`), then call the `record*` methods
 * from the observer callbacks. Stale per-block state is cleaned up automatically.
 */
class ConsensusMetrics(
  metricsFacade: MetricsFacade,
  /**
   * Returns the current chain head block number. Used to bound the [recordMessageReceived] window
   * so that crafted far-future sequenceNumber values cannot cause unbounded map growth.
   */
  private val currentHeightProvider: () -> Long,
) {
  companion object {
    private const val ROLE_PROPOSER = "proposer"
    private const val ROLE_NON_PROPOSER = "non_proposer"

    /**
     * Maximum number of blocks ahead of the current chain head that we will track.
     * Messages with sequenceNumber > currentHeight + MAX_FUTURE_BLOCKS are ignored.
     * QBFT only legitimately looks one round ahead, so 2 gives comfortable headroom.
     */
    private const val MAX_FUTURE_BLOCKS = 2L
  }

  // ── per-block timestamp state (monotonic ns from System.nanoTime()) ─────────

  private val timerFireTimes = ConcurrentHashMap<Long, Long>()
  private val proposalTimes = ConcurrentHashMap<Long, Long>()
  private val firstPrepareTimes = ConcurrentHashMap<Long, Long>()
  private val lastPrepareTimes = ConcurrentHashMap<Long, Long>()
  private val firstCommitTimes = ConcurrentHashMap<Long, Long>()
  private val lastCommitTimes = ConcurrentHashMap<Long, Long>()

  // Micrometer histograms (one per role).

  private fun histogram(
    metricsFacade: MetricsFacade,
    name: String,
    description: String,
    role: String,
  ) = metricsFacade.createHistogram(
    category = MaruMetricsCategory.CONSENSUS,
    name = name,
    description = description,
    tags = listOf(Tag("role", role)),
    publishPercentileHistogram = true,
    // NOTE: zkevm's createHistogram in :jvm-libs:linea:core:metrics does not yet expose
    // a `percentileBuckets` parameter (was added in newer maru-side metrics versions).
    // Dropping the customized buckets — the publishPercentileHistogram flag still emits
    // a percentile histogram with default buckets. Re-add `percentileBuckets` here once
    // the zkevm metrics interface gains the parameter.
  )

  // Total consensus latency: timer-fire to block committed (ms).
  private val blockLatencyProposer =
    histogram(metricsFacade, "block.latency", "Total consensus latency (ms)", ROLE_PROPOSER)
  private val blockLatencyNonProposer =
    histogram(metricsFacade, "block.latency", "Total consensus latency (ms)", ROLE_NON_PROPOSER)

  // Phase: timer → PROPOSAL received (non-proposer only).
  private val phaseProposal =
    histogram(metricsFacade, "phase.proposal", "Timer fire to PROPOSAL received (ms)", ROLE_NON_PROPOSER)

  // Phase: start → first PREPARE received.
  private val phaseFirstPrepareProposer =
    histogram(metricsFacade, "phase.prepare.first", "Start to first PREPARE (ms)", ROLE_PROPOSER)
  private val phaseFirstPrepareNonProposer =
    histogram(metricsFacade, "phase.prepare.first", "Start to first PREPARE (ms)", ROLE_NON_PROPOSER)

  // Phase: first → last PREPARE (spread).
  private val phasePrepareSpreadProposer =
    histogram(metricsFacade, "phase.prepare.spread", "First to last PREPARE (ms)", ROLE_PROPOSER)
  private val phasePrepareSpreadNonProposer =
    histogram(metricsFacade, "phase.prepare.spread", "First to last PREPARE (ms)", ROLE_NON_PROPOSER)

  // Phase: last PREPARE → first COMMIT.
  private val phaseFirstCommitProposer =
    histogram(metricsFacade, "phase.commit.first", "Last PREPARE to first COMMIT (ms)", ROLE_PROPOSER)
  private val phaseFirstCommitNonProposer =
    histogram(metricsFacade, "phase.commit.first", "Last PREPARE to first COMMIT (ms)", ROLE_NON_PROPOSER)

  // Phase: first → last COMMIT (spread).
  private val phaseCommitSpreadProposer =
    histogram(metricsFacade, "phase.commit.spread", "First to last COMMIT (ms)", ROLE_PROPOSER)
  private val phaseCommitSpreadNonProposer =
    histogram(metricsFacade, "phase.commit.spread", "First to last COMMIT (ms)", ROLE_NON_PROPOSER)

  // Phase: last COMMIT → block committed.
  private val phaseImportProposer =
    histogram(metricsFacade, "phase.import", "Last COMMIT to block committed (ms)", ROLE_PROPOSER)
  private val phaseImportNonProposer =
    histogram(metricsFacade, "phase.import", "Last COMMIT to block committed (ms)", ROLE_NON_PROPOSER)

  // Recording methods.

  /** Called when BLOCK_TIMER_EXPIRY fires on this validator. */
  fun recordTimerFire(blockNumber: Long) {
    timerFireTimes[blockNumber] = System.nanoTime()
    cleanupOldEntries(blockNumber)
  }

  /**
   * Called when a QBFT message arrives from the P2P network.
   * Tracks per-message-type timestamps for phase breakdown.
   */
  fun recordMessageReceived(
    msgCode: Int,
    sequenceNumber: Long,
  ) {
    if (sequenceNumber <= 0) return
    // Reject messages for blocks too far ahead of the current chain head to prevent
    // unbounded memory growth from crafted large sequenceNumber values.
    if (sequenceNumber > currentHeightProvider() + MAX_FUTURE_BLOCKS) return
    val now = System.nanoTime()
    when (msgCode) {
      QbftV1.PROPOSAL -> proposalTimes.putIfAbsent(sequenceNumber, now)
      QbftV1.PREPARE -> {
        firstPrepareTimes.putIfAbsent(sequenceNumber, now)
        lastPrepareTimes[sequenceNumber] = now
      }
      QbftV1.COMMIT -> {
        firstCommitTimes.putIfAbsent(sequenceNumber, now)
        lastCommitTimes[sequenceNumber] = now
      }
    }
  }

  /**
   * Called when a sealed beacon block is committed to the database.
   * Determines the role (proposer vs non-proposer) and records all phase durations
   * with the appropriate role tag.
   */
  fun recordBlockCommitted(sealedBlock: SealedBeaconBlock) {
    val commitTime = System.nanoTime()
    val blockNumber =
      sealedBlock.beaconBlock.beaconBlockHeader.number
        .toLong()
    if (blockNumber <= 0) return

    val timerFire = timerFireTimes[blockNumber] ?: return
    val totalLatencyMs = (commitTime - timerFire) / 1_000_000.0
    if (totalLatencyMs !in 0.0..5000.0) return

    // Determine role: if a PROPOSAL was received via P2P, this node was a non-proposer.
    val proposalTime = proposalTimes[blockNumber]
    val isProposer = proposalTime == null

    if (isProposer) {
      blockLatencyProposer.record(totalLatencyMs)
    } else {
      blockLatencyNonProposer.record(totalLatencyMs)
    }

    // Phase: timer → PROPOSAL (non-proposer only)
    if (!isProposer) {
      val duration = (proposalTime - timerFire) / 1_000_000.0
      if (duration >= 0.0) phaseProposal.record(duration)
    }

    // Phase: start → first PREPARE
    // "start" is either PROPOSAL received (non-proposer) or timer fire (proposer)
    val phaseStart = proposalTime ?: timerFire
    val firstPrepare = firstPrepareTimes[blockNumber]
    if (firstPrepare != null) {
      val duration = (firstPrepare - phaseStart) / 1_000_000.0
      if (duration >= 0.0) {
        (if (isProposer) phaseFirstPrepareProposer else phaseFirstPrepareNonProposer).record(duration)
      }
    }

    // Phase: first → last PREPARE
    val lastPrepare = lastPrepareTimes[blockNumber]
    if (firstPrepare != null && lastPrepare != null) {
      val duration = (lastPrepare - firstPrepare) / 1_000_000.0
      if (duration >= 0.0) {
        (if (isProposer) phasePrepareSpreadProposer else phasePrepareSpreadNonProposer).record(duration)
      }
    }

    // Phase: last PREPARE → first COMMIT
    val firstCommit = firstCommitTimes[blockNumber]
    if (lastPrepare != null && firstCommit != null) {
      val duration = (firstCommit - lastPrepare) / 1_000_000.0
      if (duration >= 0.0) {
        (if (isProposer) phaseFirstCommitProposer else phaseFirstCommitNonProposer).record(duration)
      }
    }

    // Phase: first → last COMMIT
    val lastCommit = lastCommitTimes[blockNumber]
    if (firstCommit != null && lastCommit != null) {
      val duration = (lastCommit - firstCommit) / 1_000_000.0
      if (duration >= 0.0) {
        (if (isProposer) phaseCommitSpreadProposer else phaseCommitSpreadNonProposer).record(duration)
      }
    }

    // Phase: last COMMIT → committed
    if (lastCommit != null) {
      val duration = (commitTime - lastCommit) / 1_000_000.0
      if (duration >= 0.0) {
        (if (isProposer) phaseImportProposer else phaseImportNonProposer).record(duration)
      }
    }
  }

  /**
   * Remove older entries to prevent memory leaks.
   */
  private fun cleanupOldEntries(currentBlockNumber: Long) {
    val threshold = currentBlockNumber - 1
    if (threshold <= 0) return
    listOf(
      timerFireTimes,
      proposalTimes,
      firstPrepareTimes,
      lastPrepareTimes,
      firstCommitTimes,
      lastCommitTimes,
    ).forEach { map ->
      map.keys.removeAll { it <= threshold }
    }
  }
}
