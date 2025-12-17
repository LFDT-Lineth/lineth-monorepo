package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate

class AggregationTriggerCalculatorByBlobLimit(private val maxBlobsPerAggregation: UInt) :
  SyncAggregationTriggerCalculator {
  data class InFlightAggregation(
    val blobsCount: UInt,
    val blobsToAggregate: BlobsToAggregate,
  )

  private var inFlightAggregation: InFlightAggregation? = null

  private fun willReachBlobLimitWithOneMoreBlob(): Boolean {
    val blobCount = (inFlightAggregation?.blobsCount ?: 0u) + 1u
    return blobCount >= maxBlobsPerAggregation
  }

  @Synchronized
  override fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger? {
    return if (willReachBlobLimitWithOneMoreBlob()) {
      AggregationTrigger(
        aggregationTriggerType = AggregationTriggerType.BLOB_LIMIT,
        aggregation =
        BlobsToAggregate(
          inFlightAggregation?.blobsToAggregate?.startBlockNumber
            ?: blobCounters.startBlockNumber,
          blobCounters.endBlockNumber,
        ),
      )
    } else {
      null
    }
  }

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    val blobCount = (inFlightAggregation?.blobsCount ?: 0u) + 1u
    if (willReachBlobLimitWithOneMoreBlob()) {
      throw IllegalArgumentException(
        "Aggregation blob limit overflow: maxBlobsPerAggregation=$maxBlobsPerAggregation " +
          "blobsCount=$blobCount " +
          "blob=${blobCounters.intervalString()}",
      )
    }
    inFlightAggregation =
      InFlightAggregation(
        blobsCount = blobCount,
        blobsToAggregate =
        BlobsToAggregate(
          inFlightAggregation?.blobsToAggregate?.startBlockNumber
            ?: blobCounters.startBlockNumber,
          blobCounters.endBlockNumber,
        ),
      )
  }

  @Synchronized
  override fun reset() {
    inFlightAggregation = null
  }
}
