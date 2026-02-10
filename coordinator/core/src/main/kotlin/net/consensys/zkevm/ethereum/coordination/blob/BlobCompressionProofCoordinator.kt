package net.consensys.zkevm.ethereum.coordination.blob

import io.vertx.core.Handler
import io.vertx.core.Vertx
import linea.LongRunningService
import linea.domain.BlockIntervals
import linea.domain.toBlockIntervalsString
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.clients.BlobCompressionProofRequest
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClientV2
import net.consensys.zkevm.domain.Blob
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.ethereum.coordination.conflation.BlobCreationHandler
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.LinkedBlockingDeque
import kotlin.time.Duration
import kotlin.time.Instant

class BlobCompressionProofCoordinator(
  private val vertx: Vertx,
  private val blobCompressionProverClient: BlobCompressionProverClientV2,
  private val rollingBlobShnarfCalculator: RollingBlobShnarfCalculator,
  private val blobZkStateProvider: BlobZkStateProvider,
  private val config: Config,
  private val blobCompressionProofHandler: BlobCompressionProofHandler,
  private val blobCompressionProofRequestHandler: BlobCompressionProofRequestHandler? = null,
  private val log: Logger = LogManager.getLogger(BlobCompressionProofCoordinator::class.java),
  metricsFacade: MetricsFacade,
) : BlobCreationHandler, LongRunningService {
  private val defaultQueueCapacity = 1000 // Should be more than blob submission limit
  private val blobsToHandle = LinkedBlockingDeque<Blob>(defaultQueueCapacity)
  private var timerId: Long? = null
  private lateinit var blobPollingAction: Handler<Long>
  private val blobCompressionProofPoller = BlobCompressionProofPoller(
    blobCompressionProverClient = blobCompressionProverClient,
    blobCompressionProofHandler = blobCompressionProofHandler,
    vertx = vertx,
    log = log,
    metricsFacade = metricsFacade,
  )

  private val blobsCounter =
    metricsFacade.createCounter(
      category = LineaMetricsCategory.BLOB,
      name = "counter",
      description = "New blobs arriving to blob compression proof coordinator",
    )
  private val blobSizeInBlocksHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.BLOB,
      name = "blocks.size",
      description = "Number of blocks in each blob",
    )
  private val blobSizeInBatchesHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.BLOB,
      name = "batches.size",
      description = "Number of batches in each blob",
    )

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "compression.queue.size",
      description = "Size of blob compression proving queue",
      measurementSupplier = { blobsToHandle.size },
    )
  }

  data class Config(
    val pollingInterval: Duration,
  )

  @Synchronized
  private fun sendBlobToCompressionProver(blob: Blob): SafeFuture<Unit> {
    log.debug("Preparing compression proof request for blob={}", blob.intervalString())

    val blobZkSateAndRollingShnarfFuture =
      blobZkStateProvider
        .getBlobZKState(blob.blocksRange)
        .thenCompose { blobZkState ->
          rollingBlobShnarfCalculator.calculateShnarf(
            compressedData = blob.compressedData,
            parentStateRootHash = blobZkState.parentStateRootHash,
            finalStateRootHash = blobZkState.finalStateRootHash,
            conflationOrder =
            BlockIntervals(
              startingBlockNumber = blob.conflations.first().startBlockNumber,
              upperBoundaries = blob.conflations.map { it.endBlockNumber },
            ),
          ).thenApply { rollingBlobShnarfResult ->
            Pair(blobZkState, rollingBlobShnarfResult)
          }
        }

    return blobZkSateAndRollingShnarfFuture.thenCompose { (blobZkState, rollingBlobShnarfResult) ->
      createBlobCompressionProofRequest(
        compressedData = blob.compressedData,
        conflations = blob.conflations,
        parentStateRootHash = blobZkState.parentStateRootHash,
        finalStateRootHash = blobZkState.finalStateRootHash,
        parentDataHash = rollingBlobShnarfResult.parentBlobHash,
        prevShnarf = rollingBlobShnarfResult.parentBlobShnarf,
        expectedShnarfResult = rollingBlobShnarfResult.shnarfResult,
        commitment = rollingBlobShnarfResult.shnarfResult.commitment,
        kzgProofContract = rollingBlobShnarfResult.shnarfResult.kzgProofContract,
        kzgProofSideCar = rollingBlobShnarfResult.shnarfResult.kzgProofSideCar,
        blobStartBlockTime = blob.startBlockTime,
        blobEndBlockTime = blob.endBlockTime,
      ).whenException { exception ->
        log.error(
          "Error in requesting blob compression proof: blob={} errorMessage={} ",
          blob.intervalString(),
          exception.message,
          exception,
        )
      }
    }
  }

  private fun createBlobCompressionProofRequest(
    compressedData: ByteArray,
    conflations: List<ConflationCalculationResult>,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    parentDataHash: ByteArray,
    prevShnarf: ByteArray,
    expectedShnarfResult: ShnarfResult,
    commitment: ByteArray,
    kzgProofContract: ByteArray,
    kzgProofSideCar: ByteArray,
    blobStartBlockTime: Instant,
    blobEndBlockTime: Instant,
  ): SafeFuture<Unit> {
    val proofRequest =
      BlobCompressionProofRequest(
        compressedData = compressedData,
        conflations = conflations,
        parentStateRootHash = parentStateRootHash,
        finalStateRootHash = finalStateRootHash,
        parentDataHash = parentDataHash,
        prevShnarf = prevShnarf,
        expectedShnarfResult = expectedShnarfResult,
        commitment = commitment,
        kzgProofContract = kzgProofContract,
        kzgProofSideCar = kzgProofSideCar,
      )
    return blobCompressionProverClient.createProofRequest(proofRequest)
      .thenApply { proofIndex ->
        val unProvenBlobRecord =
          BlobRecord(
            startBlockNumber = conflations.first().startBlockNumber,
            endBlockNumber = conflations.last().endBlockNumber,
            blobHash = expectedShnarfResult.dataHash,
            startBlockTime = blobStartBlockTime,
            endBlockTime = blobEndBlockTime,
            batchesCount = conflations.size.toUInt(),
            expectedShnarf = expectedShnarfResult.expectedShnarf,
          )
        try {
          blobCompressionProofRequestHandler
            ?.acceptNewBlobCompressionProofRequest(proofIndex = proofIndex, unProvenBlobRecord = unProvenBlobRecord)
        } finally {
          blobCompressionProofPoller
            .addProofRequestsInProgressForPolling(proofIndex = proofIndex, unProvenBlobRecord = unProvenBlobRecord)
        }
      }
  }

  @Synchronized
  override fun handleBlob(blob: Blob): SafeFuture<*> {
    blobsCounter.increment()
    log.debug(
      "new blob: blob={} queuedBlobsToProve={} blobBatches={}",
      blob.intervalString(),
      blobsToHandle.size,
      blob.conflations.toBlockIntervalsString(),
    )
    blobSizeInBlocksHistogram.record(blob.blocksRange.count().toDouble())
    blobSizeInBatchesHistogram.record(blob.conflations.size.toDouble())
    blobsToHandle.put(blob)
    log.trace("Blob was added to the handling queue {}", blob)
    return SafeFuture.completedFuture(Unit)
  }

  override fun start(): SafeFuture<Unit> {
    if (timerId == null) {
      blobPollingAction =
        Handler<Long> {
          handleBlobFromTheQueue().whenComplete { _, _ ->
            timerId = vertx.setTimer(config.pollingInterval.inWholeMilliseconds, blobPollingAction)
          }
        }
      timerId = vertx.setTimer(config.pollingInterval.inWholeMilliseconds, blobPollingAction)
    }
    return blobCompressionProofPoller.start()
  }

  private fun handleBlobFromTheQueue(): SafeFuture<Unit> {
    return if (blobsToHandle.isNotEmpty()) {
      val blobToHandle = blobsToHandle.poll()
      sendBlobToCompressionProver(blobToHandle)
        .whenException { exception ->
          blobsToHandle.putFirst(blobToHandle)
          log.warn(
            "Error handling blob from BlobCompressionProofCoordinator queue: blob={} errorMessage={}",
            blobToHandle.intervalString(),
            exception.message,
            exception,
          )
        }
    } else {
      SafeFuture.completedFuture(Unit)
    }
  }

  override fun stop(): SafeFuture<Unit> {
    if (timerId != null) {
      vertx.cancelTimer(timerId!!)
      blobPollingAction = Handler<Long> {}
    }
    return blobCompressionProofPoller.stop()
  }
}
