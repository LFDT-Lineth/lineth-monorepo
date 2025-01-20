package net.consensys.linea.metrics

import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import java.util.function.Function
import java.util.function.Supplier

data class Tag(val key: String, val value: String)

enum class LineaMetricsCategory {
  AGGREGATION,
  BATCH,
  BLOB,
  CONFLATION,
  GAS_PRICE_CAP,
  TX_EXCLUSION_API;

  override fun toString(): String {
    return this.name.replace('_', '.').lowercase()
  }
}

interface Counter {
  fun increment(amount: Double)

  fun increment()
}

interface Histogram {
  fun record(data: Double)
}

interface TimerCapture<T> {
  fun captureTime(f: CompletableFuture<T>): CompletableFuture<T>
  fun captureTime(action: Callable<T>): T
}

interface MetricsFacade {
  fun createGauge(
    category: LineaMetricsCategory? = null,
    name: String,
    description: String,
    measurementSupplier: Supplier<Number>,
    tags: List<Tag> = emptyList()
  )

  fun createCounter(
    category: LineaMetricsCategory? = null,
    name: String,
    description: String,
    tags: List<Tag> = emptyList()
  ): Counter

  fun createHistogram(
    category: LineaMetricsCategory? = null,
    name: String,
    description: String,
    tags: List<Tag> = emptyList(),
    isRatio: Boolean = false,
    baseUnit: String? = null
  ): Histogram

  fun <T> createSimpleTimer(
    category: LineaMetricsCategory? = null,
    name: String,
    description: String,
    tags: List<Tag> = emptyList()
  ): TimerCapture<T>

  fun <T> createDynamicTagTimer(
    category: LineaMetricsCategory? = null,
    name: String,
    description: String,
    tagKey: String,
    tagValueExtractorOnError: Function<Throwable, String>,
    tagValueExtractor: Function<T, String>
  ): TimerCapture<T>
}
