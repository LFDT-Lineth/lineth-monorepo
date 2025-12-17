package net.consensys.zkevm.persistence

import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.ProofToFinalize
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface AggregationsRepository {
  /**
   * This method should:
   *    1. Get all consecutive blobs starting from `fromBlockNumber`, inclusive, but no more than a defined limit
   *    This logic will be similar to the existing `PostgresBatchesRepository.getConsecutiveBatchesFromBlockNumber`
   *    2. Join this information to the batches table and get all the execution proofs for the same block range.
   *    We also only want consecutive execution proofs here, same as for blobs. Consecutive takes prioritiy i.e:
   *    If there are compression proofs available in the blobs table for range [1, 100] and execution is proven for:
   *    [1, 50], [52, 101] (execution for 51 is not proven yet), then we want to return range [1, 50] for both
   *    compression and execution proofs
   *    3. Enrich blobs data with:
   *      * the number of conflated batches per blob
   *      * firstBlockTimestamp
   *      * lastBlockTimestamp
   *      * Not sure yet about blockMetaData, we'll revisit it later
   */
  fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>>

  fun saveNewAggregation(aggregation: Aggregation): SafeFuture<Unit>

  fun getProofsToFinalize(
    fromBlockNumber: Long,
    finalEndBlockCreatedBefore: Instant,
    maximumNumberOfProofs: Int,
  ): SafeFuture<List<ProofToFinalize>>

  fun findHighestConsecutiveEndBlockNumber(fromBlockNumber: Long? = null): SafeFuture<Long?>

  fun findAggregationProofByEndBlockNumber(endBlockNumber: Long): SafeFuture<ProofToFinalize?>

  fun deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int>

  fun deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int>
}
