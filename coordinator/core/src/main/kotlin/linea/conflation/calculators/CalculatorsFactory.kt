package linea.conflation.calculators

import linea.DisabledService
import linea.LongRunningService
import linea.blob.BlobCompressor
import linea.conflation.SafeBlockProvider
import linea.timer.TimerFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCounters
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.concurrent.ConcurrentSkipListSet
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Instant

data class ConflationCalculators(
  val blockConflationCalculator: BlockConflationCalculator,
  val aggregationCalculator: AggregationCalculator,
  val service: LongRunningService,
)

data class AggregationCalculators(
  val aggregationCalculator: AggregationCalculator,
  val service: LongRunningService,
)

object CalculatorsFactory {
  fun create(
    blobCompressor: BlobCompressor,
    tracesCountersLimit: TracesCounters,
    blocksLimit: UInt?,
    lastAggregatedBlockNumber: ULong,
    lastAggregatedTimestamp: Instant,
    lastConflatedBlockNumber: ULong = lastAggregatedBlockNumber,
    lastConflatedTimestamp: Instant = lastAggregatedTimestamp,
    timestampBasedHardForks: List<Instant> = emptyList(),
    blobBatchesLimit: UInt? = null,
    aggregationTargetEndBlockNumbers: Set<ULong> = emptySet(),
    aggregationProofsLimit: UInt,
    aggregationBlobLimit: UInt?,
    aggregationSizeMultipleOf: UInt,
    metricsFacade: MetricsFacade,
    conflationDeadlineCheckInterval: Duration = Duration.ZERO,
    conflationDeadline: Duration? = null,
    conflationDeadlineLastBlockConfirmationDelay: Duration = Duration.ZERO,
    aggregationDeadline: Duration? = null,
    aggregationDeadlineCheckInterval: Duration? = null,
    aggregationDeadlineNoL2ActivityTimeout: Duration? = null,
    extraSyncCalculators: List<ConflationTriggerCalculator> = emptyList(),
    timerFactory: TimerFactory,
    safeBlockProvider: SafeBlockProvider,
    clock: Clock,
    log: Logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java),
  ): ConflationCalculators {
    val aggregationTargetEndBlockNumbersUpdatable: MutableSet<ULong> =
      ConcurrentSkipListSet(aggregationTargetEndBlockNumbers)
    val blobCalculator = ConflationTriggerCalculatorByDataCompressed(blobCompressor = blobCompressor)
    val syncCalculators = createConflationTriggerCalculators(
      tracesCountersLimit = tracesCountersLimit,
      blocksLimit = blocksLimit,
      timestampBasedHardForks = timestampBasedHardForks,
      compressedBlobCalculator = blobCalculator,
      lastConflatedTimestamp = lastConflatedTimestamp,
      aggregationTargetEndBlockNumbers = aggregationTargetEndBlockNumbersUpdatable,
      logger = log,
      metricsFacade = metricsFacade,
    ).also {
      it.filterIsInstance<ConflationTriggerCalculatorByHardForkTimestamp>().forEach { calculator ->
        log.info("Added timestamp-based hard fork calculator={} ", calculator)
      }
    }

    val conflationDeadlineCalculator = conflationDeadline?.let {
      require(conflationDeadlineCheckInterval > Duration.ZERO) {
        "conflationDeadlineCheckInterval must be > 0 when conflationDeadline is set"
      }
      DeadlineConflationTriggerCalculatorRunner(
        conflationDeadlineCheckInterval = conflationDeadlineCheckInterval,
        delegate = ConflationTriggerCalculatorByTimeDeadline(
          config = ConflationTriggerCalculatorByTimeDeadline.Config(
            conflationDeadline = conflationDeadline,
            conflationDeadlineLastBlockConfirmationDelay = conflationDeadlineLastBlockConfirmationDelay,
          ),
          lastBlockNumber = lastConflatedBlockNumber,
          clock = clock,
          latestBlockProvider = safeBlockProvider,
        ),
      )
    }

    val globalCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastConflatedBlockNumber,
      syncCalculators = syncCalculators + extraSyncCalculators,
      deferredTriggerConflationCalculators = listOfNotNull(conflationDeadlineCalculator),
      emptyTracesCounters = tracesCountersLimit.emptyTracesCounters,
      log = log,
    )

    val blockConflationCalculator = GlobalBlobAwareConflationCalculator(
      conflationCalculator = globalCalculator,
      blobCalculator = blobCalculator,
      metricsFacade = metricsFacade,
      batchesLimit = blobBatchesLimit ?: (aggregationProofsLimit - 1U),
      aggregationTargetEndBlocks = aggregationTargetEndBlockNumbersUpdatable,
      log = log,
    )

    val aggCalc = createAggregationCalculator(
      lastAggregatedBlockNumber = lastAggregatedBlockNumber,
      lastAggregatedTimestamp = lastAggregatedTimestamp,
      aggregationTargetEndBlockNumbers = aggregationTargetEndBlockNumbersUpdatable,
      timestampBasedHardForks = timestampBasedHardForks,
      aggregationProofsLimit = aggregationProofsLimit,
      aggregationBlobLimit = aggregationBlobLimit,
      aggregationSizeMultipleOf = aggregationSizeMultipleOf,
      aggregationDeadline = aggregationDeadline,
      aggregationDeadlineCheckInterval = aggregationDeadlineCheckInterval,
      aggregationDeadlineNoL2ActivityTimeout = aggregationDeadlineNoL2ActivityTimeout,
      safeBlockProvider = safeBlockProvider,
      timerFactory = timerFactory,
      metricsFacade = metricsFacade,
      clock = clock,
    )

    // ----------------------------------------
    // Conflation + Aggregation joint
    // ----------------------------------------
    val service = run {
      val confService: LongRunningService = conflationDeadlineCalculator
        ?: DisabledService("Conflation Deadline")
      LongRunningService.compose(confService, aggCalc.service)
    }

    return ConflationCalculators(
      blockConflationCalculator = blockConflationCalculator,
      aggregationCalculator = aggCalc.aggregationCalculator,
      service = service,
    )
  }

  private fun createAggregationCalculator(
    lastAggregatedBlockNumber: ULong,
    lastAggregatedTimestamp: Instant,
    aggregationTargetEndBlockNumbers: Set<ULong> = emptySet(),
    timestampBasedHardForks: List<Instant> = emptyList(),
    aggregationProofsLimit: UInt,
    aggregationBlobLimit: UInt?,
    aggregationSizeMultipleOf: UInt,
    aggregationDeadline: Duration? = null,
    aggregationDeadlineCheckInterval: Duration? = null,
    aggregationDeadlineNoL2ActivityTimeout: Duration? = null,
    safeBlockProvider: SafeBlockProvider,
    timerFactory: TimerFactory,
    metricsFacade: MetricsFacade,
    clock: Clock,
  ): AggregationCalculators {
    val aggregationDeadlineCalculator = aggregationDeadline?.let {
      require(aggregationDeadlineNoL2ActivityTimeout != null) {
        "aggregationDeadlineNoL2ActivityTimeout must be set when aggregationDeadline is set"
      }
      require(aggregationDeadlineCheckInterval != null) {
        "aggregationDeadlineCheckInterval must be defined when aggregationDeadline is set"
      }
      val aggregationCalculatorByDeadline = AggregationTriggerCalculatorByDeadline(
        config =
        AggregationTriggerCalculatorByDeadline.Config(
          aggregationDeadline = aggregationDeadline,
          noL2ActivityTimeout = aggregationDeadlineNoL2ActivityTimeout,
        ),
        clock = clock,
        latestBlockProvider = safeBlockProvider,
      )
      AggregationTriggerCalculatorByDeadlineRunner(
        timerFactory = timerFactory,
        deadlineCheckInterval = aggregationDeadlineCheckInterval,
        aggregationTriggerByDeadline = aggregationCalculatorByDeadline,
      )
    }

    val aggregationCalculator = GlobalAggregationCalculator.create(
      lastAggregatedBlockNumber = lastAggregatedBlockNumber,
      lastAggregatedTimestamp = lastAggregatedTimestamp,
      aggregationTargetEndBlockNumbers = aggregationTargetEndBlockNumbers,
      timestampBasedHardForks = timestampBasedHardForks,
      aggregationProofsLimit = aggregationProofsLimit,
      aggregationBlobLimit = aggregationBlobLimit,
      aggregationSizeMultipleOf = aggregationSizeMultipleOf,
      aggregationDeadlineCalculator = aggregationDeadlineCalculator,
      metricsFacade = metricsFacade,
    )

    return AggregationCalculators(
      aggregationCalculator = aggregationCalculator,
      service = aggregationDeadlineCalculator ?: DisabledService("Aggregation Deadline"),
    )
  }

  private fun createConflationTriggerCalculators(
    tracesCountersLimit: TracesCounters,
    blocksLimit: UInt?,
    timestampBasedHardForks: List<Instant> = emptyList(),
    compressedBlobCalculator: ConflationTriggerCalculatorByDataCompressed,
    lastConflatedTimestamp: Instant,
    aggregationTargetEndBlockNumbers: MutableSet<ULong>,
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
        ConflationTriggerCalculatorByTargetBlockNumbers(targetEndBlockNumbers = aggregationTargetEndBlockNumbers),
        compressedBlobCalculator,
      )
    if (blocksLimit != null) {
      calculators.add(ConflationTriggerCalculatorByBlockLimit(blockLimit = blocksLimit))
    }
    if (timestampBasedHardForks.isNotEmpty()) {
      calculators.add(
        ConflationTriggerCalculatorByHardForkTimestamp(
          hardForkTimestamps = timestampBasedHardForks,
          initialTimestamp = lastConflatedTimestamp,
        ),
      )
    }
    return calculators
  }
}
