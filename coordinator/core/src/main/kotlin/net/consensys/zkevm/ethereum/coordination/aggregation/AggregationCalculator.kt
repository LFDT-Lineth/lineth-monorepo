package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate

fun interface AggregationHandler {
  fun onAggregation(blobsToAggregate: BlobsToAggregate)

  companion object {
    internal val NOOP_HANDLER: AggregationHandler = AggregationHandler { }
  }
}

enum class AggregationTriggerType {
  TIME_LIMIT,
  PROOF_LIMIT,
  BLOB_LIMIT,

  /**
   * Aggregation trigger by target block numbers specified in the configuration.
   * This is meant for either Development,  Testing or Match specific blobs sent to L1.
   */
  TARGET_BLOCK_NUMBER,

  /**
   * Aggregation trigger by hard fork events (timestamp-based or TTD-based).
   * When a hard fork is detected, any pending aggregation should be finalized.
   */
  HARD_FORK,
}

data class AggregationTrigger(
  val aggregationTriggerType: AggregationTriggerType,
  val aggregation: BlobsToAggregate,
)

fun interface AggregationTriggerHandler {
  fun onAggregationTrigger(aggregationTrigger: AggregationTrigger)

  companion object {
    internal val NOOP_HANDLER: AggregationTriggerHandler = AggregationTriggerHandler { }
  }
}

/**
 * The aggregation proof can be triggered by 2 conditions:
 *  - Number of batches - if number of all batches combined for all subsequent blobs above a given threshold;
 *  - Finalization deadline: the time elapsed between first block’s timestamp in the first submitted blob and
 *     current time is greater than a finalization deadline configured.
 *
 *   Sum(blob.numberOfBatches) < prover_batches_limit
 *   blob.firstBlockTimestamp < current_time - submission_deadline;
 */
interface AggregationCalculator {
  fun newBlob(blobCounters: BlobCounters)

  fun onAggregation(aggregationHandler: AggregationHandler)
}

interface AggregationTriggerCalculator {
  fun newBlob(blobCounters: BlobCounters)

  fun reset()
}

interface SyncAggregationTriggerCalculator : AggregationTriggerCalculator {
  fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger?
}

interface DeferredAggregationTriggerCalculator : AggregationTriggerCalculator {
  fun onAggregationTrigger(aggregationTriggerHandler: AggregationTriggerHandler)
}
