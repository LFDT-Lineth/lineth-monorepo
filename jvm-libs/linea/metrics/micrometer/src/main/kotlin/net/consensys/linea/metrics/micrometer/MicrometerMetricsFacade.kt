package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.DistributionSummary
import io.micrometer.core.instrument.Gauge
import io.micrometer.core.instrument.MeterRegistry
import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.Histogram
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import net.consensys.linea.metrics.TimerCapture
import java.util.function.Function
import java.util.function.Supplier
import io.micrometer.core.instrument.Counter as MicrometerCounter
import io.micrometer.core.instrument.Timer as MicrometerTimer

class MicrometerMetricsFacade(
  private val registry: MeterRegistry,
  private val metricsPrefix: String? = null
) : MetricsFacade {
  companion object {
    private val validBaseUnits = listOf(
      "seconds",
      "minutes",
      "hours"
    )

    fun requireValidMicrometerName(name: String) {
      require(name.lowercase().trim() == name && name.all { it.isLetterOrDigit() || it == '.' }) {
        "$name must adhere to Micrometer naming convention!"
      }
    }
    fun requireValidBaseUnit(baseUnit: String) {
      require(validBaseUnits.contains(baseUnit))
    }
  }

  init {
    if (metricsPrefix != null) requireValidMicrometerName(metricsPrefix)
  }

  private fun metricHandle(category: LineaMetricsCategory?, metricName: String): String {
    val prefixName = if (metricsPrefix == null) "" else "$metricsPrefix."
    val categoryName = if (category == null) "" else "$category."
    return "$prefixName$categoryName$metricName"
  }

  override fun createGauge(
    category: LineaMetricsCategory?,
    name: String,
    description: String,
    measurementSupplier: Supplier<Number>,
    tags: List<Tag>
  ) {
    if (category != null) requireValidMicrometerName(category.toString())
    requireValidMicrometerName(name)
    val builder = Gauge.builder(metricHandle(category, name), measurementSupplier)
    if (tags.isNotEmpty()) {
      val flatTags = tags.flatMap {
        requireValidMicrometerName(it.key)
        listOf(it.key, it.value)
      }
      builder.tags(*flatTags.toTypedArray())
    }
    builder.description(description)
    builder.register(registry)
  }

  override fun createCounter(
    category: LineaMetricsCategory?,
    name: String,
    description: String,
    tags: List<Tag>
  ): Counter {
    if (category != null) requireValidMicrometerName(category.toString())
    requireValidMicrometerName(name)
    val builder = MicrometerCounter.builder(metricHandle(category, name))
    if (tags.isNotEmpty()) {
      val flatTags = tags.flatMap {
        requireValidMicrometerName(it.key)
        listOf(it.key, it.value)
      }
      builder.tags(*flatTags.toTypedArray())
    }
    builder.description(description)
    return MicrometerCounterAdapter(builder.register(registry))
  }

  override fun createHistogram(
    category: LineaMetricsCategory?,
    name: String,
    description: String,
    tags: List<Tag>,
    isRatio: Boolean,
    baseUnit: String?
  ): Histogram {
    if (category != null) requireValidMicrometerName(category.toString())
    requireValidMicrometerName(name)
    if (baseUnit != null) requireValidBaseUnit(baseUnit)
    val distributionSummaryBuilder = DistributionSummary.builder(metricHandle(category, name))
    if (tags.isNotEmpty()) {
      val flatTags = tags.flatMap {
        requireValidMicrometerName(it.key)
        listOf(it.key, it.value)
      }
      distributionSummaryBuilder.tags(*flatTags.toTypedArray())
    }
    distributionSummaryBuilder.description(description)
    distributionSummaryBuilder.baseUnit(baseUnit)
    if (isRatio) {
      distributionSummaryBuilder.scale(100.0)
      distributionSummaryBuilder.maximumExpectedValue(100.0)
    }
    return MicrometerHistogramAdapter(distributionSummaryBuilder.register(registry))
  }

  override fun <T> createSimpleTimer(
    category: LineaMetricsCategory?,
    name: String,
    description: String,
    tags: List<Tag>
  ): TimerCapture<T> {
    if (category != null) requireValidMicrometerName(category.toString())
    requireValidMicrometerName(name)
    val builder = MicrometerTimer.builder(metricHandle(category, name))
    if (tags.isNotEmpty()) {
      val flatTags = tags.flatMap {
        requireValidMicrometerName(it.key)
        listOf(it.key, it.value)
      }
      builder.tags(*flatTags.toTypedArray())
    }
    builder.description(description)

    return SimpleTimerCapture(registry, builder)
  }

  override fun <T> createDynamicTagTimer(
    category: LineaMetricsCategory?,
    name: String,
    description: String,
    tagKey: String,
    tagValueExtractorOnError: Function<Throwable, String>,
    tagValueExtractor: Function<T, String>
  ): TimerCapture<T> {
    if (category != null) requireValidMicrometerName(category.toString())
    requireValidMicrometerName(name)
    requireValidMicrometerName(tagKey)
    return DynamicTagTimerCapture<T>(registry, metricHandle(category, name))
      .setDescription(description)
      .setTagKey(tagKey)
      .setTagValueExtractor(tagValueExtractor)
      .setTagValueExtractorOnError(tagValueExtractorOnError)
  }
}
