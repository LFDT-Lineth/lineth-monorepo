package net.consensys.zkevm.ethereum.coordination.aggregation

import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.LongRunningService
import linea.contract.l2.L2MessageServiceSmartContractClientReadOnly
import linea.domain.BlockIntervals
import linea.domain.toBlockIntervalsString
import linea.ethapi.EthApiClient
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.domain.ProofsToAggregate
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import net.consensys.zkevm.persistence.AggregationsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.function.Consumer
import java.util.function.Supplier
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class ProofAggregationCoordinatorService(
  private val vertx: Vertx,
  private val config: Config,
  private val metricsFacade: MetricsFacade,
  private var nextBlockNumberToPoll: Long,
  private val aggregationCalculator: AggregationCalculator,
  private val aggregationsRepository: AggregationsRepository,
  private val consecutiveProvenBlobsProvider: ConsecutiveProvenBlobsProvider,
  private val proofAggregationClient: ProofAggregationProverClientV2,
  private val aggregationL2StateProvider: AggregationL2StateProvider,
  private val provenAggregationEndBlockNumberConsumer: Consumer<ULong> = Consumer<ULong> { },
  private val provenConsecutiveAggregationEndBlockNumberConsumer: Consumer<ULong> = Consumer<ULong> { },
  private val lastFinalizedBlockNumberSupplier: Supplier<ULong> = Supplier<ULong> { 0UL },
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
    val predicateToFilterCompressionBlobs: (BlobAndBatchCounters) -> Boolean = {
      it.blobCounters.startBlockNumber >= blobsToAggregate.startBlockNumber &&
        it.blobCounters.endBlockNumber <= blobsToAggregate.endBlockNumber
    }
    val compressionBlobs = mutableListOf<BlobAndBatchCounters>()
    while (pendingBlobs.isNotEmpty() && predicateToFilterCompressionBlobs(pendingBlobs.peek())) {
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

    val compressionProofIndexes =
      compressionBlobs.map {
        CompressionProofIndex(
          startBlockNumber = it.blobCounters.startBlockNumber,
          endBlockNumber = it.blobCounters.endBlockNumber,
          hash = it.blobCounters.expectedShnarf,
        )
      }

    val startingBlockNumber = compressionBlobs.first().executionProofs.startingBlockNumber
    val upperBoundaries =
      compressionBlobs.flatMap {
        it.executionProofs.upperBoundaries
      }
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
        )
      },
    ) {
      log.debug("requesting aggregation proof: aggregation={}", blobsToAggregate.intervalString())
      aggregationProofCreation(blockIntervals, compressionProofIndexes)
    }
      .thenPeek {
        log.info("aggregation proof generated: aggregation={}", blobsToAggregate.intervalString())
      }
      .thenCompose { aggregationProof ->
        val aggregation =
          Aggregation(
            startBlockNumber = blobsToAggregate.startBlockNumber,
            endBlockNumber = blobsToAggregate.endBlockNumber,
            batchCount = batchCount.toULong(),
            aggregationProof = aggregationProof,
          )
        aggregationsRepository
          .saveNewAggregation(aggregation = aggregation)
          .thenPeek {
            provenAggregationEndBlockNumberConsumer.accept(aggregation.endBlockNumber)
          }
          .whenException {
            log.error(
              "Error saving proven aggregation to DB: aggregation={} errorMessage={}",
              blobsToAggregate.intervalString(),
              it.message,
              it,
            )
          }
          .thenPeek {
            aggregationsRepository.findHighestConsecutiveEndBlockNumber(
              lastFinalizedBlockNumberSupplier.get().toLong() + 1L,
            )
              .thenApply { it ->
                if (it != null) {
                  provenConsecutiveAggregationEndBlockNumberConsumer.accept(it.toULong())
                }
              }
              .whenException {
                log.warn(
                  "Failed to get consecutive aggregation end block number from DB: aggregation={} errorMessage={}",
                  blobsToAggregate.intervalString(),
                  it.message,
                  it,
                )
              }
          }
      }
  }

  private fun aggregationProofCreation(
    batchIntervals: BlockIntervals,
    compressionProofIndexes: List<CompressionProofIndex>,
  ): SafeFuture<ProofToFinalize> {
    val blobsToAggregate = batchIntervals.toBlockInterval()
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
      .thenApply { rollingInfo ->
        ProofsToAggregate(
          compressionProofIndexes = compressionProofIndexes,
          executionProofs = batchIntervals,
          parentAggregationLastBlockTimestamp = rollingInfo.parentAggregationLastBlockTimestamp,
          parentAggregationLastL1RollingHashMessageNumber = rollingInfo.parentAggregationLastL1RollingHashMessageNumber,
          parentAggregationLastL1RollingHash = rollingInfo.parentAggregationLastL1RollingHash,
        )
      }
      .thenCompose(proofAggregationClient::requestProof)
      .whenException {
        log.debug(
          "Error getting aggregation proof: aggregation={} errorMessage={}",
          batchIntervals.toBlockInterval().intervalString(),
          it.message,
          it,
        )
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
      aggregationsRepository: AggregationsRepository,
      consecutiveProvenBlobsProvider: ConsecutiveProvenBlobsProvider,
      proofAggregationClient: ProofAggregationProverClientV2,
      l2EthApiClient: EthApiClient,
      l2MessageService: L2MessageServiceSmartContractClientReadOnly,
      noL2ActivityTimeout: Duration,
      waitForNoL2ActivityToTriggerAggregation: Boolean,
      targetEndBlockNumbers: List<ULong>,
      metricsFacade: MetricsFacade,
      provenAggregationEndBlockNumberConsumer: Consumer<ULong>,
      provenConsecutiveAggregationEndBlockNumberConsumer: Consumer<ULong>,
      lastFinalizedBlockNumberSupplier: Supplier<ULong>,
      aggregationSizeMultipleOf: UInt,
      hardForkTimestamps: List<Instant> = emptyList(),
      initialTimestamp: Instant,
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
      val syncAggregationTriggerCalculators = mutableListOf<SyncAggregationTriggerCalculator>()
      syncAggregationTriggerCalculators
        .add(AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = maxProofsPerAggregation))
      if (targetEndBlockNumbers.isNotEmpty()) {
        syncAggregationTriggerCalculators
          .add(AggregationTriggerCalculatorByTargetBlockNumbers(targetEndBlockNumbers = targetEndBlockNumbers))
      }
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
          aggregationsRepository = aggregationsRepository,
          consecutiveProvenBlobsProvider = consecutiveProvenBlobsProvider,
          proofAggregationClient = proofAggregationClient,
          aggregationL2StateProvider =
          AggregationL2StateProviderImpl(
            ethApiClient = l2EthApiClient,
            messageService = l2MessageService,
          ),
          provenAggregationEndBlockNumberConsumer = provenAggregationEndBlockNumberConsumer,
          provenConsecutiveAggregationEndBlockNumberConsumer = provenConsecutiveAggregationEndBlockNumberConsumer,
          lastFinalizedBlockNumberSupplier = lastFinalizedBlockNumberSupplier,
        )

      return LongRunningService.compose(deadlineCheckRunner, proofAggregationService)
    }
  }

  override fun handleError(error: Throwable) {
    log.error("Error polling blobs for aggregation: errorMessage={}", error.message, error)
  }
}
