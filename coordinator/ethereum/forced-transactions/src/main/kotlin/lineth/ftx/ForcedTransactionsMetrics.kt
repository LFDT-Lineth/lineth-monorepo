package lineth.ftx

import lineth.metrics.LineaMetricsCategory
import lineth.metrics.Counter
import lineth.metrics.MetricsFacade
import java.util.concurrent.atomic.AtomicLong

internal interface ForcedTransactionsMetrics {
  fun recordAcceptedEvent(forcedTransactionNumber: ULong)
}

internal class ForcedTransactionsMetricsRecorder(
  metricsFacade: MetricsFacade,
  initialLatestForcedTransactionNumber: ULong,
) : ForcedTransactionsMetrics {
  private val latestForcedTransactionNumber = AtomicLong(initialLatestForcedTransactionNumber.toLong())
  private val acceptedEventsCounter: Counter = metricsFacade.createCounter(
    category = LineaMetricsCategory.FORCED_TRANSACTION,
    name = "events.consumed",
    description = "Number of ForcedTransactionAdded L1 events consumed by the coordinator",
  )

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.FORCED_TRANSACTION,
      name = "events.highest",
      description = "Latest ForcedTransactionAdded forced transaction number observed by the coordinator",
      measurementSupplier = { latestForcedTransactionNumber.get() },
    )
  }

  override fun recordAcceptedEvent(forcedTransactionNumber: ULong) {
    latestForcedTransactionNumber.set(forcedTransactionNumber.toLong())
    acceptedEventsCounter.increment()
  }
}

internal object NoOpForcedTransactionsMetrics : ForcedTransactionsMetrics {
  override fun recordAcceptedEvent(forcedTransactionNumber: ULong) {}
}
