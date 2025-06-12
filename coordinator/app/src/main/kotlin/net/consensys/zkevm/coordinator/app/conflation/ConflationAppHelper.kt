package net.consensys.zkevm.coordinator.app.conflation

import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture

object ConflationAppHelper {
  /**
   * Returns the last block number inclusive upto which we have consecutive proven blobs or the last finalized block
   * number inclusive
   */
  fun resumeConflationFrom(
    aggregationsRepository: AggregationsRepository,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return aggregationsRepository
      .findConsecutiveProvenBlobs(lastFinalizedBlock.toLong() + 1)
      .thenApply { blobAndBatchCounters ->
        if (blobAndBatchCounters.isNotEmpty()) {
          blobAndBatchCounters.last().blobCounters.endBlockNumber
        } else {
          lastFinalizedBlock
        }
      }
  }

  fun resumeAggregationFrom(
    aggregationsRepository: AggregationsRepository,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return aggregationsRepository
      .findHighestConsecutiveEndBlockNumber(lastFinalizedBlock.toLong() + 1)
      .thenApply { highestEndBlockNumber ->
        highestEndBlockNumber?.toULong() ?: lastFinalizedBlock
      }
  }

  fun cleanupDbDataAfterBlockNumbers(
    lastProcessedBlockNumber: ULong,
    lastConsecutiveAggregatedBlockNumber: ULong,
    batchesRepository: BatchesRepository,
    blobsRepository: BlobsRepository,
    aggregationsRepository: AggregationsRepository,
  ): SafeFuture<*> {
    val blockNumberInclusiveToDeleteFrom = lastProcessedBlockNumber + 1u
    val cleanupBatches = batchesRepository.deleteBatchesAfterBlockNumber(blockNumberInclusiveToDeleteFrom.toLong())
    val cleanupBlobs = blobsRepository.deleteBlobsAfterBlockNumber(blockNumberInclusiveToDeleteFrom)
    val cleanupAggregations = aggregationsRepository
      .deleteAggregationsAfterBlockNumber((lastConsecutiveAggregatedBlockNumber + 1u).toLong())

    return SafeFuture.allOf(cleanupBatches, cleanupBlobs, cleanupAggregations)
  }
}
