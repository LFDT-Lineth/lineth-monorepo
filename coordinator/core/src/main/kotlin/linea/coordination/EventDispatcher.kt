package linea.coordination

import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.MetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.function.Consumer

class EventDispatcher<T>(
  private val consumers: Map<Consumer<T>, String>,
  private val log: Logger = LogManager.getLogger(EventDispatcher::class.java),
  metricsFacade: MetricsFacade? = null,
  metricsCategory: MetricsCategory? = null,
) : Consumer<T> {
  private val consumerFailureCounters: Map<String, Counter>? =
    if (metricsFacade != null && metricsCategory != null) {
      val counterFactory = metricsFacade.createCounterFactory(
        category = metricsCategory,
        name = "event.consumer.failures",
        description = "Number of failures while dispatching an event to a consumer",
      )
      consumers.values.associateWith { consumerName ->
        counterFactory.create(listOf(Tag("consumer", consumerName)))
      }
    } else {
      null
    }

  override fun accept(event: T) {
    consumers.forEach { (consumer, name) ->
      try {
        consumer.accept(event)
      } catch (e: Exception) {
        log.warn(
          "Failed to consume event: consumer={} event={} errorMessage={}",
          name,
          event,
          e.message,
          e,
        )
        consumerFailureCounters?.get(name)?.increment()
      }
    }
  }
}
