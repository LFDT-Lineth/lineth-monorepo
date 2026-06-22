/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.hyperledger.besu.plugin.services.metrics.LabelledMetric
import net.consensys.linea.metrics.Histogram as LineaHistogram
import net.consensys.linea.metrics.MetricsCategory as LineaMetricsCategory
import org.hyperledger.besu.plugin.services.metrics.Histogram as BesuHistogram

class HistogramAdapter(
  val lineaHistogram: LineaHistogram,
) : BesuHistogram {
  override fun observe(amount: Double) {
    lineaHistogram.record(amount)
  }
}

class LabelledHistogramAdapter(
  val metricsFacade: MetricsFacade,
  val lineaMetricsCategory: LineaMetricsCategory,
  val name: String,
  val description: String,
  val labelNames: List<String>,
) : LabelledMetric<BesuHistogram> {
  override fun labels(vararg labels: String): HistogramAdapter {
    if (labels.size != labelNames.size) {
      throw IllegalArgumentException("Number of labels provided does not match the expected number of labels.")
    }
    val lineaHistogram =
      metricsFacade.createHistogram(
        category = lineaMetricsCategory,
        name = name,
        description = description,
        tags = labelNames.zip(labels).map { (name, value) -> Tag(name, value) },
      )
    return HistogramAdapter(lineaHistogram)
  }
}
