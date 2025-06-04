package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.DistributionSummary
import io.micrometer.core.instrument.Gauge
import io.micrometer.core.instrument.MeterRegistry
import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.Histogram
import net.consensys.linea.metrics.MetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import net.consensys.linea.metrics.TimerCapture
import java.util.function.Function
import java.util.function.Supplier
import io.micrometer.core.instrument.Counter as MicrometerCounter
import io.micrometer.core.instrument.Timer as MicrometerTimer

class MicrometerMetricsFacade(
  private val registry: MeterRegistry,
  private val metricsPrefix: String? = null,
  private val commonTags: List<Tag> = emptyList(),
) : MetricsFacade {
  companion object {
    private val validBaseUnits = listOf(
      "seconds",
      "minutes",
      "hours",
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
    commonTags.forEach { requireValidMicrometerName(it.key) }
  }

  private fun metricHandle(category: MetricsCategory, metricName: String): String {
    val prefixName = if (metricsPrefix == null) "" else "$metricsPrefix."
    val categoryName = "${category.toValidMicrometerName()}."
    return "$prefixName$categoryName$metricName"
  }

  private fun flattenTags(tags: List<Tag>): List<String> {
    tags.forEach { requireValidMicrometerName(it.key) }
    return (tags + commonTags).flatMap {
      listOf(it.key, it.value)
    }
  }

  override fun createGauge(
    category: MetricsCategory,
    name: String,
    description: String,
    measurementSupplier: Supplier<Number>,
    tags: List<Tag>,
  ) {
    requireValidMicrometerName(category.toValidMicrometerName())
    requireValidMicrometerName(name)
    val builder = Gauge.builder(metricHandle(category, name), measurementSupplier)
    flattenTags(tags).takeIf { it.isNotEmpty() }?.let { builder.tags(*it.toTypedArray()) }
    builder.description(description)
    builder.register(registry)
  }

  override fun createCounter(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag>,
  ): Counter {
    requireValidMicrometerName(category.toValidMicrometerName())
    requireValidMicrometerName(name)
    val builder = MicrometerCounter.builder(metricHandle(category, name))
    flattenTags(tags).takeIf { it.isNotEmpty() }?.let { builder.tags(*it.toTypedArray()) }
    builder.description(description)
    return MicrometerCounterAdapter(builder.register(registry))
  }

  override fun createHistogram(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag>,
    isRatio: Boolean,
    baseUnit: String?,
  ): Histogram {
    requireValidMicrometerName(category.toValidMicrometerName())
    requireValidMicrometerName(name)
    if (baseUnit != null) requireValidBaseUnit(baseUnit)
    val distributionSummaryBuilder = DistributionSummary.builder(metricHandle(category, name))
    flattenTags(tags).takeIf { it.isNotEmpty() }?.let { distributionSummaryBuilder.tags(*it.toTypedArray()) }
    distributionSummaryBuilder.description(description)
    distributionSummaryBuilder.baseUnit(baseUnit)
    if (isRatio) {
      distributionSummaryBuilder.scale(100.0)
      distributionSummaryBuilder.maximumExpectedValue(100.0)
    }
    return MicrometerHistogramAdapter(distributionSummaryBuilder.register(registry))
  }

  override fun <T> createSimpleTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag>,
  ): TimerCapture<T> {
    requireValidMicrometerName(category.toValidMicrometerName())
    requireValidMicrometerName(name)
    val builder = MicrometerTimer.builder(metricHandle(category, name))
    flattenTags(tags).takeIf { it.isNotEmpty() }?.let { builder.tags(*it.toTypedArray()) }
    builder.description(description)

    return SimpleTimerCapture(registry, builder)
  }

  override fun <T> createDynamicTagTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tagKey: String,
    tagValueExtractorOnError: Function<Throwable, String>,
    tagValueExtractor: Function<T, String>,
  ): TimerCapture<T> {
    requireValidMicrometerName(category.toValidMicrometerName())
    requireValidMicrometerName(name)
    requireValidMicrometerName(tagKey)
    val dynamicTagTimerCapture = DynamicTagTimerCapture<T>(registry, metricHandle(category, name))
      .setDescription(description)
      .setTagKey(tagKey)
      .setTagValueExtractor(tagValueExtractor)
      .setTagValueExtractorOnError(tagValueExtractorOnError)
    commonTags.forEach { dynamicTagTimerCapture.setTag(it.key, it.value) }
    return dynamicTagTimerCapture
  }
}
