package linea.conflation.calculators

import linea.domain.BlobCounters
import linea.domain.BlobsToAggregate

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

  /**
   * Aggregation trigger for forced transaction invalidity proof generation.
   * When a forced transaction is detected, create an aggregation boundary at the FTX execution block
   * to isolate the FTX for invalidity proof generation.
   */
  FORCED_TRANSACTION,
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
 * The proof aggregation can be triggered by the following conditions:
 *  - proof limit: when (batches + blobs) reach a certain limit configured;
 *  - blob limit: when (blobs) reach a certain limit configured;
 *  - forced transaction was rejected: when an invalid (BadNonce) forced transaction is detected at block B, we need to
 *      make aggregation boundary at B-1, to include invalidity proof a beginning of next aggregation;
 *  - Finalization deadline: the time elapsed between first block’s timestamp in the first submitted blob and
 *     current time is greater than a finalization deadline configured.
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
