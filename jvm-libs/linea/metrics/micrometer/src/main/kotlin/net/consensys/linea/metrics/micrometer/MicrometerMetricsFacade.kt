package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.DistributionSummary
import io.micrometer.core.instrument.Gauge
import io.micrometer.core.instrument.MeterRegistry
import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.CounterFactory
import net.consensys.linea.metrics.DynamicTagTimer
import net.consensys.linea.metrics.Histogram
import net.consensys.linea.metrics.MetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import net.consensys.linea.metrics.Timer
import net.consensys.linea.metrics.TimerFactory
import java.util.function.Supplier

class MicrometerMetricsFacade(
  private val registry: MeterRegistry,
  private val metricsPrefix: String? = null,
  private val allMetricsCommonTags: List<Tag> = emptyList(),
) : MetricsFacade {
  companion object {
    private val validBaseUnits = listOf(
      "seconds",
      "minutes",
      "hours",
    )

    fun requireValidBaseUnit(baseUnit: String) {
      require(validBaseUnits.contains(baseUnit))
    }
  }

  init {
    metricsPrefix?.requireValidMicrometerName()
    allMetricsCommonTags.forEach { it.requireValidMicrometerName() }
  }

  private val allMetricsCommonMicrometerTags = allMetricsCommonTags.toMicrometerTags()

  private fun metricHandle(category: MetricsCategory, metricName: String): String {
    val prefixName = if (metricsPrefix == null) "" else "$metricsPrefix."
    return "$prefixName${category.toValidMicrometerName()}.$metricName"
  }

  override fun createGauge(
    category: MetricsCategory,
    name: String,
    description: String,
    measurementSupplier: Supplier<Number>,
    tags: List<Tag>,
  ) {
    category.toValidMicrometerName().requireValidMicrometerName()
    name.requireValidMicrometerName()
    tags.forEach { it.requireValidMicrometerName() }
    val builder = Gauge.builder(metricHandle(category, name), measurementSupplier)
      .description(description)
      .tags(allMetricsCommonMicrometerTags)
      .tags(tags.toMicrometerTags())
    builder.register(registry)
  }

  override fun createCounter(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag>,
  ): Counter = createCounterFactory(category, name, description, tags).create()

  override fun createHistogram(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag>,
    isRatio: Boolean,
    baseUnit: String?,
  ): Histogram {
    category.toValidMicrometerName().requireValidMicrometerName()
    name.requireValidMicrometerName()
    tags.forEach { it.requireValidMicrometerName() }
    if (baseUnit != null) requireValidBaseUnit(baseUnit)
    val distributionSummaryBuilder = DistributionSummary.builder(metricHandle(category, name))
      .description(description)
      .baseUnit(baseUnit)
      .tags(allMetricsCommonMicrometerTags)
      .tags(tags.toMicrometerTags())
    if (isRatio) {
      distributionSummaryBuilder.scale(100.0)
      distributionSummaryBuilder.maximumExpectedValue(100.0)
    }
    return MicrometerHistogramAdapter(distributionSummaryBuilder.register(registry))
  }

  override fun createTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag>,
  ): Timer = createTimerFactory(category, name, description, tags).create()

  override fun <T> createDynamicTagTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tagValueExtractorOnError: (Throwable) -> List<Tag>,
    tagValueExtractor: (T) -> List<Tag>,
    commonTags: List<Tag>,
  ): DynamicTagTimer<T> {
    category.toValidMicrometerName().requireValidMicrometerName()
    name.requireValidMicrometerName()
    commonTags.forEach { it.requireValidMicrometerName() }
    return DynamicTagTimerImpl<T>(
      meterRegistry = registry,
      name = metricHandle(category, name),
      description = description,
      commonTags = commonTags + this.allMetricsCommonTags,
      extractor = tagValueExtractor,
      extractorOnError = tagValueExtractorOnError,
    )
  }

  override fun createCounterFactory(
    category: MetricsCategory,
    name: String,
    description: String,
    commonTags: List<Tag>,
  ): CounterFactory {
    category.toValidMicrometerName().requireValidMicrometerName()
    name.requireValidMicrometerName()
    commonTags.forEach { it.requireValidMicrometerName() }
    return CounterFactoryImpl(
      meterRegistry = registry,
      name = metricHandle(category, name),
      description = description,
      commonTags = commonTags + this.allMetricsCommonTags,
    )
  }

  override fun createTimerFactory(
    category: MetricsCategory,
    name: String,
    description: String,
    commonTags: List<Tag>,
  ): TimerFactory {
    category.toValidMicrometerName().requireValidMicrometerName()
    name.requireValidMicrometerName()
    commonTags.forEach { it.requireValidMicrometerName() }
    return TimerFactoryImpl(
      meterRegistry = registry,
      name = metricHandle(category, name),
      description = description,
      commonTags = commonTags + this.allMetricsCommonTags,
    )
  }
}
