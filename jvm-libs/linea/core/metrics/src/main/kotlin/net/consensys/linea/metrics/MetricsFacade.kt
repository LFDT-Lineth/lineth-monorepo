package net.consensys.linea.metrics

import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import java.util.function.Function
import java.util.function.Supplier

data class Tag(val key: String, val value: String)

interface MetricsCategory {
  val name: String
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
    category: MetricsCategory,
    name: String,
    description: String,
    measurementSupplier: Supplier<Number>,
    tags: List<Tag> = emptyList(),
  )

  fun createCounter(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag> = emptyList(),
  ): Counter

  fun createHistogram(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag> = emptyList(),
    isRatio: Boolean = false,
    baseUnit: String? = null,
  ): Histogram

  fun <T> createSimpleTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag> = emptyList(),
  ): TimerCapture<T>

  fun <T> createDynamicTagTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tagKey: String,
    tagValueExtractorOnError: Function<Throwable, String>,
    tagValueExtractor: Function<T, String>,
  ): TimerCapture<T>
}

class FakeHistogram : Histogram {
  val records = mutableListOf<Double>()
  override fun record(data: Double) {
    records.add(data)
  }
}
