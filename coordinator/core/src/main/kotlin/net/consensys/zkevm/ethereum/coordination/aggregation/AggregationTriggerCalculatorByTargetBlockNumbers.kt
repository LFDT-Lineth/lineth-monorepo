package net.consensys.zkevm.ethereum.coordination.aggregation

import linea.domain.BlockInterval
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class AggregationTriggerCalculatorByTargetBlockNumbers(
  targetEndBlockNumbers: List<ULong>,
  private val log: Logger = LogManager.getLogger(SyncAggregationTriggerCalculator::class.java)
) : SyncAggregationTriggerCalculator {
  private val endBlockNumbers = targetEndBlockNumbers.sorted()
  private var firstBlobWasConsumed: Boolean = false

  private var inFlightAggregation: BlobsToAggregate? = null

  internal fun <T : BlockInterval> checkAggregationTrigger(blob: T): AggregationTrigger? {
    if (endBlockNumbers.isEmpty()) {
      return null
    }

    if (blob.endBlockNumber > endBlockNumbers.last()) {
      if (!firstBlobWasConsumed) {
        firstBlobWasConsumed = true
        log.warn(
          "first blob={} is already beyond last target aggregation endBlockNumber={} " +
            "please check configuration",
          blob,
          endBlockNumbers.last()
        )
      }

      return null
    }

    val overlapedTargetAggregation = endBlockNumbers
      .firstOrNull { it >= blob.startBlockNumber && it < blob.endBlockNumber }

    return when {
      overlapedTargetAggregation != null -> {
        log.warn(
          "blob={} overlaps target aggregation with endBlockNumber={}",
          blob.intervalString(),
          overlapedTargetAggregation
        )
        null
      }

      endBlockNumbers.contains(blob.endBlockNumber) -> {
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.TARGET_BLOCK_NUMBER,
          aggregation = BlobsToAggregate(
            startBlockNumber = inFlightAggregation?.startBlockNumber ?: blob.startBlockNumber,
            endBlockNumber = blob.endBlockNumber
          )
        )
      }

      else -> {
        null
      }
    }
  }

  @Synchronized
  override fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger? =
    checkAggregationTrigger(blob = blobCounters)

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    inFlightAggregation = if (inFlightAggregation == null) {
      BlobsToAggregate(blobCounters.startBlockNumber, blobCounters.endBlockNumber)
    } else {
      BlobsToAggregate(inFlightAggregation!!.startBlockNumber, blobCounters.endBlockNumber)
    }
  }

  @Synchronized
  override fun reset() {
    inFlightAggregation = null
  }
}
