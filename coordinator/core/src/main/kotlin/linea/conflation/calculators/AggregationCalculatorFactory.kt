package linea.conflation.calculators

import net.consensys.linea.metrics.MetricsFacade
import kotlin.time.Instant

object AggregationCalculatorFactory {
  fun createAggregationCalculator(
    startBlockNumberInclusive: ULong,
    maxProofsPerAggregation: UInt,
    maxBlobsPerAggregation: UInt?,
    targetEndBlockNumbers: Set<ULong> = emptySet(),
    aggregationSizeMultipleOf: UInt,
    hardForkTimestamps: List<Instant> = emptyList(),
    initialTimestamp: Instant,
    forcedTransactionTriggerAggCalculator: SyncAggregationTriggerCalculator,
    deferredAggregationTriggerCalculators: List<DeferredAggregationTriggerCalculator> = emptyList(),
    metricsFacade: MetricsFacade,
  ): GlobalAggregationCalculator {
    val syncAggregationTriggerCalculators = mutableListOf(
      forcedTransactionTriggerAggCalculator,
      AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = maxProofsPerAggregation),
      AggregationTriggerCalculatorByTargetBlockNumbers(targetEndBlockNumbers = targetEndBlockNumbers),
    )
    if (maxBlobsPerAggregation != null) {
      syncAggregationTriggerCalculators.add(
        AggregationTriggerCalculatorByBlobLimit(maxBlobsPerAggregation = maxBlobsPerAggregation),
      )
    }
    if (hardForkTimestamps.isNotEmpty()) {
      syncAggregationTriggerCalculators.add(
        AggregationTriggerCalculatorByTimestampHardFork(
          hardForkTimestamps = hardForkTimestamps,
          initialTimestamp = initialTimestamp,
        ),
      )
    }

    return GlobalAggregationCalculator(
      lastBlockNumber = startBlockNumberInclusive - 1UL,
      syncAggregationTrigger = syncAggregationTriggerCalculators,
      deferredAggregationTrigger = deferredAggregationTriggerCalculators,
      metricsFacade = metricsFacade,
      aggregationSizeMultipleOf = aggregationSizeMultipleOf,
    )
  }
}
