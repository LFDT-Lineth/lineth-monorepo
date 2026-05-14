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
import net.consensys.linea.testing.filesystem.getPathTo
import org.apache.logging.log4j.LogManager
import org.http4k.client.OkHttp
import org.http4k.core.Method
import org.http4k.core.Request
import org.junit.jupiter.api.Test

/**
 * Collects QBFT consensus metrics from all maru pods via their Prometheus /metrics endpoint
 * and prints aggregated stats (avg, p95, max) per role, mirroring the format of
 * [maru.app.QbftConsensus4ValidatorBenchmarkTest].
 *
 * Requires the maru metrics port-forward to be active:
 *   make port-forward-component component=maru port=9545
 * which writes per-pod URLs to: chaos-testing/tmp/port-forward-maru-9545.txt
 *
 * Run standalone:
 *   ./gradlew :chaos-testing:health-checker:acceptanceTest \
 *       --tests "chaos.ConsensusMetricsBenchmarkTest"
 *
 * Metrics are published by [maru.app.ConsensusMetrics] as Micrometer DistributionSummary.
 * MicrometerMetricsFacade.metricHandle() builds the name as:
 *   "$prefix.$category.$name" = "maru.consensus.block.latency" → "maru_consensus_block_latency"
 * Prometheus exports: _count, _sum, _max, and _bucket{le=...} lines for p95 computation.
 */
class ConsensusMetricsBenchmarkTest {
  private val log = LogManager.getLogger(this.javaClass)
  private val httpClient = OkHttp()

  // Metric definitions.

  private data class MetricDef(
    val promName: String,
    val description: String,
  )

  private val metricsToCollect =
    listOf(
      MetricDef("maru_consensus_block_latency", "timer-fire → block committed"),
      MetricDef("maru_consensus_phase_proposal", "timer-fire → PROPOSAL received (non-proposer only)"),
      MetricDef("maru_consensus_phase_prepare_first", "start → first PREPARE received"),
      MetricDef("maru_consensus_phase_prepare_spread", "first → last PREPARE (parallel validation spread)"),
      MetricDef("maru_consensus_phase_commit_first", "last PREPARE → first COMMIT"),
      MetricDef("maru_consensus_phase_commit_spread", "first → last COMMIT (gossip jitter)"),
      MetricDef(
        "maru_consensus_phase_import",
        "last COMMIT → block committed (queue wait + seal verify + state transition + DB write + setHead)",
      ),
    )

  // Histogram data model.

  private data class HistogramData(
    val count: Long,
    val sum: Double,
    val max: Double,
    /** Exact quantile values published by Micrometer (quantile label → value). */
    val quantiles: Map<Double, Double>,
  ) {
    val mean: Double
      get() = if (count > 0) sum / count else 0.0

    /** Returns the exact [p]-th percentile from pre-computed quantile values. */
    fun percentile(p: Double): Double = quantiles[p] ?: 0.0
  }

