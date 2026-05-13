/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import io.micrometer.core.instrument.Clock
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.hyperledger.besu.plugin.services.metrics.LabelledMetric
import java.util.concurrent.CompletableFuture
import net.consensys.linea.metrics.MetricsCategory as LineaMetricsCategory
import net.consensys.linea.metrics.Timer as LineaTimer
import org.hyperledger.besu.plugin.services.metrics.OperationTimer as BesuTimer

class TimerAdapter(
  val lineaTimer: LineaTimer,
  val clock: Clock,
) : BesuTimer {
  override fun startTimer(): BesuTimer.TimingContext {
    val startTime = clock.monotonicTime()
    val future = CompletableFuture<Unit>()
    lineaTimer.captureTime(future)
    return TimingContextAdapter(
      future = future,
      startTime = startTime,
      clock = clock,
    )
  }
}

class TimingContextAdapter(
  val future: CompletableFuture<Unit>,
  val startTime: Long,
  val clock: Clock,
) : BesuTimer.TimingContext {
  override fun stopTimer(): Double {
    val endTime = clock.monotonicTime()
    future.complete(null)
    return (endTime - startTime).toDouble()
  }
}

class LabelledTimerAdapter(
  val metricsFacade: MetricsFacade,
  val lineaMetricsCategory: LineaMetricsCategory,
  val labelNames: List<String>,
  val name: String,
  val description: String,
  private val clock: Clock = Clock.SYSTEM,
) : LabelledMetric<BesuTimer> {
  override fun labels(vararg labels: String): BesuTimer? {
    if (labels.size != labelNames.size) {
      throw IllegalArgumentException("Number of labels provided does not match the expected number of labels.")
    }
    val lineaTimer =
      metricsFacade.createTimer(
        category = lineaMetricsCategory,
        name = name,
        description = description,
        tags = labelNames.zip(labels).map { (name, value) -> Tag(name, value) },
      )
    return TimerAdapter(lineaTimer, clock)
  }
}
