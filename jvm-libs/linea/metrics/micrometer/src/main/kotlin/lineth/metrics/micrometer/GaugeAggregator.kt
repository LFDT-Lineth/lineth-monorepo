package lineth.metrics.micrometer

import java.util.function.Supplier

/**
 * Util class to aggregate multiple counters/gauges into a single one.
 * Useful for gauges where the total counting needs to come from multiple sources
 *
 * Note: it was considered using a WeakHashMap to store the reporters,
 * but if the supplier is a lambda, it will be garbage collected and the value will be lost.
 * Reporters are expected to be long-lived objects for the whole application lifespan
 * so it should not be a problem.
 */
class GaugeAggregator : Supplier<Number> {
  private val reporters = mutableSetOf<Supplier<Number>>()

  @Synchronized
  fun addReporter(reporter: Supplier<Number>) {
    reporters.add(reporter)
  }

  @Synchronized
  override fun get(): Number {
    return reporters.sumOf { it.get().toLong() }
  }
}
