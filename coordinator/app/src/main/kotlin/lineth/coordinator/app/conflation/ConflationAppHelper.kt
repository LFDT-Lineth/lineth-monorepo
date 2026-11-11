package lineth.coordinator.app.conflation

import linea.clients.L2ExecutionProverClientV1
import linea.clients.RollupAggregationProverClientV1
import linea.clients.RollupProverClientV1
import linea.domain.toBlockParameter
import linea.ethapi.EthApiClient
import lineth.coordinator.config.v2.ForcedTransactionsConfig
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.persistence.AggregationsRepository
import lineth.persistence.BatchesRepository
import lineth.persistence.BlobsRepository
import lineth.persistence.BlobsRepositoryV2
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

  /**
   * Where the RISC-V pipeline resumes after the cutover, all from the same last finalized block:
   * - conflation after the highest consecutive persisted batch from the last finalized block + 1, so already proven
   *   batches are kept;
   * - the rollup stage after the highest consecutive rollup row from the last finalized block + 1;
   * - aggregation at the last finalized block, as there is no RISC-V aggregation yet.
   * Each falls back to the last finalized block when nothing follows it.
   */
  fun getLastRiscvProcessedBlocks(
    lastFinalizedBlock: ULong,
    batchesRepository: BatchesRepository,
    blobsRepository: BlobsRepositoryV2,
    l2EthClient: EthApiClient,
  ): SafeFuture<RiscvLastProcessedBlocks> {
    val lastConflatedBlock = batchesRepository
      .findHighestConsecutiveEndBlockNumberFromBlockNumber(lastFinalizedBlock.toLong() + 1)
      .thenCompose { highestEnd ->
        l2EthClient.ethGetBlockByNumberTxHashes((highestEnd?.toULong() ?: lastFinalizedBlock).toBlockParameter())
      }
    val lastAggregatedBlock = l2EthClient.ethGetBlockByNumberTxHashes(lastFinalizedBlock.toBlockParameter())
    val lastRollupEndBlockNumber = resumeRiscvRollupFrom(blobsRepository, lastFinalizedBlock)
    return SafeFuture.collectAll(lastConflatedBlock, lastAggregatedBlock)
      .thenCombine(lastRollupEndBlockNumber) { blocks, lastRollupEnd ->
        RiscvLastProcessedBlocks(
          lastConflatedBlock = blocks.first(),
          lastAggregatedBlock = blocks.last(),
          lastRollupEndBlockNumber = lastRollupEnd,
        )
      }
  }

  /**
   * End block of the last consecutive RISC-V rollup row from the last finalized block + 1, or the last finalized block
   * if there is none.
   */
  internal fun resumeRiscvRollupFrom(
    blobsRepository: BlobsRepositoryV2,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return blobsRepository
      .findHighestConsecutiveEndBlockNumberFromBlockNumber(lastFinalizedBlock + 1UL)
      .thenApply { it ?: lastFinalizedBlock }
  }

  /**
   * Deletes what the RISC-V pipeline will rebuild: batches after [lastConflatedBlockNumber], rollup rows after
   * [lastRollupEndBlockNumber] and aggregations after [lastAggregatedBlockNumber].
   */
  fun cleanupRiscvDbDataAfterBlockNumbers(
    lastConflatedBlockNumber: ULong,
    lastRollupEndBlockNumber: ULong,
    lastAggregatedBlockNumber: ULong,
    batchesRepository: BatchesRepository,
    blobsRepository: BlobsRepositoryV2,
    aggregationsRepository: AggregationsRepository,
  ): SafeFuture<*> {
    if (lastRollupEndBlockNumber > lastConflatedBlockNumber) {
      return SafeFuture.failedFuture<Unit>(
        IllegalStateException(
          "last rollup ends at block $lastRollupEndBlockNumber, after the last persisted batch " +
            "$lastConflatedBlockNumber: rollups are only built from persisted batches",
        ),
      )
    }
    return SafeFuture.allOf(
      batchesRepository.deleteBatchesAfterBlockNumber((lastConflatedBlockNumber + 1UL).toLong()),
      blobsRepository.deleteBlobsAfterBlockNumber(lastRollupEndBlockNumber + 1UL),
      aggregationsRepository.deleteAggregationsAfterBlockNumber((lastAggregatedBlockNumber + 1UL).toLong()),
    )
  }

  /** Removes the RISC-V prover request files past each stage's resume point. */
  fun cleanupRiscvProverRequests(
    l2ExecutionProverClient: L2ExecutionProverClientV1,
    rollupProverClient: RollupProverClientV1,
    rollupAggregationProverClient: RollupAggregationProverClientV1,
    lastConflatedBlockNumber: ULong,
    lastRollupEndBlockNumber: ULong,
    lastAggregatedBlockNumber: ULong,
  ): SafeFuture<*> {
    return SafeFuture.allOf(
      l2ExecutionProverClient.removeRequests((lastConflatedBlockNumber + 1UL).toLong()),
      rollupProverClient.removeRequests((lastRollupEndBlockNumber + 1UL).toLong()),
      rollupAggregationProverClient.removeRequests((lastAggregatedBlockNumber + 1UL).toLong()),
    )
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
