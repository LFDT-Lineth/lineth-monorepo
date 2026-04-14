package net.consensys.zkevm.ethereum.coordination.aggregation

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.domain.BlockIntervals
import linea.domain.toBlockIntervalsString
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.ProofsToAggregate
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class ProofAggregationCoordinatorService(
  private val vertx: Vertx,
  private val config: Config,
  private val metricsFacade: MetricsFacade,
  private var nextBlockNumberToPoll: Long,
  private val aggregationCalculator: AggregationCalculator,
  private val aggregationProofHandler: AggregationProofHandler,
  private val aggregationProofRequestHandler: AggregationProofRequestHandler? = null,
  private val invalidityProofProvider: InvalidityProofProvider,
  private val consecutiveProvenBlobsProvider: ConsecutiveProvenBlobsProvider,
  private val proofAggregationClient: ProofAggregationProverClientV2,
  private val aggregationL2StateProvider: AggregationL2StateProvider,
  private val log: Logger = LogManager.getLogger(ProofAggregationCoordinatorService::class.java),
) : AggregationHandler, VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log,
  name = "ProofAggregationCoordinatorService",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  data class Config(
    val pollingInterval: Duration,
    val proofsLimit: UInt,
    val proofGenerationRetryBackoffDelay: Duration,
  )

  internal val aggregationProofPoller: AggregationProofPoller = AggregationProofPoller(
    aggregationProofClient = proofAggregationClient,
    aggregationProofHandler = aggregationProofHandler,
    log = log,
    vertx = vertx,
  )

  private val pendingBlobs = ConcurrentLinkedQueue<BlobAndBatchCounters>()
  private val aggregationSizeInBlocksHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.AGGREGATION,
      name = "blocks.size",
      description = "Number of blocks in each aggregation",
    )
  private val aggregationSizeInBatchesHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.AGGREGATION,
      name = "batches.size",
      description = "Number of batches in each aggregation",
    )
  private val aggregationSizeInBlobsHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.AGGREGATION,
      name = "blobs.size",
      description = "Number of blobs in each aggregation",
    )

  init {
    aggregationCalculator.onAggregation(this)
  }

  /**
   * This component is responsible for periodically monitoring the blobs table to check how many blobs were PROVEN
   *  and aggregate all the corresponding batch proofs into a single final proof.
   *  The aggregation proof can be triggered by 2 conditions:
   *    Number of batches - if number of all batches combined for all subsequent blobs above a given threshold;
   *    Finalization deadline: the time elapsed between first block’s timestamp in the first submitted blob and
   *    current time is greater than a finalization deadline configured.
   *  This is delegated to AggregationCalculator
   * Note: do we need to take into account the 18 days blob eviction on L1?
   *    It should not be a problem in practice, assuming conflation of 20 blocks (over estimation),
   *    500 batches proving limit gives (20*500*12)/3600 ~ 33 hours max.
   *
   * High level steps:
   *   Poll blobs table for blobs, respecting
   *      Blob status is COMPRESSION_PROVEN and Join with batches table, PROVEN status
   *      Blob submission start block number order
   *   Send blob to aggregation calculator. Which will trigger aggregation based on:
   *      Sum(blob.numberOfBatches) < prover_batches_limit
   *      blob.firstBlockTimestamp < current_time - submission_deadline;
   *        (we don't wait to fill prover capacity of the submission_deadline is reached)
   *   When aggregation calculator fires an aggregation (onAggregation), do:
   *    Build aggregation proof request and send it to the prover;
   *    Insert a record in aggregation proof response table PROVING status
   *    Wait for the prover response and update record with PROVEN status;
   *
   *    Aggregations Table {
   *     start_block_number,
   *     end_block_number,
   *     status: PROVING, PROVEN
   *     // Meta/Redundant data
   *     start_block_timestamp // useful for submitter to implement submission by deadline
   *     batch_count
   *    }
   */

  @Synchronized
  override fun action(): SafeFuture<*> {
    log.debug("Polling blobs for aggregation calculator from block={}", nextBlockNumberToPoll)
    return consecutiveProvenBlobsProvider
      .findConsecutiveProvenBlobs(nextBlockNumberToPoll)
      .thenApply { blobs ->
        blobs.forEach {
          pendingBlobs.offer(it)
          aggregationCalculator.newBlob(it.blobCounters)
        }
        if (blobs.isNotEmpty()) {
          nextBlockNumberToPoll = blobs.last().blobCounters.endBlockNumber.toLong() + 1
          val numberOfBatches = blobs.sumOf { it.blobCounters.numberOfBatches }
          log.info(
            "new blobs sent to aggregation calculator: " +
              "nextBlockToPoll={} numberOfBlobs={} numberOfBatches={} total={} blobs={}",
            nextBlockNumberToPoll,
            blobs.size,
            numberOfBatches,
            blobs.size + numberOfBatches.toInt(),
            blobs.map(BlobAndBatchCounters::blobCounters).toBlockIntervalsString(),
          )
        } else {
          log.debug("Found no new blobs for aggregation. nextBlockToPoll={}", nextBlockNumberToPoll)
        }
      }
  }

  @Synchronized
  override fun onAggregation(blobsToAggregate: BlobsToAggregate) {
    log.debug("new aggregation={}", blobsToAggregate.intervalString())
    val compressionBlobs = mutableListOf<BlobAndBatchCounters>()
    while (pendingBlobs.isNotEmpty() && blobsToAggregate.contains(pendingBlobs.peek().blobCounters)) {
      compressionBlobs.add(pendingBlobs.poll())
    }
    assert(compressionBlobs.first().blobCounters.startBlockNumber == blobsToAggregate.startBlockNumber)
    assert(compressionBlobs.last().blobCounters.endBlockNumber == blobsToAggregate.endBlockNumber)

    val batchCount =
      compressionBlobs.sumOf { blobCounters ->
        blobCounters.blobCounters.numberOfBatches
      }

    aggregationSizeInBlocksHistogram.record(blobsToAggregate.blocksRange.count().toDouble())
    aggregationSizeInBatchesHistogram.record(batchCount.toDouble())
    aggregationSizeInBlobsHistogram.record(compressionBlobs.size.toDouble())

    val aggregationStartBlockTimestamp = compressionBlobs.first().blobCounters.startBlockTimestamp
    val compressionProofIndexes =
      compressionBlobs.map {
        CompressionProofIndex(
          startBlockNumber = it.blobCounters.startBlockNumber,
          endBlockNumber = it.blobCounters.endBlockNumber,
          hash = it.blobCounters.expectedShnarf,
          startBlockTimestamp = it.blobCounters.startBlockTimestamp,
        )
      }

    val startingBlockNumber = compressionBlobs.first().executionProofs.startingBlockNumber
    val upperBoundaries = compressionBlobs.flatMap { it.executionProofs.upperBoundaries }
    val blockIntervals = BlockIntervals(startingBlockNumber, upperBoundaries)

    AsyncRetryer.retry(
      vertx = vertx,
      backoffDelay = config.proofGenerationRetryBackoffDelay,
      exceptionConsumer = {
        // log failure as warning, but keeps on retrying...
        log.warn(
          "aggregation proof creation failed aggregation={} will retry in backOff={} errorMessage={}",
          blockIntervals.toBlockInterval().intervalString(),
          config.proofGenerationRetryBackoffDelay,
          it.message,
          it,
        )
      },
    ) {
      log.debug("creating aggregation proof request: aggregation={}", blobsToAggregate.intervalString())
      aggregationProofCreation(
        executionProofsIndexes = blockIntervals,
        compressionProofIndexes = compressionProofIndexes,
        aggregationStartBlockTimestamp = aggregationStartBlockTimestamp,
      )
    }
      .thenApply { aggregationProofIndex ->
        val unProvenAggregation =
          Aggregation(
            startBlockNumber = blobsToAggregate.startBlockNumber,
            endBlockNumber = blobsToAggregate.endBlockNumber,
            batchCount = batchCount.toULong(),
            aggregationProof = null,
          )
        try {
          aggregationProofRequestHandler?.acceptNewAggregationProofRequest(
            proofIndex = aggregationProofIndex,
            unProvenAggregation = unProvenAggregation,
          )
        } finally {
          aggregationProofPoller.addProofRequestsInProgressForPolling(
            aggregationProofIndex,
            unProvenAggregation,
          )
        }
      }
  }

  private fun aggregationProofCreation(
    executionProofsIndexes: BlockIntervals,
    compressionProofIndexes: List<CompressionProofIndex>,
    aggregationStartBlockTimestamp: Instant,
  ): SafeFuture<AggregationProofIndex> {
    val blobsToAggregate = executionProofsIndexes.toBlockInterval()
    return aggregationL2StateProvider
      .getAggregationL2State(blockNumber = blobsToAggregate.startBlockNumber.toLong() - 1)
      .whenException {
        log.debug(
          "failed to get parent aggregation l2 message rolling hash: aggregation={} errorMessage={}",
          blobsToAggregate.intervalString(),
          it.message,
          it,
        )
      }
      .thenCompose { rollingInfo ->
        invalidityProofProvider.getInvalidityProofs(
          ftxStartingNumber = rollingInfo.parentAggregationLastFtxNumber.inc(),
          aggregationStartingBlockNumber = blobsToAggregate.startBlockNumber,
        ).thenApply { invalidityProofIndexes ->
          ProofsToAggregate(
            compressionProofIndexes = compressionProofIndexes,
            executionProofs = executionProofsIndexes,
            invalidityProofs = invalidityProofIndexes,
            parentAggregationLastBlockTimestamp = rollingInfo.parentAggregationLastBlockTimestamp,
            parentAggregationLastL1RollingHashMessageNumber =
            rollingInfo.parentAggregationLastL1RollingHashMessageNumber,
            parentAggregationLastL1RollingHash = rollingInfo.parentAggregationLastL1RollingHash,
            parentAggregationLastFtxNumber = rollingInfo.parentAggregationLastFtxNumber,
            parentAggregationLastFtxRollingHash = rollingInfo.parentAggregationLastFtxRollingHash,
            startBlockTimestamp = aggregationStartBlockTimestamp,
          )
        }
          .thenCompose(proofAggregationClient::createProofRequest)
          .whenException {
            log.debug(
              "Error creating aggregation proof request: aggregation={} errorMessage={}",
              executionProofsIndexes.toBlockInterval().intervalString(),
              it.message,
              it,
            )
          }
      }
  }

  companion object {
    fun create(
      vertx: Vertx,
      aggregationCoordinatorPollingInterval: Duration,
      deadlineCheckInterval: Duration,
      aggregationDeadline: Duration,
      latestBlockProvider: SafeBlockProvider,
      maxProofsPerAggregation: UInt,
      maxBlobsPerAggregation: UInt?,
      startBlockNumberInclusive: ULong,
      aggregationProofHandler: AggregationProofHandler,
      aggregationProofRequestHandler: AggregationProofRequestHandler? = null,
      invalidityProofProvider: InvalidityProofProvider,
      aggregationL2StateProvider: AggregationL2StateProvider,
      consecutiveProvenBlobsProvider: ConsecutiveProvenBlobsProvider,
      proofAggregationClient: ProofAggregationProverClientV2,
      noL2ActivityTimeout: Duration,
      waitForNoL2ActivityToTriggerAggregation: Boolean,
      targetEndBlockNumbers: Set<ULong>,
      metricsFacade: MetricsFacade,
      aggregationSizeMultipleOf: UInt,
      hardForkTimestamps: List<Instant> = emptyList(),
      initialTimestamp: Instant,
      forcedTransactionTriggerAggCalculator: SyncAggregationTriggerCalculator,
    ): LongRunningService {
      val aggregationCalculatorByDeadline =
        AggregationTriggerCalculatorByDeadline(
          config =
          AggregationTriggerCalculatorByDeadline.Config(
            aggregationDeadline = aggregationDeadline,
            noL2ActivityTimeout = noL2ActivityTimeout,
            waitForNoL2ActivityToTriggerAggregation = waitForNoL2ActivityToTriggerAggregation,
          ),
          clock = Clock.System,
          latestBlockProvider = latestBlockProvider,
        )
      val syncAggregationTriggerCalculators = mutableListOf<SyncAggregationTriggerCalculator>(
        forcedTransactionTriggerAggCalculator,
        AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = maxProofsPerAggregation),
        AggregationTriggerCalculatorByTargetBlockNumbers(
          targetEndBlockNumbers = targetEndBlockNumbers,
        ),
      )
      if (maxBlobsPerAggregation != null) {
        syncAggregationTriggerCalculators
          .add(AggregationTriggerCalculatorByBlobLimit(maxBlobsPerAggregation = maxBlobsPerAggregation))
      }

      if (hardForkTimestamps.isNotEmpty()) {
        syncAggregationTriggerCalculators.add(
          AggregationTriggerCalculatorByTimestampHardFork(
            hardForkTimestamps = hardForkTimestamps,
            initialTimestamp = initialTimestamp,
          ),
        )
      }

      val globalAggregationCalculator =
        GlobalAggregationCalculator(
          lastBlockNumber = startBlockNumberInclusive - 1UL,
          syncAggregationTrigger = syncAggregationTriggerCalculators,
          deferredAggregationTrigger = listOf(aggregationCalculatorByDeadline),
          metricsFacade = metricsFacade,
          aggregationSizeMultipleOf = aggregationSizeMultipleOf,
        )

      val deadlineCheckRunner =
        AggregationTriggerCalculatorByDeadlineRunner(
          vertx = vertx,
          config =
          AggregationTriggerCalculatorByDeadlineRunner.Config(
            deadlineCheckInterval = deadlineCheckInterval,
          ),
          aggregationTriggerByDeadline = aggregationCalculatorByDeadline,
        )

      val proofAggregationService =
        ProofAggregationCoordinatorService(
          vertx = vertx,
          config =
          Config(
            pollingInterval = aggregationCoordinatorPollingInterval,
            proofsLimit = maxProofsPerAggregation,
            proofGenerationRetryBackoffDelay = 5.seconds,
          ),
          metricsFacade = metricsFacade,
          nextBlockNumberToPoll = startBlockNumberInclusive.toLong(),
          aggregationCalculator = globalAggregationCalculator,
          aggregationProofHandler = aggregationProofHandler,
          aggregationProofRequestHandler = aggregationProofRequestHandler,
          invalidityProofProvider = invalidityProofProvider,
          consecutiveProvenBlobsProvider = consecutiveProvenBlobsProvider,
          proofAggregationClient = proofAggregationClient,
          aggregationL2StateProvider = aggregationL2StateProvider,
        )

      return LongRunningService.compose(deadlineCheckRunner, proofAggregationService)
    }
  }

  override fun handleError(error: Throwable) {
    log.error("Error polling blobs for aggregation: errorMessage={}", error.message, error)
  }

  override fun start(): SafeFuture<Unit> {
    return aggregationProofPoller.start().thenCompose {
      super.start()
    }
  }

  override fun stop(): SafeFuture<Unit> {
    return super.stop().thenCompose {
      aggregationProofPoller.stop()
    }
  }
}
