/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import io.micrometer.core.instrument.ImmutableTag
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.data.Offset
import org.awaitility.kotlin.await
import org.awaitility.kotlin.untilAsserted
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.util.Optional
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration
import net.consensys.linea.metrics.MetricsCategory as LineaMetricsCategory
import org.hyperledger.besu.plugin.services.metrics.MetricCategory as BesuMetricsCategory

class BesuMetricsSystemAdapterTest {
  private lateinit var meterRegistry: MeterRegistry
  private lateinit var metricsFacade: MetricsFacade
  private lateinit var vertx: Vertx
  private lateinit var besuMetricsSystemAdapter: BesuMetricsSystemAdapter

  enum class TestLineaCategory : LineaMetricsCategory {
    TEST_CATEGORY,
  }

  enum class TestBesuMetricsCategory : BesuMetricsCategory {
    BESU_TEST_CATEGORY {
      override fun getName(): String? = "BESU_TEST_CATEGORY"

      override fun getApplicationPrefix(): Optional<String> = Optional.empty()
    },
  }

  @BeforeEach
  fun beforeEach() {
    vertx = Vertx.vertx()
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(
      meterRegistry,
      metricsPrefix = "linea.test",
      allMetricsCommonTags = listOf(Tag("version", "1.0.1")),
    )
    besuMetricsSystemAdapter = BesuMetricsSystemAdapter(
      vertx = vertx,
      metricsFacade = metricsFacade,
    )
  }

  @Test
  fun `creates counter with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val labelledMetric =
      besuMetricsSystemAdapter.createLabelledCounter(
        category = BesuMetricsCategoryAdapter.from(TestLineaCategory.TEST_CATEGORY),
        name = "some.metric",
        help = "This is a test counter",
        labelNames = expectedTags.map { it.key }.toList().toTypedArray(),
      )
    val counter = labelledMetric.labels(*expectedTags.map { it.value }.toList().toTypedArray())
    val createdCounter = meterRegistry.find("linea.test.test.category.some.metric").counter()
    assertThat(createdCounter!!.count()).isEqualTo(0.0)
    assertThat(createdCounter).isNotNull
    counter.inc(13)
    assertThat(createdCounter.count()).isEqualTo(13.0)

    counter.inc(2)
    assertThat(createdCounter.count()).isEqualTo(15.0)
    counter.inc()
    assertThat(createdCounter.count()).isEqualTo(16.0)
    assertThat(
      createdCounter.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
    assertThat(createdCounter.id.description).isEqualTo("This is a test counter")
  }

  @Test
  fun `with new besu category creates counter with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val labelledMetric =
      besuMetricsSystemAdapter.createLabelledCounter(
        category = TestBesuMetricsCategory.BESU_TEST_CATEGORY,
        name = "some.metric",
        help = "This is a test counter",
        labelNames = expectedTags.map { it.key }.toList().toTypedArray(),
      )
    val counter = labelledMetric.labels(*expectedTags.map { it.value }.toList().toTypedArray())
    val createdCounter = meterRegistry.find("linea.test.besu.test.category.some.metric").counter()
    assertThat(createdCounter!!.count()).isEqualTo(0.0)
    assertThat(createdCounter).isNotNull
    counter.inc(13)
    assertThat(createdCounter.count()).isEqualTo(13.0)

    counter.inc(2)
    assertThat(createdCounter.count()).isEqualTo(15.0)
    counter.inc()
    assertThat(createdCounter.count()).isEqualTo(16.0)
    assertThat(
      createdCounter.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
    assertThat(createdCounter.id.description).isEqualTo("This is a test counter")
  }