  /**
   * Parses Prometheus text exposition format into a flat map of (metricName, labelMap) → value.
   * Skips comment and blank lines.
   */
  private fun parsePrometheusText(text: String): Map<Pair<String, Map<String, String>>, Double> {
    val result = mutableMapOf<Pair<String, Map<String, String>>, Double>()
    val labelRegex = Regex("""(\w+)="([^"]*)"""")

    for (line in text.lines()) {
      if (line.startsWith('#') || line.isBlank()) continue

      val braceStart = line.indexOf('{')
      val braceEnd = line.lastIndexOf('}')

      val metricName: String
      val labels: Map<String, String>
      val valueStr: String

      if (braceStart >= 0 && braceEnd > braceStart) {
        metricName = line.substring(0, braceStart).trim()
        labels = labelRegex
          .findAll(line.substring(braceStart + 1, braceEnd))
          .associate { it.groupValues[1] to it.groupValues[2] }
        valueStr = line.substring(braceEnd + 1).trim()
      } else {
        val spaceIdx = line.lastIndexOf(' ')
        if (spaceIdx < 0) continue
        metricName = line.substring(0, spaceIdx).trim()
        labels = emptyMap()
        valueStr = line.substring(spaceIdx).trim()
      }

      val value = valueStr.toDoubleOrNull() ?: continue
      result[metricName to labels] = value
    }
    return result
  }

  /**
   * Extracts a [HistogramData] for the given [metricName] and [role] from a parsed Prometheus map.
   * Returns null if no count sample is found for this metric+role combination.
   */
  private fun extractHistogram(
    samples: Map<Pair<String, Map<String, String>>, Double>,
    metricName: String,
    role: String,
  ): HistogramData? {
    fun matchesRole(labels: Map<String, String>): Boolean = labels["role"] == role

    val count =
      samples.entries
        .firstOrNull { (k, _) -> k.first == "${metricName}_count" && matchesRole(k.second) }
        ?.value
        ?.toLong()
        ?: return null

    val sum =
      samples.entries
        .firstOrNull { (k, _) -> k.first == "${metricName}_sum" && matchesRole(k.second) }
        ?.value ?: 0.0

    val max =
      samples.entries
        .firstOrNull { (k, _) -> k.first == "${metricName}_max" && matchesRole(k.second) }
        ?.value ?: 0.0

    val quantiles =
      samples.entries
        .filter { (k, _) -> k.first == metricName && matchesRole(k.second) && k.second.containsKey("quantile") }
        .associate { (k, v) ->
          val quantile = k.second["quantile"]?.toDoubleOrNull() ?: 0.0
          quantile to v
        }

    return HistogramData(count = count, sum = sum, max = max, quantiles = quantiles)
  }

  // Cross-pod aggregation.

  /**
   * Merges per-pod [HistogramData] into a single aggregate.
   * Counts/sums are summed; max is the global max;
   * histogram buckets are merged by summing cumulative counts at each le boundary.
   */
  private fun mergeHistograms(data: List<HistogramData>): HistogramData? {
    if (data.isEmpty()) return null
    val totalCount = data.sumOf { it.count }
    val totalSum = data.sumOf { it.sum }
    val totalMax = data.maxOf { it.max }
    val allQuantileKeys = data.flatMap { it.quantiles.keys }.toSortedSet()
    val mergedQuantiles =
      allQuantileKeys.associateWith { q ->
        data
          .filter { it.quantiles.containsKey(q) && it.count > 0 }
          .let { pods ->
            if (pods.isEmpty()) {
              0.0
            } else {
              pods.sumOf { it.quantiles[q]!! * it.count } / pods.sumOf { it.count }
            }
          }
      }
    return HistogramData(count = totalCount, sum = totalSum, max = totalMax, quantiles = mergedQuantiles)
  }

  // Test entry point.

  @Test
  fun `collect and print QBFT consensus metrics from all chaos pods`() {
    val experimentLatency = System.getProperty("experiment.latency")

    val metricsNodes =
      getNodesUrlsFromFile(
        getPathTo("tmp/port-forward-maru-9545.txt"),
      )

    log.info("Fetching /metrics from {} pods: {}", metricsNodes.size, metricsNodes.map { it.label })

    val podSamples: Map<String, Map<Pair<String, Map<String, String>>, Double>> =
      metricsNodes.associate { nodeInfo ->
        val url = "${nodeInfo.value}/metrics"
        val response =
          runCatching { httpClient(Request(Method.GET, url)) }
            .getOrElse { e ->
              log.error("Failed to fetch metrics from {} ({}): {}", nodeInfo.label, url, e.message)
              throw e
            }
        if (!response.status.successful) {
          log.error("Non-2xx from {} ({}): status={}", nodeInfo.label, url, response.status)
        }
        val samples = parsePrometheusText(response.bodyString())
        val consensusNames =
          samples.keys
            .map { it.first }
            .filter { it.startsWith("maru_consensus") }
            .toSortedSet()
        if (consensusNames.isEmpty()) {
          log.warn(
            "No maru_consensus_* metrics found for pod {}. First 20 available: {}",
            nodeInfo.label,
            samples.keys
              .map { it.first }
              .toSortedSet()
              .take(20),
          )
        } else {
          log.info("Pod {}: found consensus metrics: {}", nodeInfo.label, consensusNames)
        }
        nodeInfo.label to samples
      }

    printReport(podSamples, experimentLatency)
  }

  // Report printer.

  private fun printReport(
    podSamples: Map<String, Map<Pair<String, Map<String, String>>, Double>>,
    experimentLatency: String? = null,
  ) {
    val latencyLabel = experimentLatency?.let { " — injected latency: $it" } ?: ""
    log.info("==========================================================")
    log.info("  QBFT Consensus Latency — chaos testing K8s, {} validators{}", podSamples.size, latencyLabel)
    log.info("  Pods: {}", podSamples.keys)
    log.info("")

    val (totalMetrics, phaseMetrics) = metricsToCollect.partition { it.promName == "maru_consensus_block_latency" }

    log.info("  ── Total consensus latency ──")
    for (metric in totalMetrics) {
      printAggregatedMetric(podSamples, metric)
    }
    log.info("")

    log.info("  ── Phase breakdown ──")
    for (metric in phaseMetrics) {
      printAggregatedMetric(podSamples, metric)
    }
    log.info("==========================================================")
  }

  private fun printAggregatedMetric(
    podSamples: Map<String, Map<Pair<String, Map<String, String>>, Double>>,
    metric: MetricDef,
  ) {
    for (role in listOf("proposer", "non_proposer")) {
      val perPodData =
        podSamples
          .mapNotNull { (_, samples) ->
            extractHistogram(samples, metric.promName, role)
          }.filter { it.count > 0 }

      if (perPodData.isEmpty()) continue

      val merged = mergeHistograms(perPodData) ?: continue

      val quantileStr =
        merged.quantiles.entries
          .sortedBy { it.key }
          .joinToString(" ") { (q, v) -> "p${(q * 100).toInt()}=${"%.1fms".format(v)}" }
      log.info(
        "  {} [{}] (n={}, pods={}): mean={} {} max={}",
        metric.promName,
        role,
        merged.count,
        perPodData.size,
        "%.1fms".format(merged.mean),
        quantileStr,
        "%.1fms".format(merged.max),
      )
    }
    log.info("    ^ {}", metric.description)
  }
}
