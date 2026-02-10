package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Instant

/**
 * Aggregation trigger calculator that triggers on timestamp-based hard forks.
 * When a blob's timestamp crosses a hard fork timestamp boundary,
 * any pending aggregation should be finalized before the hard fork.
 */
class AggregationTriggerCalculatorByTimestampHardFork(
  private val hardForkTimestamps: List<Instant>,
  initialTimestamp: Instant,
  private val log: Logger = LogManager.getLogger(AggregationTriggerCalculatorByTimestampHardFork::class.java),
) : SyncAggregationTriggerCalculator {
  private var lastProcessedTimestamp: Instant = initialTimestamp
  private var inFlightAggregation: BlobsToAggregate? = null

  init {
    require(hardForkTimestamps.isNotEmpty()) {
      "Hard fork timestamps list cannot be empty"
    }
    require(hardForkTimestamps.sorted() == hardForkTimestamps) {
      "Hard fork timestamps must be sorted in ascending order"
    }
  }

  @Synchronized
  override fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger? {
    val blobEndTimestamp = blobCounters.endBlockTimestamp

    // Check if this blob crosses any hard fork timestamp boundary
    val applicableTimestamp =
      hardForkTimestamps.find { timestamp ->
        lastProcessedTimestamp < timestamp && blobEndTimestamp >= timestamp
      }

    if (applicableTimestamp != null && inFlightAggregation != null) {
      log.info(
        "Hard fork detected at blob ending at blockNumber={} with timestamp={}, forkTimestamp={}. " +
          "Triggering aggregation.",
        blobCounters.endBlockNumber,
        blobEndTimestamp,
        applicableTimestamp,
      )

      return AggregationTrigger(
        aggregationTriggerType = AggregationTriggerType.HARD_FORK,
        aggregation = inFlightAggregation!!,
      )
    }

    return null
  }

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    inFlightAggregation =
      BlobsToAggregate(
        inFlightAggregation?.startBlockNumber ?: blobCounters.startBlockNumber,
        blobCounters.endBlockNumber,
      )
    lastProcessedTimestamp = blobCounters.endBlockTimestamp
  }

  @Synchronized
  override fun reset() {
    inFlightAggregation = null
    // Note: We don't reset lastProcessedTimestamp as we need to track progress across aggregations
  }
}
