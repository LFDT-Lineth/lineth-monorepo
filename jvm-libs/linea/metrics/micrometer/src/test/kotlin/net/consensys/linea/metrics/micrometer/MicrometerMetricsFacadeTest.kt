package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.ImmutableTag
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.micrometer.prometheus.PrometheusConfig
import io.micrometer.prometheus.PrometheusMeterRegistry
import net.consensys.linea.metrics.MetricsCategory
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

  enum class TestCategory : MetricsCategory {
    TEST_CATEGORY,
  }

  @BeforeEach
  fun beforeEach() {
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(
      meterRegistry,
      metricsPrefix = "linea.test",
      allMetricsCommonTags = listOf(Tag("version", "1.0.1")),
    )
  }

  @Test
  fun `createGauge creates gauge with specified parameters`() {
    var metricMeasureValue = 0L
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    metricsFacade.createGauge(
      category = TestCategory.TEST_CATEGORY,
      name = "some.metric",
      description = "This is a test metric",
      measurementSupplier = { metricMeasureValue },
      tags = expectedTags,
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
  fun `createCounter creates counter with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val counter = metricsFacade.createCounter(
      category = TestCategory.TEST_CATEGORY,
      name = "some.metric",
      description = "This is a test metric",
      tags = expectedTags,
    )
    val createdCounter = meterRegistry.find("linea.test.test.category.some.metric").counter()
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
    assertThat(
      createdCounter.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
    assertThat(createdCounter.id.description).isEqualTo("This is a test metric")
  }

  @Test
  fun `createHistogram creates histogram with specified parameters`() {
    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val histogram = metricsFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "some.metric",
      description = "This is a test metric",
      tags = expectedTags,
      baseUnit = "seconds",
    )

    val createdHistogram = meterRegistry.find("linea.test.test.category.some.metric").summary()
    assertThat(createdHistogram).isNotNull
    assertThat(createdHistogram!!.id.description).isEqualTo("This is a test metric")
    assertThat(
      createdHistogram.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
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
  fun `createHistogram with publishPercentileHistogram enables histogram buckets`() {
    val prometheusRegistry = PrometheusMeterRegistry(PrometheusConfig.DEFAULT)
    val prometheusFacade = MicrometerMetricsFacade(prometheusRegistry, metricsPrefix = "linea.test")
    val histogram = prometheusFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "hist.buckets.metric",
      description = "Histogram with percentile histogram",
      baseUnit = "seconds",
      publishPercentileHistogram = true,
    )

    histogram.record(10.0)
    histogram.record(50.0)

    val scrapeOutput = prometheusRegistry.scrape()
    assertThat(scrapeOutput).contains("linea_test_test_category_hist_buckets_metric_seconds_bucket{le=\"")
  }

  @Test
  fun `createHistogram without publishPercentileHistogram does not enable extra histogram buckets`() {
    val prometheusRegistry = PrometheusMeterRegistry(PrometheusConfig.DEFAULT)
    val prometheusFacade = MicrometerMetricsFacade(prometheusRegistry, metricsPrefix = "linea.test")
    val histogram = prometheusFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "no.hist.buckets.metric",
      description = "Histogram without percentile histogram",
      baseUnit = "seconds",
    )

    histogram.record(10.0)
    histogram.record(50.0)

    val scrapeOutput = prometheusRegistry.scrape()
    // Without publishPercentileHistogram, Prometheus still produces default buckets
    // but should not have the fine-grained Micrometer percentile histogram buckets
    val bucketLines = scrapeOutput.lines()
      .filter { it.startsWith("linea_test_test_category_no_hist_buckets_metric_seconds_bucket{") }
    // Default Prometheus buckets are coarse (typically ~15 buckets)
    // publishPercentileHistogram() generates many more (60+)
    assertThat(bucketLines.size).isLessThan(20)
  }

  @Test
  fun `createHistogram with percentileBuckets publishes specified percentiles`() {
    val percentiles = listOf(0.5, 0.95, 0.99)
    val histogram = metricsFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "some.percentile.metric",
      description = "Histogram with percentiles",
      tags = listOf(Tag("key1", "value1")),
      baseUnit = "seconds",
      percentileBuckets = percentiles,
    )

    for (i in 1..100) {
      histogram.record(i.toDouble())
    }

    val createdHistogram = meterRegistry.find("linea.test.test.category.some.percentile.metric").summary()
    assertThat(createdHistogram).isNotNull
    assertThat(createdHistogram!!.count()).isEqualTo(100L)

    val percentileSnapshots = createdHistogram.takeSnapshot().percentileValues()
    assertThat(percentileSnapshots).isNotEmpty
    val reportedPercentiles = percentileSnapshots.map { it.percentile() }
    assertThat(reportedPercentiles).containsAll(percentiles)
  }

  @Test
  fun `createHistogram without percentileBuckets does not publish specific percentiles`() {
    val histogram = metricsFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "some.no.percentile.metric",
      description = "Histogram without percentiles",
      baseUnit = "seconds",
    )

    for (i in 1..100) {
      histogram.record(i.toDouble())
    }

    val createdHistogram = meterRegistry.find("linea.test.test.category.some.no.percentile.metric").summary()
    assertThat(createdHistogram).isNotNull
    assertThat(createdHistogram!!.count()).isEqualTo(100L)

    val percentileSnapshots = createdHistogram.takeSnapshot().percentileValues()
    assertThat(percentileSnapshots).isEmpty()
  }

  @Test
  fun `createHistogram with both publishPercentileHistogram and percentileBuckets enables both`() {
    val prometheusRegistry = PrometheusMeterRegistry(PrometheusConfig.DEFAULT)
    val prometheusFacade = MicrometerMetricsFacade(prometheusRegistry, metricsPrefix = "linea.test")
    val percentiles = listOf(0.5, 0.99)
    val histogram = prometheusFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "both.metric",
      description = "Histogram with both options",
      baseUnit = "seconds",
      publishPercentileHistogram = true,
      percentileBuckets = percentiles,
    )

    for (i in 1..100) {
      histogram.record(i.toDouble())
    }

    val scrapeOutput = prometheusRegistry.scrape()
    // Has fine-grained histogram buckets
    assertThat(scrapeOutput).contains("linea_test_test_category_both_metric_seconds_bucket{le=\"")
    val bucketLines = scrapeOutput.lines()
      .filter { it.startsWith("linea_test_test_category_both_metric_seconds_bucket{") }
    assertThat(bucketLines.size).isGreaterThan(20)

    // Also has client-side percentile quantiles
    assertThat(scrapeOutput).contains("linea_test_test_category_both_metric_seconds{quantile=\"0.5\"")
    assertThat(scrapeOutput).contains("linea_test_test_category_both_metric_seconds{quantile=\"0.99\"")
  }

  @Test
  fun `createSimpleTimer creates timer with specified parameters`() {
    fun mockTimer() {
      Thread.sleep(200L)
    }

    val expectedTags = listOf(Tag("key1", "value1"), Tag("key2", "value2"))
    val timer = metricsFacade.createTimer(
      category = TestCategory.TEST_CATEGORY,
      name = "some.timer.metric",
      description = "This is a test metric",
      tags = expectedTags,
    )

    timer.captureTime(::mockTimer)
    val createdTimer = meterRegistry.find("linea.test.test.category.some.timer.metric").timer()
    assertThat(createdTimer).isNotNull
    assertThat(createdTimer!!.id.description).isEqualTo("This is a test metric")
    assertThat(
      createdTimer.id.tags,
    ).containsAll(
      listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key1", "value1"), ImmutableTag("key2", "value2")),
    )
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
      category = TestCategory.TEST_CATEGORY,
      name = "some.dynamictag.timer.metric",
      description = "This is a test metric",
      tagValueExtractorOnError = { listOf(Tag(key = "key", value = "unfound_key")) },
      tagValueExtractor = { listOf(Tag(key = "key", value = "value")) },
    )

    timer.captureTime(::mockTimer)
    val createdTimer = meterRegistry.find("linea.test.test.category.some.dynamictag.timer.metric").timer()
    assertThat(createdTimer).isNotNull
    assertThat(createdTimer!!.id.description).isEqualTo("This is a test metric")
    assertThat(createdTimer.id.tags).containsAll(listOf(ImmutableTag("version", "1.0.1"), ImmutableTag("key", "value")))
    assertThat(createdTimer.max(TimeUnit.SECONDS)).isGreaterThan(0.2)

    timer.captureTime(::mockTimer)
    assertThat(createdTimer.totalTime(TimeUnit.SECONDS)).isGreaterThan(0.4)
    assertThat(createdTimer.mean(TimeUnit.SECONDS)).isGreaterThan(0.2)
  }

  @Test
  fun `createGauge creates gauge with correct name when metrics prefix is absent`() {
    val metricMeasureValue = 0L
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    metricsFacade.createGauge(
      category = TestCategory.TEST_CATEGORY,
      name = "some.gauge.metric",
      description = "This is a test metric",
      measurementSupplier = { metricMeasureValue },
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2")),
    )
    val createdGauge = meterRegistry.find("test.category.some.gauge.metric").gauge()
    assertThat(createdGauge).isNotNull
  }

  @Test
  fun `createCounter creates counter with correct name when metrics prefix is absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    metricsFacade.createCounter(
      category = TestCategory.TEST_CATEGORY,
      name = "some.counter.metric",
      description = "This is a test metric",
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2")),
    )
    val createdCounter = meterRegistry.find("test.category.some.counter.metric").counter()
    assertThat(createdCounter).isNotNull
  }

  @Test
  fun `createHistogram creates histogram with correct name when metrics prefix is absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    metricsFacade.createHistogram(
      category = TestCategory.TEST_CATEGORY,
      name = "some.histogram.metric",
      description = "This is a test metric",
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2")),
      baseUnit = "seconds",
    )
    val createdHistogram = meterRegistry.find("test.category.some.histogram.metric").summary()
    assertThat(createdHistogram).isNotNull
  }

  @Test
  fun `createSimpleTimer creates timer with correct name when metrics prefix is absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    val timer = metricsFacade.createTimer(
      category = TestCategory.TEST_CATEGORY,
      name = "some.timer.metric",
      description = "This is a test metric",
      tags = listOf(Tag("key1", "value1"), Tag("key2", "value2")),
    )
    timer.captureTime {}
    val createdTimer = meterRegistry.find("test.category.some.timer.metric").timer()
    assertThat(createdTimer).isNotNull
  }

  @Test
  fun `createDynamicTagTimer creates timer with correct name when metrics prefix is absent`() {
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    val timer = metricsFacade.createDynamicTagTimer<Unit>(
      category = TestCategory.TEST_CATEGORY,
      name = "some.dynamictag.timer.metric",
      description = "This is a test metric",
      tagValueExtractorOnError = { listOf(Tag("key", "unfound_key")) },
      tagValueExtractor = { listOf(Tag("key", "value")) },
    )
    timer.captureTime {}
    val createdTimer = meterRegistry.find("test.category.some.dynamictag.timer.metric").timer()
    assertThat(createdTimer).isNotNull
  }

  @Test
  fun `counter provider creates multiple counters`() {
    val requestCounter = metricsFacade.createCounterFactory(
      category = TestCategory.TEST_CATEGORY,
      name = "request.counter",
      description = "This is a test counter provider",
      commonTags = listOf(Tag("apitype", "engine_prague")),
    )
    requestCounter.create(listOf(Tag("method", "getPayload")))
      .increment(5.0)
    requestCounter.create(listOf(Tag("method", "newPayload")))
      .increment(3.0)
    requestCounter.create(listOf(Tag("method", "getPayload")))
      .increment()

    val createdCounter1 = meterRegistry
      .find("linea.test.test.category.request.counter")
      .tags("method", "getPayload")
      .counter()

    val createdCounter2 = meterRegistry
      .find("linea.test.test.category.request.counter")
      .tags("method", "newPayload")
      .counter()

    assertThat(createdCounter1).isNotNull
    assertThat(createdCounter2).isNotNull
    assertThat(createdCounter1!!.count()).isEqualTo(6.0)
    assertThat(createdCounter2!!.count()).isEqualTo(3.0)
  }

  @Test
  fun `timer provider creates multiple timers`() {
    val requestTimer = metricsFacade.createTimerFactory(
      category = TestCategory.TEST_CATEGORY,
      name = "request.latency",
      description = "This is a test counter provider",
      commonTags = listOf(Tag("apitype", "engine_prague")),
    )
    requestTimer.create(listOf(Tag("method", "getPayload")))
      .captureTime { Thread.sleep(2) }
    requestTimer.create(listOf(Tag("method", "newPayload")))
      .captureTime { Thread.sleep(10) }
    requestTimer.create(listOf(Tag("method", "getPayload")))
      .captureTime { Thread.sleep(2) }

    val createdTimer1 = meterRegistry
      .find("linea.test.test.category.request.latency")
      .tags("method", "getPayload")
      .timer()

    val createdTimer2 = meterRegistry
      .find("linea.test.test.category.request.latency")
      .tags("method", "newPayload")
      .timer()

    assertThat(createdTimer1).isNotNull
    assertThat(createdTimer1!!.count()).isEqualTo(2)
    assertThat(createdTimer2).isNotNull
    assertThat(createdTimer2!!.count()).isEqualTo(1)
  }
}
