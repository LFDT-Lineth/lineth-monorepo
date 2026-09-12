package linea.coordination.riscv.rollup

import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.runCatching
import io.vertx.core.Vertx
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProofResponseV1
import linea.clients.RollupProverClientV1
import linea.conflation.BlobCreationHandler
import linea.domain.Blob
import linea.domain.BlockIntervalProofIndex
import linea.domain.CommonDomainFunctions
import linea.metrics.LineaMetricsCategory
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedDeque
import kotlin.time.Duration

fun interface RollupRequestBuilder {
  fun build(blobs: List<Blob>): SafeFuture<RollupProofRequestV1>
}

fun interface RollupProofHandler {
  fun acceptNewRollupProof(proof: RollupProofResponseV1): SafeFuture<*>
}

class RollupProofGeneratingCoordinator(
  private val rollupProverClient: RollupProverClientV1,
  private val rollupRequestBuilder: RollupRequestBuilder,
  private val rollupProofHandler: RollupProofHandler,
  private val vertx: Vertx,
  private val config: Config,
  private val log: Logger = LogManager.getLogger(RollupProofGeneratingCoordinator::class.java),
  metricsFacade: MetricsFacade,
) : BlobCreationHandler,
  VertxPeriodicPollingService(
    vertx = vertx,
    name = "RollupProofPollingService",
    pollingIntervalMs = config.rollupProofPollingInterval.inWholeMilliseconds,
    log = log,
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {

  data class Config(
    val conflationAndProofGenerationRetryBackoffDelay: Duration,
    val rollupProofPollingInterval: Duration,
    val rollupProofPollsPerTick: Int = 200,
    /**
     * Number of blobs to accumulate before submitting a single rollup proof request (K in the
     * RISC-V conflation spec). K=1 preserves the current 1-blob-per-proof topology; K>1 amortises
     * the rollup proof's recursive verification overhead across multiple blobs.
     */
    val blobsPerRollupProof: Int = 1,
  ) {
    init {
      require(blobsPerRollupProof >= 1) { "blobsPerRollupProof must be >= 1, got $blobsPerRollupProof" }
    }
  }

  private val proofRequestsInProgress = ConcurrentLinkedDeque<BlockIntervalProofIndex>()
  private val pendingBlobs = mutableListOf<Blob>()

  private val blobsCounter =
    metricsFacade.createCounter(
      category = LineaMetricsCategory.BLOB,
      name = "counter",
      description = "New blobs arriving to rollup proof generating coordinator",
    )
  private val blobSizeInBlocksHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.BLOB,
      name = "blocks.size",
      description = "Number of blocks in each blob received by rollup proof coordinator",
    )
  private val blobSizeInBatchesHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.BLOB,
      name = "batches.size",
      description = "Number of batches in each blob received by rollup proof coordinator",
    )

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "prover.riscv.rollup.pendingproofs",
      description = "Number of rollup proof requests waiting for responses",
      measurementSupplier = { proofRequestsInProgress.size },
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "compression.queue.size",
      description = "Number of blobs accumulated and waiting to form a rollup proof request",
      measurementSupplier = { pendingBlobs.size },
    )
  }

  private fun pollProofIndex(proofIndex: BlockIntervalProofIndex): SafeFuture<*> {
    return rollupProverClient.findProofResponse(proofIndex).thenCompose { proofResponse ->
      if (proofResponse != null) {
        log.info("rollup proof generated: blocks={}", proofIndex.intervalString())
        rollupProofHandler.acceptNewRollupProof(proofResponse).thenApply {
          proofRequestsInProgress.remove(proofIndex)
        }
      } else {
        SafeFuture.completedFuture(Unit)
      }
    }
  }

  override fun action(): SafeFuture<*> {
    if (proofRequestsInProgress.isEmpty()) {
      return SafeFuture.completedFuture(Unit)
    }
    val iterator = proofRequestsInProgress.iterator()
    val proofIndicesToPoll = mutableListOf<BlockIntervalProofIndex>()
    while (iterator.hasNext() && proofIndicesToPoll.size < config.rollupProofPollsPerTick) {
      proofIndicesToPoll.add(iterator.next())
    }
    val proofsPollFutures = proofIndicesToPoll.map { pollProofIndex(it) }
    return SafeFuture.allOf(proofsPollFutures.stream())
  }

  @Synchronized
  override fun handleBlob(blob: Blob): SafeFuture<*> {
    blobsCounter.increment()
    blobSizeInBlocksHistogram.record(blob.blocksRange.count().toDouble())
    blobSizeInBatchesHistogram.record(blob.conflations.size.toDouble())
    pendingBlobs.add(blob)
    if (pendingBlobs.size < config.blobsPerRollupProof) {
      log.debug(
        "accumulating blob: blob={} pending={}/{}",
        blob.intervalString(),
        pendingBlobs.size,
        config.blobsPerRollupProof,
      )
      return SafeFuture.completedFuture(Unit)
    }
    val blobsToProcess = pendingBlobs.toList()
    pendingBlobs.clear()
    val blockIntervalString = CommonDomainFunctions.blockIntervalString(
      blobsToProcess.first().startBlockNumber,
      blobsToProcess.last().endBlockNumber,
    )
    return runCatching {
      log.info(
        "submitting rollup proof: blocks={} blobs={}",
        blockIntervalString,
        blobsToProcess.map { it.intervalString() },
      )
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.conflationAndProofGenerationRetryBackoffDelay,
        exceptionConsumer = {
          log.warn(
            "rollup proof creation flow failed blocks={} will retry in backOff={} errorMessage={}",
            blockIntervalString,
            config.conflationAndProofGenerationRetryBackoffDelay,
            it.message,
          )
        },
      ) {
        blobsToProofCreation(blobsToProcess)
      }
    }.getOrElse { error -> SafeFuture.failedFuture<Unit>(error) }
      .whenException { th ->
        log.error(
          "rollup proof request failed: blocks={} errorMessage={}",
          blockIntervalString,
          th.message,
          th,
        )
      }
  }

  private fun blobsToProofCreation(blobs: List<Blob>): SafeFuture<*> {
    val blockIntervalString = CommonDomainFunctions.blockIntervalString(
      blobs.first().startBlockNumber,
      blobs.last().endBlockNumber,
    )
    return rollupRequestBuilder.build(blobs)
      .whenException { th ->
        log.debug(
          "rollup request building failed: blocks={} errorMessage={}",
          blockIntervalString,
          th.message,
          th,
        )
      }
      .thenCompose { proofRequest ->
        rollupProverClient.createProofRequest(proofRequest)
          .thenCompose { proofIndex ->
            rollupProverClient.findProofResponse(proofIndex)
              .thenCompose<Unit> { existingResponse ->
                if (existingResponse != null) {
                  log.info(
                    "blocks={} already proven, skipping rollup proof tracking",
                    blockIntervalString,
                  )
                  rollupProofHandler.acceptNewRollupProof(existingResponse).thenApply { }
                } else {
                  log.info(
                    "rollup proof request generated: proofIndex={} blocks={}",
                    proofIndex,
                    blockIntervalString,
                  )
                  proofRequestsInProgress.addLast(proofIndex)
                  SafeFuture.completedFuture(Unit)
                }
              }
          }
          .whenException { th ->
            log.debug(
              "rollup proof failure: blocks={} errorMessage={}",
              blockIntervalString,
              th.message,
              th,
            )
          }
      }
  }
}
