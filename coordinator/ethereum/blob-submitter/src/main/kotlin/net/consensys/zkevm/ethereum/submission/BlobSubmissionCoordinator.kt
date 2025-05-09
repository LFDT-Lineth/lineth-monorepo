package net.consensys.zkevm.ethereum.submission

import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import linea.domain.filterOutWithEndBlockNumberBefore
import linea.domain.toBlockIntervals
import linea.domain.toBlockIntervalsString
import linea.kotlin.trimToMinutePrecision
import net.consensys.linea.async.AsyncFilter
import net.consensys.zkevm.PeriodicPollingService
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlobSubmittedEvent
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BlobsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import kotlin.time.Duration

class BlobSubmissionCoordinator(
  private val config: Config,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
  private val lineaRollup: LineaRollupSmartContractClient,
  private val blobSubmitter: BlobSubmitter,
  private val vertx: Vertx,
  private val clock: Clock,
  private val blobSubmissionFilter: AsyncFilter<BlobRecord>,
  private val blobsGrouperForSubmission: BlobsGrouperForSubmission,
  private val log: Logger = LogManager.getLogger(BlobSubmissionCoordinator::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log
) {
  class Config(
    val pollingInterval: Duration,
    val proofSubmissionDelay: Duration,
    val maxBlobsToSubmitPerTick: UInt,
    val targetBlobsToSubmitPerTx: UInt = 9u
  ) {
    init {
      require(maxBlobsToSubmitPerTick > 0u) {
        "maxBlobsToSubmitPerTick=$maxBlobsToSubmitPerTick must be greater than 0"
      }
      require(targetBlobsToSubmitPerTx > 0u) {
        "targetBlobsToSubmitPerTx=$maxBlobsToSubmitPerTick must be greater than 0"
      }
      require(maxBlobsToSubmitPerTick > targetBlobsToSubmitPerTx) {
        "maxBlobsToSubmitPerTick=$maxBlobsToSubmitPerTick must be greater or equal to" +
          " targetBlobsToSubmitPerTx=$targetBlobsToSubmitPerTx"
      }
    }
  }

  override fun action(): SafeFuture<Unit> {
    return lineaRollup.updateNonceAndReferenceBlockToLastL1Block()
      .thenCompose { l1Reference ->
        lineaRollup.finalizedL2BlockNumber()
          .thenPeek { finalizedL2BlockNumber ->
            log.debug(
              "tick: l1BlockNumber={} nonce={} lastFinalizedBlockNumber={}",
              l1Reference.blockNumber,
              l1Reference.nonce,
              finalizedL2BlockNumber
            )
          }
      }
      .thenCompose(::getBlobChunksToSubmit)
      .thenCompose { blobChunksToSubmit ->
        if (blobChunksToSubmit.isEmpty()) {
          SafeFuture.completedFuture(Unit)
        } else {
          submitBlobsAfterEthCall(blobChunksToSubmit)
        }
      }
  }

  private fun getBlobChunksToSubmit(lastFinalizedBlockNumber: ULong): SafeFuture<List<List<BlobRecord>>> {
    val endBlockCreatedBefore = clock.now().minus(config.proofSubmissionDelay).trimToMinutePrecision()

    return blobsRepository.getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive = lastFinalizedBlockNumber.inc().toLong(),
      endBlockCreatedBefore = endBlockCreatedBefore
    ).thenCompose { blobRecords ->
      blobSubmissionFilter.invoke(blobRecords)
        .thenApply { blobRecordsToSubmit ->
          blobRecordsToSubmit.take(config.maxBlobsToSubmitPerTick.toInt())
            .also {
              if (it.isEmpty()) {
                log.debug(
                  "skipping blob submission: lastFinalizedBlockNumber={} cutOffTime={} totalBlobs={}",
                  lastFinalizedBlockNumber,
                  endBlockCreatedBefore,
                  blobRecords.size
                )
              } else {
                log.info(
                  "blobs to submit: lastFinalizedBlockNumber={} totalBlobs={} maxBlobsToSubmitPerTick={} " +
                    "cutOffTime={} newBlobsToSubmit={} alreadySubmittedBlobs={}",
                  lastFinalizedBlockNumber,
                  blobRecords.size,
                  config.maxBlobsToSubmitPerTick,
                  endBlockCreatedBefore,
                  it.toBlockIntervalsString(),
                  (blobRecords - blobRecordsToSubmit.toSet()).toBlockIntervalsString()
                )
              }
            }
        }
        .thenCompose { blobRecordsToSubmitCappedToLimit ->
          if (blobRecordsToSubmitCappedToLimit.isEmpty()) {
            SafeFuture.completedFuture(emptyList())
          } else {
            aggregationsRepository.getProofsToFinalize(
              fromBlockNumber = lastFinalizedBlockNumber.inc().toLong(),
              finalEndBlockCreatedBefore = clock.now().minus(config.proofSubmissionDelay),
              maximumNumberOfProofs = config.maxBlobsToSubmitPerTick.toInt()
            ).thenApply { proofsToFinalize ->
              if (
                proofsToFinalize.isEmpty() ||
                !blobBelongsToAnyAggregation(blobRecordsToSubmitCappedToLimit.first(), proofsToFinalize)
              ) {
                // wait for the next tick. We need to know when is next aggregation so the tx
                // does include blobs beyond it
                emptyList()
              } else {
                val aggregations = proofsToFinalize
                  .filterOutWithEndBlockNumberBefore(
                    endBlockNumberInclusive = blobRecordsToSubmitCappedToLimit.first().startBlockNumber - 1UL
                  )
                  .toBlockIntervals()
                log.debug(
                  "chunking blobs: lastFinalizedBlockNumber={} blobs={} aggregations={} allAggregations={}",
                  lastFinalizedBlockNumber,
                  blobRecordsToSubmitCappedToLimit.toBlockIntervalsString(),
                  aggregations.toIntervalList().toBlockIntervalsString(),
                  proofsToFinalize.toBlockIntervalsString()
                )

                blobsGrouperForSubmission.chunkBlobs(
                  blobsIntervals = blobRecordsToSubmitCappedToLimit,
                  aggregations = aggregations
                )
              }
            }
          }
        }
    }
  }

  private fun blobBelongsToAnyAggregation(
    blobRecord: BlobRecord,
    proofsToFinalize: List<ProofToFinalize>
  ): Boolean {
    return proofsToFinalize
      .any { blobRecord.startBlockNumber in it.startBlockNumber..it.endBlockNumber }
  }

  private fun submitBlobsAfterEthCall(
    blobsChunks: List<List<BlobRecord>>
  ): SafeFuture<Unit> {
    return blobSubmitter
      .submitBlobCall(blobsChunks.first())
      .thenApply { true }
      .exceptionally { th ->
        logSubmissionError(
          log,
          blobsChunks.first().first().intervalString(),
          th,
          isEthCall = true
        )
        false
      }
      .thenCompose { ethCallSucceeded ->
        // this is to avoid doubling up the error. It was already logged in the previous step
        if (ethCallSucceeded) {
          blobSubmitter.submitBlobs(blobsChunks)
        } else {
          SafeFuture.completedFuture(emptyList())
        }
      }.thenApply { Unit }
  }

  override fun handleError(error: Throwable) {
    logUnhandledError(log = log, errorOrigin = "blob submission", error = error)
  }

  companion object {
    fun create(
      config: Config,
      blobsRepository: BlobsRepository,
      aggregationsRepository: AggregationsRepository,
      lineaSmartContractClient: LineaRollupSmartContractClient,
      gasPriceCapProvider: GasPriceCapProvider?,
      alreadySubmittedBlobsFilter: AsyncFilter<BlobRecord>,
      blobSubmittedEventDispatcher: Consumer<BlobSubmittedEvent>,
      vertx: Vertx,
      clock: Clock
    ): BlobSubmissionCoordinator {
      val blobsGrouperForSubmission: BlobsGrouperForSubmission = BlobsGrouperForSubmissionSwitcherByTargetBock(
        eip4844TargetBlobsPerTx = config.targetBlobsToSubmitPerTx
      )
      val blobSubmitter = BlobSubmitterAsEIP4844MultipleBlobsPerTx(
        contract = lineaSmartContractClient,
        gasPriceCapProvider = gasPriceCapProvider,
        blobSubmittedEventConsumer = blobSubmittedEventDispatcher,
        clock = clock
      )

      return BlobSubmissionCoordinator(
        config = config,
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        lineaRollup = lineaSmartContractClient,
        blobSubmitter = blobSubmitter,
        vertx = vertx,
        clock = clock,
        blobSubmissionFilter = alreadySubmittedBlobsFilter,
        blobsGrouperForSubmission = blobsGrouperForSubmission
      )
    }
  }
}
