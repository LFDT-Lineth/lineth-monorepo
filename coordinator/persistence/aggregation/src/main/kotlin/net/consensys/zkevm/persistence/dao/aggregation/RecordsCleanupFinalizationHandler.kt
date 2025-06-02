package net.consensys.zkevm.persistence.dao.aggregation

import net.consensys.zkevm.ethereum.finalization.FinalizationHandler
import net.consensys.zkevm.ethereum.finalization.FinalizationMonitor
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture

class RecordsCleanupFinalizationHandler(
  private val batchesRepository: BatchesRepository,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
) : FinalizationHandler {
  override fun handleUpdate(update: FinalizationMonitor.FinalizationUpdate): SafeFuture<*> {
    // We do not need to keep batches, blobs and aggregation objects in the DB that are not needed after finalization.
    // We only need to keep
    // - the last blob because BlobCompressionProofCoordinator needs the shnarf from the previous blob and
    // - the last aggregation because we do not want to delete the last aggregation.
    //
    // Consider the case when we have a finalization event for block 100.
    //
    // We could have the following objects in the DB
    // Batches : [90, 92], [93, 96], [97, 98], [99, 99], [100, 100]
    // Blobs: [87, 89], [90, 96], [97, 99], [100, 100]
    // Aggregations: [82, 89], [90, 100]
    //
    // When the finalization event for block 100 is received, we can cleanup
    // - Batches with endBlockNumber <= 100
    // - Blobs with endBlockNumber <= 100 - 1 so that the blob [100, 100] remains in DB
    // - Aggregation with endBlockNumber <= 100 - 1 so that the aggregation [90, 100] remains in DB
    //
    // If we subtract 2 instead of 1, ie cleanup all blobs and aggregation with endBlockNumber <= 100 - 2
    // we end up keeping
    // - Blobs [97, 99] and [100, 100]
    // - Aggregation [90, 100]
    //
    // Ideally subtracting 1 to clean up blobs and aggregation should work however subtracting 2 is more conservative
    // and can result in an extra blob (and possibly aggregation) object in the DB after cleanup
    // but does not impact any functionality.

    val batchesCleanup = batchesRepository.deleteBatchesUpToEndBlockNumber(update.blockNumber.toLong())
    val blobsCleanup = blobsRepository.deleteBlobsUpToEndBlockNumber(
      endBlockNumberInclusive = update.blockNumber - 1u,
    )
    val aggregationsCleanup = aggregationsRepository
      .deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive = update.blockNumber.toLong() - 1L)

    return SafeFuture.allOf(batchesCleanup, blobsCleanup, aggregationsCleanup)
  }
}
