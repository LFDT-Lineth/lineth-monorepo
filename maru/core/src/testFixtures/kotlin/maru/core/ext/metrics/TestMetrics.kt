/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core.ext.metrics

import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import maru.metrics.BesuMetricsSystemAdapter
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.hyperledger.besu.plugin.services.MetricsSystem

object TestMetrics {
  private val TestMeterRegistry: MeterRegistry = SimpleMeterRegistry()
  private val vertx = Vertx.vertx()
  val TestMetricsFacade: MetricsFacade =
    MicrometerMetricsFacade(
      registry = TestMeterRegistry,
      metricsPrefix = "maru.test",
    )
  val TestMetricsSystemAdapter: MetricsSystem =
    BesuMetricsSystemAdapter(
      metricsFacade = TestMetricsFacade,
      vertx = vertx,
    )
}
