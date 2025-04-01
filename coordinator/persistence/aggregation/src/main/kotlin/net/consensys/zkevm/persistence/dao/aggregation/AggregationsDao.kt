package net.consensys.zkevm.persistence.dao.aggregation

import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.ProofToFinalize
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface AggregationsDao {
  /**
   * This method should:
   *    1. Get block number of the last aggregation in `PROVING` or `PROVEN` status. Let's call it
   *    "last_aggregated_block_number"
   *    2. Get all consecutive blobs starting from last_aggregated_block_number, but no more than a defined limit
   *    This logic will be similar to the existing `PostgresBatchesRepository.getConsecutiveBatchesFromBlockNumber`
   *    3. Join this information to the batches table and get all the execution proofs for the same block range.
   *    We also only want consecutive execution proofs here, same as for blobs. Consecutive takes priority i.e:
   *    If there are compression proofs available in the blobs table for range [1, 100] and execution is proven for:
   *    [1, 50], [52, 101] (execution for 51 is not proven yet), then we want to return range [1, 50] for both
   *    compression and execution proofs
   *    4. Enrich blobs data with:
   *      * the number of conflated batches per blob
   *      * firstBlockTimestamp
   *      * lastBlockTimestamp
   *      * Not sure yet about blockMetaData, we'll revisit it later
   */

  /**
   * Find all PROVEN consecutive blobs and makes sure all correspondent batches are proven as well.
   * If multiple batches/blobs with different versions are proven, selects the most recent version.
   */
  fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>>

  fun saveNewAggregation(aggregation: Aggregation): SafeFuture<Unit>

  fun getProofsToFinalize(
    fromBlockNumber: Long,
    finalEndBlockCreatedBefore: Instant,
    maximumNumberOfProofs: Int
  ): SafeFuture<List<ProofToFinalize>>

  fun findHighestConsecutiveEndBlockNumber(
    fromBlockNumber: Long
  ): SafeFuture<Long?>

  fun findAggregationProofByEndBlockNumber(endBlockNumber: Long): SafeFuture<ProofToFinalize?>

  fun deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int>

  fun deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int>
}
