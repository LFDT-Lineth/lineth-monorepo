package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.ImmutableTag
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.data.Offset
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.util.concurrent.TimeUnit

class MicrometerMetricsFacadeTest {
  private lateinit var meterRegistry: MeterRegistry
  private lateinit var metricsFacade: MetricsFacade

  @BeforeEach
  fun beforeEach() {
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(meterRegistry, "linea.test")
  }

  @Test
  fun `createGauge creates gauge with specified parameters`() {
    var metricMeasureValue = 0L
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "some.metric",
      description = "This is a test metric",
      measurementSupplier = { metricMeasureValue },
      tags = expectedTags
    )
    metricMeasureValue = 13L
    val createdGauge = meterRegistry.find("linea.test.batch.some.metric").gauge()
    assertThat(createdGauge).isNotNull
    assertThat(createdGauge!!.value()).isEqualTo(13.0)
    metricMeasureValue = 2L
    assertThat(createdGauge.value()).isEqualTo(2.0)
    assertThat(createdGauge.id.tags).isEqualTo(listOf(ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")))
    assertThat(createdGauge.id.description).isEqualTo("This is a test metric")
  }

  @Test
  fun `createCounter creates counter with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val counter = metricsFacade.createCounter(
      category = LineaMetricsCategory.BATCH,
      name = "some.metric",
      description = "This is a test metric",
      tags = expectedTags
    )
    val createdCounter = meterRegistry.find("linea.test.batch.some.metric").counter()
    assertThat(createdCounter!!.count()).isEqualTo(0.0)
    assertThat(createdCounter).isNotNull
    counter.increment(13.0)
    assertThat(createdCounter.count()).isEqualTo(13.0)

    counter.increment(2.0)
    assertThat(createdCounter.count()).isEqualTo(15.0)
    counter.increment()
    assertThat(createdCounter.count()).isEqualTo(16.0)
    counter.increment(0.5)
    assertThat(createdCounter.count()).isEqualTo(16.5)
    assertThat(createdCounter.id.tags).isEqualTo(listOf(ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")))
    assertThat(createdCounter.id.description).isEqualTo("This is a test metric")
  }

  @Test
  fun `createHistogram creates histogram with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val histogram = metricsFacade.createHistogram(
      category = LineaMetricsCategory.BATCH,
      name = "some.metric",
      description = "This is a test metric",
      tags = expectedTags,
      baseUnit = "seconds"
    )

    val createdHistogram = meterRegistry.find("linea.test.batch.some.metric").summary()
    assertThat(createdHistogram).isNotNull
    assertThat(createdHistogram!!.id.description).isEqualTo("This is a test metric")
    assertThat(createdHistogram.id.tags).isEqualTo(
      listOf(ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2"))
    )
    assertThat(createdHistogram.id.baseUnit).isEqualTo("seconds")
    assertThat(createdHistogram.count()).isEqualTo(0L)

    histogram.record(10.0)
    assertThat(createdHistogram.count()).isEqualTo(1L)
    assertThat(createdHistogram.totalAmount()).isEqualTo(10.0)
    assertThat(createdHistogram.mean()).isEqualTo(10.0)
    assertThat(createdHistogram.max()).isEqualTo(10.0)
    histogram.record(5.0)
    assertThat(createdHistogram.count()).isEqualTo(2L)
    assertThat(createdHistogram.totalAmount()).isEqualTo(15.0)
    assertThat(createdHistogram.mean()).isEqualTo(7.5)
    assertThat(createdHistogram.max()).isEqualTo(10.0)
    histogram.record(100.0)
    assertThat(createdHistogram.count()).isEqualTo(3L)
    assertThat(createdHistogram.totalAmount()).isEqualTo(115.0)
    assertThat(createdHistogram.mean()).isCloseTo(38.333, Offset.offset(0.1))
    assertThat(createdHistogram.max()).isEqualTo(100.0)
  }

  @Test
  fun `createSimpleTimer creates timer with specified parameters`() {
    fun mockTimer() {
      Thread.sleep(200L)
    }

    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val timer = metricsFacade.createSimpleTimer<Unit>(
      name = "some.timer.metric",
      description = "This is a test metric",
      tags = expectedTags
    )

    timer.captureTime(::mockTimer)
    val createdTimer = meterRegistry.find("linea.test.some.timer.metric").timer()
    assertThat(createdTimer).isNotNull
    assertThat(createdTimer!!.id.description).isEqualTo("This is a test metric")
    assertThat(createdTimer.id.tags).isEqualTo(listOf(ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")))
    assertThat(createdTimer.max(TimeUnit.SECONDS)).isGreaterThan(0.2)

    timer.captureTime(::mockTimer)
    assertThat(createdTimer.totalTime(TimeUnit.SECONDS)).isGreaterThan(0.4)
    assertThat(createdTimer.mean(TimeUnit.SECONDS)).isGreaterThan(0.2)
  }

  @Test
  fun `createDynamicTagTimer creates timer with specified parameters`() {
    fun mockTimer() {
      Thread.sleep(200L)
    }

    val timer = metricsFacade.createDynamicTagTimer<Unit>(
      name = "some.dynamictag.timer.metric",
      description = "This is a test metric",
      tagKey = "key",
      tagValueExtractorOnError = { "unfound_key" }
    ) {
      "value"
    }

    timer.captureTime(::mockTimer)
    val createdTimer = meterRegistry.find("linea.test.some.dynamictag.timer.metric").timer()
    assertThat(createdTimer).isNotNull
    assertThat(createdTimer!!.id.description).isEqualTo("This is a test metric")
    assertThat(createdTimer.id.tags).isEqualTo(listOf(ImmutableTag("key", "value")))
    assertThat(createdTimer.max(TimeUnit.SECONDS)).isGreaterThan(0.2)

    timer.captureTime(::mockTimer)
    assertThat(createdTimer.totalTime(TimeUnit.SECONDS)).isGreaterThan(0.4)
    assertThat(createdTimer.mean(TimeUnit.SECONDS)).isGreaterThan(0.2)
  }

  @Test
  fun `createGauge creates gauge with correct name when metrics prefix and category are absent`() {
    val metricMeasureValue = 0L
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    metricsFacade.createGauge(
      name = "some.gauge.metric",
      description = "This is a test metric",
      measurementSupplier = { metricMeasureValue },
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    )
    val createdGauge = meterRegistry.find("some.gauge.metric").gauge()
    assertThat(createdGauge).isNotNull
  }

  @Test
  fun `createCounter creates counter with correct name when metrics prefix and category are absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    metricsFacade.createCounter(
      name = "some.counter.metric",
      description = "This is a test metric",
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    )
    val createdCounter = meterRegistry.find("some.counter.metric").counter()
    assertThat(createdCounter).isNotNull
  }

  @Test
  fun `createHistogram creates histogram with correct name when metrics prefix and category are absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    metricsFacade.createHistogram(
      name = "some.histogram.metric",
      description = "This is a test metric",
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2")),
      baseUnit = "seconds"
    )
    val createdHistogram = meterRegistry.find("some.histogram.metric").summary()
    assertThat(createdHistogram).isNotNull
  }

  @Test
  fun `createSimpleTimer creates timer with correct name when metrics prefix and category are absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    val timer = metricsFacade.createSimpleTimer<Unit>(
      name = "some.timer.metric",
      description = "This is a test metric",
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    )
    timer.captureTime {}
    val createdTimer = meterRegistry.find("some.timer.metric").timer()
    assertThat(createdTimer).isNotNull
  }

  @Test
  fun `createDynamicTagTimer creates timer with correct name when metrics prefix and category are absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    val timer = metricsFacade.createDynamicTagTimer<Unit>(
      name = "some.dynamictag.timer.metric",
      description = "This is a test metric",
      tagKey = "key",
      tagValueExtractorOnError = { "unfound_key" }
    ) {
      "value"
    }
    timer.captureTime {}
    val createdTimer = meterRegistry.find("some.dynamictag.timer.metric").timer()
    assertThat(createdTimer).isNotNull
  }
}
