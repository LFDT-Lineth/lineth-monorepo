package net.consensys.zkevm.coordinator.app.conflation

import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.isDisabled
import linea.ethapi.EthApiClient
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.blockcreation.FixedLaggingHeadSafeBlockProvider
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByBlockLimit
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByDataCompressed
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByExecutionTraces
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByTargetBlockNumbers
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByTimeDeadline
import net.consensys.zkevm.ethereum.coordination.conflation.DeadlineConflationCalculatorRunner
import net.consensys.zkevm.ethereum.coordination.conflation.TimestampHardForkConflationCalculator
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

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

  fun addBlocksLimitCalculatorIfDefined(
    configs: CoordinatorConfig,
    calculators: MutableList<ConflationCalculator>,
  ) {
    if (configs.conflation.blocksLimit != null) {
      calculators.add(
        ConflationCalculatorByBlockLimit(
          blockLimit = configs.conflation.blocksLimit,
        ),
      )
    }
  }

  fun addTargetEndBlockConflationCalculatorIfDefined(
    configs: CoordinatorConfig,
    calculators: MutableList<ConflationCalculator>,
  ) {
    if (configs.conflation.proofAggregation.targetEndBlocks?.isNotEmpty() ?: false) {
      calculators.add(
        ConflationCalculatorByTargetBlockNumbers(
          targetEndBlockNumbers = configs.conflation.proofAggregation.targetEndBlocks.toSet(),
        ),
      )
    }
  }

  fun addTimestampHardForkCalculatorIfDefined(
    configs: CoordinatorConfig,
    lastProcessedTimestamp: Instant,
    calculators: MutableList<ConflationCalculator>,
  ) {
    if (configs.conflation.proofAggregation.timestampBasedHardForks.isNotEmpty()) {
      calculators.add(
        TimestampHardForkConflationCalculator(
          hardForkTimestamps = configs.conflation.proofAggregation.timestampBasedHardForks,
          initialTimestamp = lastProcessedTimestamp,
        ),
      )
    }
  }

  fun createCalculatorsForBlobsAndConflation(
    configs: CoordinatorConfig,
    compressedBlobCalculator: ConflationCalculatorByDataCompressed,
    lastProcessedTimestamp: Instant,
    logger: Logger,
    metricsFacade: MetricsFacade,
  ): List<ConflationCalculator> {
    val calculators: MutableList<ConflationCalculator> =
      mutableListOf(
        ConflationCalculatorByExecutionTraces(
          tracesCountersLimit = configs.conflation.tracesLimits,
          emptyTracesCounters = configs.conflation.tracesLimits.emptyTracesCounters,
          metricsFacade = metricsFacade,
          log = logger,
        ),
        compressedBlobCalculator,
      )
    addBlocksLimitCalculatorIfDefined(configs = configs, calculators = calculators)
    addTargetEndBlockConflationCalculatorIfDefined(configs = configs, calculators = calculators)
    addTimestampHardForkCalculatorIfDefined(
      configs = configs,
      calculators = calculators,
      lastProcessedTimestamp = lastProcessedTimestamp,
    )
    return calculators
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
