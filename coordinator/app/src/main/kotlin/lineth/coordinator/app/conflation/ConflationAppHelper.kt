package lineth.coordinator.app.conflation

import linea.domain.toBlockParameter
import linea.ethapi.EthApiClient
import lineth.coordinator.config.v2.ForcedTransactionsConfig
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.persistence.AggregationsRepository
import lineth.persistence.BatchesRepository
import lineth.persistence.BlobsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture

object ConflationAppHelper {
  /**
   * Forced transactions are a rollup-only feature in the coordinator today: ForcedTransactionsApp reads
   * the contract through the rollup client, which fails against a validium contract. They are therefore
   * disabled under VALIDIUM even when enabled in config. Both the ForcedTransactionsApp guard and the
   * forced-transactions DAO selection in CoordinatorApp go through this, so the two always agree.
   *
   * The validium V2 contract itself enforces forced-transaction inclusion at finalization, so storing
   * a forced transaction on such a deployment halts finalization until coordinator support is added.
   * Do not grant FORCED_TRANSACTION_SENDER_ROLE on a validium chain with this coordinator.
   */
  internal fun forcedTransactionsEnabled(
    forcedTransactions: ForcedTransactionsConfig?,
    dataAvailability: L1SubmissionConfig.DataAvailability,
  ): Boolean =
    forcedTransactions?.disabled == false &&
      dataAvailability != L1SubmissionConfig.DataAvailability.VALIDIUM

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
