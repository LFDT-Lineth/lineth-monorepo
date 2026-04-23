package net.consensys.zkevm.coordinator.app.conflation

import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.isDisabled
import linea.ethapi.EthApiClient
import linea.persistence.AggregationsRepository
import linea.persistence.BatchesRepository
import linea.persistence.BlobsRepository
import net.consensys.zkevm.coordinator.blockcreation.FixedLaggingHeadSafeBlockProvider
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByTimeDeadline
import net.consensys.zkevm.ethereum.coordination.conflation.DeadlineConflationCalculatorRunner
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock

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
    val cleanupAggregations =
      aggregationsRepository
        .deleteAggregationsAfterBlockNumber((lastConsecutiveAggregatedBlockNumber + 1u).toLong())

    return SafeFuture.allOf(cleanupBatches, cleanupBlobs, cleanupAggregations)
  }

  fun createDeadlineConflationCalculatorRunner(
    configs: CoordinatorConfig,
    lastProcessedBlockNumber: ULong,
    l2EthClient: EthApiClient,
  ): DeadlineConflationCalculatorRunner? {
    if (configs.conflation.isDisabled() || configs.conflation.conflationDeadline == null) {
      return null
    }

    return DeadlineConflationCalculatorRunner(
      conflationDeadlineCheckInterval = configs.conflation.conflationDeadlineCheckInterval,
      delegate = ConflationCalculatorByTimeDeadline(
        config = ConflationCalculatorByTimeDeadline.Config(
          conflationDeadline = configs.conflation.conflationDeadline,
          conflationDeadlineLastBlockConfirmationDelay =
          configs.conflation.conflationDeadlineLastBlockConfirmationDelay,
        ),
        lastBlockNumber = lastProcessedBlockNumber,
        clock = Clock.System,
        latestBlockProvider = FixedLaggingHeadSafeBlockProvider(
          ethApiBlockClient = l2EthClient,
          blocksToFinalization = 0UL,
        ),
      ),
    )
  }
}
