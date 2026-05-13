/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import io.vertx.core.Vertx
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.hyperledger.besu.plugin.services.metrics.LabelledSuppliedMetric
import java.util.function.DoubleSupplier
import kotlin.time.DurationUnit
import net.consensys.linea.metrics.MetricsCategory as LineaMetricsCategory

class LabelledSuppliedCounterAdapter(
  val metricsFacade: MetricsFacade,
  val lineaMetricsCategory: LineaMetricsCategory,
  val name: String,
  val description: String,
  val labelNames: List<String>,
  val vertx: Vertx,
  val pollingConfig: SuppliedMetricPollingConfig = SuppliedMetricPollingConfig(),
) : LabelledSuppliedMetric {
  override fun labels(
    valueSupplier: DoubleSupplier,
    vararg labels: String,
  ) {
    if (labels.size != labelNames.size) {
      throw IllegalArgumentException("Number of labels provided does not match the expected number of labels.")
    }
    val lineaCounter =
      metricsFacade.createCounter(
        category = lineaMetricsCategory,
        name = name,
        description = description,
        tags = labelNames.zip(labels).map { (name, value) -> Tag(name, value) },
      )
    vertx.setPeriodic(
      pollingConfig.initialDelay.toLong(DurationUnit.MILLISECONDS),
      pollingConfig.pollingInterval.toLong(DurationUnit.MILLISECONDS),
      {
        lineaCounter.increment(valueSupplier.asDouble)
      },
    )
  }
}
