package net.consensys.zkevm.ethereum.coordination.conflation

import linea.blob.BlobCompressor
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCounters
import net.consensys.zkevm.ethereum.coordination.DynamicBlockNumberSet
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
    extraSyncCalculators: List<ConflationCalculator> = emptyList(),
    deferredTriggerConflationCalculators: List<DeferredTriggerConflationCalculator> = emptyList(),
    metricsFacade: MetricsFacade,
    log: Logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java),
  ): GlobalBlobAwareConflationCalculator {
    val blobCalculator = ConflationCalculatorByDataCompressed(blobCompressor = blobCompressor)
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
      it.filterIsInstance<TimestampHardForkConflationCalculator>().forEach { calculator ->
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
    compressedBlobCalculator: ConflationCalculatorByDataCompressed,
    lastProcessedTimestamp: Instant,
    dynamicTargetEndBlockNumberSet: DynamicBlockNumberSet,
    logger: Logger,
    metricsFacade: MetricsFacade,
  ): List<ConflationCalculator> {
    val calculators: MutableList<ConflationCalculator> =
      mutableListOf(
        ConflationCalculatorByExecutionTraces(
          tracesCountersLimit = tracesCountersLimit,
          emptyTracesCounters = tracesCountersLimit.emptyTracesCounters,
          metricsFacade = metricsFacade,
          log = logger,
        ),
        ConflationCalculatorByTargetBlockNumbers(targetEndBlockNumbers = dynamicTargetEndBlockNumberSet),
        compressedBlobCalculator,
      )
    if (blocksLimit != null) {
      calculators.add(ConflationCalculatorByBlockLimit(blockLimit = blocksLimit))
    }
    if (timestampBasedHardForks.isNotEmpty()) {
      calculators.add(
        TimestampHardForkConflationCalculator(
          hardForkTimestamps = timestampBasedHardForks,
          initialTimestamp = lastProcessedTimestamp,
        ),
      )
    }
    return calculators
  }
}
