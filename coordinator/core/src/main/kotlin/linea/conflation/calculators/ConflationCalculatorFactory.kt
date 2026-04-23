package linea.conflation.calculators

import linea.blob.BlobCompressor
import linea.conflation.DynamicBlockNumberSet
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCounters
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Instant

object ConflationCalculatorFactory {

  fun conflationCalculator(
    blobCompressor: BlobCompressor,
    tracesCountersLimit: TracesCounters,
    blocksLimit: UInt?,
    timestampBasedHardForks: List<Instant> = emptyList(),
    lastProcessedBlockNumber: ULong,
    lastProcessedTimestamp: Instant,
    blobBatchesLimit: UInt? = null,
    aggregationProofsLimit: UInt,
    dynamicBlockNumberSet: DynamicBlockNumberSet,
    extraSyncCalculators: List<ConflationTriggerCalculator> = emptyList(),
    deferredTriggerConflationCalculators: List<DeferredConflationTriggerCalculator> = emptyList(),
    metricsFacade: MetricsFacade,
    log: Logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java),
  ): GlobalBlobAwareConflationCalculator {
    val blobCalculator = ConflationTriggerCalculatorByDataCompressed(blobCompressor = blobCompressor)
    val syncCalculators = createCalculatorsForBlobsAndConflation(
      tracesCountersLimit = tracesCountersLimit,
      blocksLimit = blocksLimit,
      timestampBasedHardForks = timestampBasedHardForks,
      compressedBlobCalculator = blobCalculator,
      lastProcessedTimestamp = lastProcessedTimestamp,
      dynamicTargetEndBlockNumberSet = dynamicBlockNumberSet,
      logger = log,
      metricsFacade = metricsFacade,
    ).also {
      it.filterIsInstance<ConflationTriggerCalculatorByHardForkTimestamp>().forEach { calculator ->
        log.info("Added timestamp-based hard fork calculator={} ", calculator)
      }
    }

    val globalCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastProcessedBlockNumber,
      syncCalculators = syncCalculators + extraSyncCalculators,
      deferredTriggerConflationCalculators = deferredTriggerConflationCalculators,
      emptyTracesCounters = tracesCountersLimit.emptyTracesCounters,
      log = log,
    )

    return GlobalBlobAwareConflationCalculator(
      conflationCalculator = globalCalculator,
      blobCalculator = blobCalculator,
      metricsFacade = metricsFacade,
      batchesLimit = blobBatchesLimit ?: (aggregationProofsLimit - 1U),
      dynamicBlockNumberSet = dynamicBlockNumberSet,
      log = log,
    )
  }

  fun createCalculatorsForBlobsAndConflation(
    tracesCountersLimit: TracesCounters,
    blocksLimit: UInt?,
    timestampBasedHardForks: List<Instant> = emptyList(),
    compressedBlobCalculator: ConflationTriggerCalculatorByDataCompressed,
    lastProcessedTimestamp: Instant,
    dynamicTargetEndBlockNumberSet: DynamicBlockNumberSet,
    logger: Logger,
    metricsFacade: MetricsFacade,
  ): List<ConflationTriggerCalculator> {
    val calculators: MutableList<ConflationTriggerCalculator> =
      mutableListOf(
        ConflationTriggerCalculatorByExecutionTraces(
          tracesCountersLimit = tracesCountersLimit,
          emptyTracesCounters = tracesCountersLimit.emptyTracesCounters,
          metricsFacade = metricsFacade,
          log = logger,
        ),
        ConflationTriggerCalculatorByTargetBlockNumbers(targetEndBlockNumbers = dynamicTargetEndBlockNumberSet),
        compressedBlobCalculator,
      )
    if (blocksLimit != null) {
      calculators.add(ConflationTriggerCalculatorByBlockLimit(blockLimit = blocksLimit))
    }
    if (timestampBasedHardForks.isNotEmpty()) {
      calculators.add(
        ConflationTriggerCalculatorByHardForkTimestamp(
          hardForkTimestamps = timestampBasedHardForks,
          initialTimestamp = lastProcessedTimestamp,
        ),
      )
    }
    return calculators
  }
}
