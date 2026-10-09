package lineth.persistence

import linea.clients.RollupAggregationProofResponseV1
import linea.domain.BlobAndBatchCounters
import linea.domain.ProofToFinalize
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface AggregationsRepositoryG<T> : AggregationsDaoG<T>

interface AggregationsRepository : AggregationsRepositoryG<ProofToFinalize> {
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
}

interface AggregationsRepositoryV2 : AggregationsRepositoryG<RollupAggregationProofResponseV1>