  @Test
  fun `creates gauge with specified parameters`() {
    var metricMeasureValue = 0L
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val labelledSuppliedGauge =
      besuMetricsSystemAdapter.createLabelledSuppliedGauge(
        category = BesuMetricsCategoryAdapter.from(TestLineaCategory.TEST_CATEGORY),
        name = "some.metric",
        help = "This is a test metric",
        labelNames = expectedTags.map { it.key }.toList().toTypedArray(),
      )
    labelledSuppliedGauge.labels(
      { metricMeasureValue.toDouble() },
      *expectedTags.map { it.value }.toList().toTypedArray(),
    )
    metricMeasureValue = 13L
    val createdGauge = meterRegistry.find("linea.test.test.category.some.metric").gauge()
    assertThat(createdGauge).isNotNull
    assertThat(createdGauge!!.value()).isEqualTo(13.0)
    metricMeasureValue = 2L
    assertThat(createdGauge.value()).isEqualTo(2.0)
    assertThat(
      createdGauge.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
    assertThat(createdGauge.id.description).isEqualTo("This is a test metric")
  }

  @Test
  fun `creates timer with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val labelledTimerAdapter =
      besuMetricsSystemAdapter.createLabelledTimer(
        category = BesuMetricsCategoryAdapter.from(TestLineaCategory.TEST_CATEGORY),
        name = "some.timer.metric",
        help = "This is a test metric",
        labelNames = expectedTags.map { it.key }.toList().toTypedArray(),
      )
    val timer = labelledTimerAdapter.labels(*expectedTags.map { it.value }.toList().toTypedArray())
    val timerContext = timer.startTimer()
    Thread.sleep(200L)
    val elapsedTime = timerContext.stopTimer()

    val createdTimer = meterRegistry.find("linea.test.test.category.some.timer.metric").timer()
    assertThat(createdTimer).isNotNull
    assertThat(createdTimer!!.id.description).isEqualTo("This is a test metric")
    assertThat(
      createdTimer.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )

    assertThat(createdTimer.max(TimeUnit.MILLISECONDS)).isGreaterThanOrEqualTo(200.0)
    assertThat(elapsedTime).isGreaterThanOrEqualTo(200.0)
  }

  @Test
  fun `creates histogram with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val labelledHistogramAdapter =
      besuMetricsSystemAdapter.createLabelledHistogram(
        category = BesuMetricsCategoryAdapter.from(TestLineaCategory.TEST_CATEGORY),
        name = "some.metric",
        help = "This is a test metric",
        labelNames = expectedTags.map { it.key }.toList().toTypedArray(),
        buckets = doubleArrayOf(),
      )
    val histogram = labelledHistogramAdapter.labels(*expectedTags.map { it.value }.toList().toTypedArray())

    val createdHistogram = meterRegistry.find("linea.test.test.category.some.metric").summary()
    assertThat(createdHistogram).isNotNull
    assertThat(createdHistogram!!.id.description).isEqualTo("This is a test metric")
    assertThat(
      createdHistogram.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
    assertThat(createdHistogram.count()).isEqualTo(0L)

    histogram.observe(10.0)
    assertThat(createdHistogram.count()).isEqualTo(1L)
    assertThat(createdHistogram.totalAmount()).isEqualTo(10.0)
    assertThat(createdHistogram.mean()).isEqualTo(10.0)
    assertThat(createdHistogram.max()).isEqualTo(10.0)
    histogram.observe(5.0)
    assertThat(createdHistogram.count()).isEqualTo(2L)
    assertThat(createdHistogram.totalAmount()).isEqualTo(15.0)
    assertThat(createdHistogram.mean()).isEqualTo(7.5)
    assertThat(createdHistogram.max()).isEqualTo(10.0)
    histogram.observe(100.0)
    assertThat(createdHistogram.count()).isEqualTo(3L)
    assertThat(createdHistogram.totalAmount()).isEqualTo(115.0)
    assertThat(createdHistogram.mean()).isCloseTo(38.333, Offset.offset(0.1))
    assertThat(createdHistogram.max()).isEqualTo(100.0)
  }

  @Test
  fun `creates counter with value supplier`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val labelledMetric =
      besuMetricsSystemAdapter.createLabelledSuppliedCounter(
        category = BesuMetricsCategoryAdapter.from(TestLineaCategory.TEST_CATEGORY),
        name = "some.metric",
        help = "This is a test counter",
        labelNames = expectedTags.map { it.key }.toList().toTypedArray(),
      )
    val counter =
      labelledMetric.labels(
        { 2.0 },
        *expectedTags.map { it.value }.toList().toTypedArray(),
      )
    val createdCounter = meterRegistry.find("linea.test.test.category.some.metric").counter()
    assertThat(createdCounter).isNotNull

    await
      .pollInterval(1.seconds.toJavaDuration())
      .timeout(10.seconds.toJavaDuration())
      .untilAsserted { assertThat(createdCounter!!.count()).isGreaterThanOrEqualTo(8.0) }

    assertThat(
      createdCounter!!.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
    assertThat(createdCounter.id.description).isEqualTo("This is a test counter")
  }
}
