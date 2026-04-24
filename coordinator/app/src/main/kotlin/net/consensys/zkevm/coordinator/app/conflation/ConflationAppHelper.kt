package net.consensys.zkevm.coordinator.app.conflation

import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.EthApiClient
import linea.persistence.AggregationsRepository
import linea.persistence.BatchesRepository
import linea.persistence.BlobsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture

object ConflationAppHelper {
  /**
   * Returns the last block number inclusive upto which we have consecutive proven blobs or the last finalized block
   * number inclusive
   */
  internal fun resumeConflationFrom(
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

  private fun resumeAggregationFrom(
    aggregationsRepository: AggregationsRepository,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return aggregationsRepository
      .findHighestConsecutiveEndBlockNumber(lastFinalizedBlock.toLong() + 1)
      .thenApply { highestEndBlockNumber ->
        highestEndBlockNumber?.toULong() ?: lastFinalizedBlock
      }
  }

  fun getLastConflatedAndAggregatedBlocks(
    lastFinalizedBlock: ULong,
    aggregationsRepository: AggregationsRepository,
    l2EthClient: EthApiClient,
  ): SafeFuture<LastProcessedBlocks> {
    val lastConflatedBlock = resumeConflationFrom(
      aggregationsRepository,
      lastFinalizedBlock,
    ).thenCompose { lastProcessedBlockNumber ->
      l2EthClient.ethGetBlockByNumberTxHashes(
        lastProcessedBlockNumber.toBlockParameter(),
      )
    }
    val lastAggregatedBlock = resumeAggregationFrom(
      aggregationsRepository,
      lastFinalizedBlock,
    ).thenCompose { lastConsecutiveAggregatedBlockNumber ->
      l2EthClient.ethGetBlockByNumberTxHashes(
        lastConsecutiveAggregatedBlockNumber.toBlockParameter(),
      )
    }

    return SafeFuture.collectAll(lastConflatedBlock, lastAggregatedBlock)
      .thenApply { blocks ->
        LastProcessedBlocks(lastConflatedBlock = blocks.first(), lastAggregatedBlock = blocks.last())
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
    val cleanupAggregations =
      aggregationsRepository
        .deleteAggregationsAfterBlockNumber((lastConsecutiveAggregatedBlockNumber + 1u).toLong())

    return SafeFuture.allOf(cleanupBatches, cleanupBlobs, cleanupAggregations)
  }
}
