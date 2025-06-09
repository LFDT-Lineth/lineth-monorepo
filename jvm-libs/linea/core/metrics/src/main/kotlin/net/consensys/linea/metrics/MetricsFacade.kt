package net.consensys.linea.metrics

import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
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

interface CounterFactory {
  fun create(tags: List<Tag> = emptyList()): Counter
}

interface TimerFactory {
  fun create(tags: List<Tag> = emptyList()): Timer
}

interface Timer {
  fun <T> captureTime(f: CompletableFuture<T>): CompletableFuture<T>
  fun <T> captureTime(action: Callable<T>): T
}

interface DynamicTagTimer<T> {
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
  ): Counter = createCounterFactory(category, name, description, tags).create()

  fun createHistogram(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag> = emptyList(),
    isRatio: Boolean = false,
    baseUnit: String? = null,
  ): Histogram

  fun createTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tags: List<Tag> = emptyList(),
  ): Timer

  fun <T> createDynamicTagTimer(
    category: MetricsCategory,
    name: String,
    description: String,
    tagValueExtractorOnError: (Throwable) -> List<Tag>,
    tagValueExtractor: (T) -> List<Tag>,
    commonTags: List<Tag> = emptyList(),
  ): DynamicTagTimer<T>

  fun createCounterFactory(
    category: MetricsCategory,
    name: String,
    description: String,
    commonTags: List<Tag> = emptyList(),
  ): CounterFactory

  fun createTimerFactory(
    category: MetricsCategory,
    name: String,
    description: String,
    commonTags: List<Tag> = emptyList(),
  ): TimerFactory
}

class FakeHistogram : Histogram {
  val records = mutableListOf<Double>()
  override fun record(data: Double) {
    records.add(data)
  }
}
