package net.consensys.zkevm.ethereum.finalization

import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import linea.kotlin.trimToMinutePrecision
import net.consensys.linea.async.AsyncFilter
import net.consensys.zkevm.PeriodicPollingService
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.ethereum.submission.logUnhandledError
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BlobsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class AggregationFinalizationCoordinator(
  private val config: Config,
  private val lineaRollup: LineaRollupSmartContractClient,
  private val aggregationsRepository: AggregationsRepository,
  private val blobsRepository: BlobsRepository,
  private val alreadySubmittedBlobsFilter: AsyncFilter<BlobRecord>,
  private val aggregationSubmitter: AggregationSubmitter,
  private val vertx: Vertx,
  private val clock: Clock,
  private val log: Logger = LogManager.getLogger(AggregationFinalizationCoordinator::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log
) {
  class Config(
    val pollingInterval: Duration,
    val proofSubmissionDelay: Duration
  )

  override fun action(): SafeFuture<Unit> {
    return lineaRollup.updateNonceAndReferenceBlockToLastL1Block()
      .thenComposeCombined(lineaRollup.finalizedL2BlockNumber()) { _, lastFinalizedBlock ->
        log.debug("fetching aggregation proofs for finalization: lastFinalizedBlock={}", lastFinalizedBlock)
        val endBlockCreatedBefore = clock.now().minus(config.proofSubmissionDelay).trimToMinutePrecision()
        fetchAggregationData(lastFinalizedBlock)
          .thenCompose { aggregationData ->
            if (aggregationData == null) {
              log.info(
                "No aggregation available to submit with endBlockCreatedBefore={}",
                endBlockCreatedBefore
              )
              SafeFuture.completedFuture(Unit)
            } else {
              alreadySubmittedBlobsFilter
                .invoke(listOf(aggregationData.aggregationEndBlob))
                .thenCompose { blobs ->
                  if (blobs.isNotEmpty()) {
                    log.debug(
                      "aggregation={} last blob={} not yet submitted on L1, waiting for it.",
                      aggregationData.aggregationProof.intervalString(),
                      aggregationData.aggregationEndBlob.intervalString()
                    )
                    SafeFuture.completedFuture(Unit)
                  } else {
                    finalizeAggregationAfterEthCall(aggregationData)
                  }
                }
            }
          }
      }
  }

  private fun fetchAggregationStartAndEndBlob(
    proofToFinalize: ProofToFinalize?
  ): SafeFuture<ProofEdgeBlobs?> {
    return if (proofToFinalize == null) {
      SafeFuture.completedFuture(null)
    } else {
      SafeFuture.collectAll(
        blobsRepository.findBlobByStartBlockNumber(proofToFinalize.startBlockNumber.toLong()),
        blobsRepository.findBlobByEndBlockNumber(proofToFinalize.endBlockNumber.toLong())
      )
        .thenApply { (startBlob, endBlob) ->
          when {
            startBlob == null ->
              throw IllegalStateException(
                "start blob of aggregation=${proofToFinalize.intervalString()} not found in the DB."
              )

            endBlob == null ->
              throw IllegalStateException(
                "end blob of aggregation=${proofToFinalize.intervalString()} not found in the DB."
              )

            else -> ProofEdgeBlobs(proofToFinalize, startBlob, endBlob)
          }
        }
    }
  }

  private fun fetchAggregationData(
    lastFinalizedBlockNumber: ULong
  ): SafeFuture<AggregationData?> {
    return aggregationsRepository.getProofsToFinalize(
      fromBlockNumber = lastFinalizedBlockNumber.toLong() + 1,
      finalEndBlockCreatedBefore = clock.now().minus(config.proofSubmissionDelay),
      maximumNumberOfProofs = 1
    )
      .thenApply { it.firstOrNull() }
      .thenCompose(::fetchAggregationStartAndEndBlob)
      .thenCompose { aggregationAndBlobs ->
        if (aggregationAndBlobs == null) {
          // no aggregation to finalize, skip
          SafeFuture.completedFuture(null)
        } else {
          val (aggregationProof, aggregationStartBlob, aggregationEndBlob) = aggregationAndBlobs
          if (lastFinalizedBlockNumber == 0.toULong()) {
            // if lastFinalizedBlockNumber = 0, this is the very fist aggregation after genesis
            // so no parent aggregation will be available
            SafeFuture.completedFuture(
              AggregationData.genesis(
                aggregationProof,
                aggregationEndBlob,
                aggregationStartBlob.blobCompressionProof!!.prevShnarf
              )
            )
          } else {
            aggregationsRepository
              .findAggregationProofByEndBlockNumber(lastFinalizedBlockNumber.toLong())
              .thenCompose { parentAggregationProof ->
                parentAggregationProof?.let {
                  SafeFuture.completedFuture(
                    AggregationData(
                      aggregationProof = aggregationProof,
                      aggregationEndBlob = aggregationEndBlob,
                      parentShnarf = aggregationStartBlob.blobCompressionProof!!.prevShnarf,
                      parentL1RollingHash = parentAggregationProof.l1RollingHash,
                      parentL1RollingHashMessageNumber = parentAggregationProof.l1RollingHashMessageNumber
                    )
                  )
                } ?: run {
                  log.info(
                    "parent aggregation not found for aggregation={} " +
                      "lastFinalizedBlockNumber={} skipping because another finalization tx was executed.",
                    aggregationProof.intervalString(),
                    parentAggregationProof
                  )
                  SafeFuture.completedFuture(null)
                }
              }
          }
        }
      }
  }

  private fun finalizeAggregationAfterEthCall(
    aggregationData: AggregationData
  ): SafeFuture<Unit> {
    return aggregationSubmitter.submitAggregationAfterEthCall(
      aggregationProof = aggregationData.aggregationProof,
      aggregationEndBlob = aggregationData.aggregationEndBlob,
      parentShnarf = aggregationData.parentShnarf,
      parentL1RollingHash = aggregationData.parentL1RollingHash,
      parentL1RollingHashMessageNumber = aggregationData.parentL1RollingHashMessageNumber
    ).thenApply { Unit }
  }

  override fun handleError(error: Throwable) {
    logUnhandledError(log = log, errorOrigin = "aggregation finalization", error = error)
  }

  private data class AggregationData(
    val aggregationProof: ProofToFinalize,
    val aggregationEndBlob: BlobRecord,
    val parentShnarf: ByteArray,
    val parentL1RollingHash: ByteArray,
    val parentL1RollingHashMessageNumber: Long
  ) {
    companion object {
      fun genesis(
        aggregationProof: ProofToFinalize,
        aggregationEndBlob: BlobRecord,
        parentShnarf: ByteArray
      ): AggregationData {
        return AggregationData(
          aggregationProof = aggregationProof,
          aggregationEndBlob = aggregationEndBlob,
          parentShnarf = parentShnarf,
          parentL1RollingHash = ByteArray(32),
          parentL1RollingHashMessageNumber = 0
        )
      }
    }
  }

  private data class ProofEdgeBlobs(
    val proof: ProofToFinalize,
    val startBlob: BlobRecord,
    val endBlob: BlobRecord
  )

  companion object {
    fun create(
      config: Config,
      aggregationsRepository: AggregationsRepository,
      blobsRepository: BlobsRepository,
      lineaRollup: LineaRollupSmartContractClient,
      alreadySubmittedBlobFilter: AsyncFilter<BlobRecord>,
      aggregationSubmitter: AggregationSubmitter,
      vertx: Vertx,
      clock: Clock
    ): AggregationFinalizationCoordinator {
      return AggregationFinalizationCoordinator(
        config = config,
        lineaRollup = lineaRollup,
        aggregationsRepository = aggregationsRepository,
        blobsRepository = blobsRepository,
        aggregationSubmitter = aggregationSubmitter,
        alreadySubmittedBlobsFilter = alreadySubmittedBlobFilter,
        vertx = vertx,
        clock = clock
      )
    }
  }
}
