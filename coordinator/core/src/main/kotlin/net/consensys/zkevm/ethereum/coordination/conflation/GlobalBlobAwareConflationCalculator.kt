package net.consensys.zkevm.ethereum.coordination.conflation

import linea.domain.BlockInterval
import linea.domain.toBlockIntervalsString
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.domain.Blob
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

internal val NOOP_BLOB_HANDLER: BlobCreationHandler = BlobCreationHandler { _: Blob ->
  SafeFuture.completedFuture(Unit)
}

/**
 * Blob calculator is special because it can contain multiple batches.
 * For this reason, it needs to be handled separately.
 */
class GlobalBlobAwareConflationCalculator(
  private val conflationCalculator: GlobalBlockConflationCalculator,
  private val blobCalculator: ConflationCalculatorByDataCompressed,
  private val batchesLimit: UInt,
  private val metricsFacade: MetricsFacade,
  private val log: Logger = LogManager.getLogger(GlobalBlobAwareConflationCalculator::class.java)
) : TracesConflationCalculator {
  private var conflationHandler: (ConflationCalculationResult) -> SafeFuture<*> = NOOP_CONSUMER
  private var blobHandler: BlobCreationHandler = NOOP_BLOB_HANDLER
  private var blobBatches = mutableListOf<ConflationCalculationResult>()
  private var blobBlockCounters = mutableListOf<BlockCounters>()
  private var numberOfBatches = 0U
  override val lastBlockNumber: ULong
    get() = conflationCalculator.lastBlockNumber

  private val gasUsedInBlobHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BLOB,
    name = "gas",
    description = "Total gas in each blob"
  )
  private val compressedDataSizeInBlobHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BLOB,
    name = "compressed.data.size",
    description = "Compressed L2 data size in bytes of each blob"
  )
  private val uncompressedDataSizeInBlobHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BLOB,
    name = "uncompressed.data.size",
    description = "Uncompressed L2 data size in bytes of each blob"
  )
  private val gasUsedInBatchHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BATCH,
    name = "gas",
    description = "Total gas in each batch"
  )
  private val compressedDataSizeInBatchHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BATCH,
    name = "compressed.data.size",
    description = "Compressed L2 data size in bytes of each batch"
  )
  private val uncompressedDataSizeInBatchHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BATCH,
    name = "uncompressed.data.size",
    description = "Uncompressed L2 data size in bytes of each batch"
  )
  private val avgCompressedTxDataSizeInBatchHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BATCH,
    name = "avg.compressed.tx.data.size",
    description = "Average compressed transaction data size in bytes of each batch"
  )
  private val avgUncompressedTxDataSizeInBatchHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BATCH,
    name = "avg.uncompressed.tx.data.size",
    description = "Average uncompressed transaction data size in bytes of each batch"
  )

  init {
    conflationCalculator.onConflatedBatch(this::handleBatchTrigger)
  }

  private fun recordBatchMetrics(conflation: ConflationCalculationResult) {
    runCatching {
      val filteredBlockCounters = blobBlockCounters
        .filter { conflation.blocksRange.contains(it.blockNumber) }
      val gasUsedInBatch = filteredBlockCounters.sumOf { it.gasUsed }
      val uncompressedDataSizeInBatch = filteredBlockCounters.sumOf { it.blockRLPEncoded.size }
      val numOfTransactionsInBatch = filteredBlockCounters.sumOf { it.numOfTransactions }
      val compressedDataSizeInBatch = blobCalculator.getCompressedDataSizeInCurrentBatch()
      gasUsedInBatchHistogram.record(gasUsedInBatch.toDouble())
      uncompressedDataSizeInBatchHistogram.record(uncompressedDataSizeInBatch.toDouble())
      compressedDataSizeInBatchHistogram.record(compressedDataSizeInBatch.toDouble())
      avgUncompressedTxDataSizeInBatchHistogram.record(
        if (numOfTransactionsInBatch > 0U) {
          uncompressedDataSizeInBatch.div(numOfTransactionsInBatch.toInt()).toDouble()
        } else {
          0.0
        }
      )
      avgCompressedTxDataSizeInBatchHistogram.record(
        if (numOfTransactionsInBatch > 0U) {
          compressedDataSizeInBatch.toInt().div(numOfTransactionsInBatch.toInt()).toDouble()
        } else {
          0.0
        }
      )
    }.onFailure {
      log.error("Error when recording batch metrics: errorMessage={}", it.message)
    }
  }

  private fun recordBlobMetrics(blobInterval: BlockInterval, blobCompressedDataSize: Int) {
    runCatching {
      val filteredBlockCounters = blobBlockCounters
        .filter { blobInterval.blocksRange.contains(it.blockNumber) }
      gasUsedInBlobHistogram.record(
        filteredBlockCounters.sumOf { it.gasUsed }.toDouble()
      )
      uncompressedDataSizeInBlobHistogram.record(
        filteredBlockCounters.sumOf { it.blockRLPEncoded.size }.toDouble()
      )
      compressedDataSizeInBlobHistogram.record(blobCompressedDataSize.toDouble())
    }.onFailure {
      log.error("Error when recording blob metrics: errorMessage={}", it.message)
    }
  }

  @Synchronized
  override fun newBlock(blockCounters: BlockCounters) {
    blobBlockCounters.add(blockCounters)
    conflationCalculator.newBlock(blockCounters)
  }

  @Synchronized
  fun handleBatchTrigger(conflation: ConflationCalculationResult): SafeFuture<*> {
    log.trace("handleBatchTrigger: numberOfBatches={} conflation={}", numberOfBatches, conflation)
    blobBatches.add(conflation)
    numberOfBatches += 1U

    // Record the batch metrics
    recordBatchMetrics(conflation)

    val future = conflationHandler.invoke(conflation)
    if (conflation.conflationTrigger == ConflationTrigger.DATA_LIMIT ||
      conflation.conflationTrigger == ConflationTrigger.TIME_LIMIT ||
      conflation.conflationTrigger == ConflationTrigger.TARGET_BLOCK_NUMBER ||
      numberOfBatches >= batchesLimit
    ) {
      fireBlobTriggerAndResetState(conflation.conflationTrigger)
    } else {
      blobCalculator.startNewBatch()
      if (blobCalculator.checkOverflow(blobBlockCounters.last()) != null) {
        // we need to close the blob and start a new one
        fireBlobTriggerAndResetState(conflation.conflationTrigger)
      }
    }
    return future
  }

  private fun fireBlobTriggerAndResetState(trigger: ConflationTrigger) {
    val compressedData = blobCalculator.getCompressedData()
    val blobInterval = BlockInterval(
      blobBatches.first().startBlockNumber,
      blobBatches.last().endBlockNumber
    )
    val blob = Blob(
      conflations = blobBatches,
      compressedData = compressedData,
      startBlockTime = blobBlockCounters
        .find { it.blockNumber == blobInterval.startBlockNumber }!!.blockTimestamp,
      endBlockTime = blobBlockCounters
        .find { it.blockNumber == blobInterval.endBlockNumber }!!.blockTimestamp
    )
    log.info(
      "new blob: blob={} trigger={} blobSizeBytes={} blobBatchesCount={} blobBatchesLimit={} blobBatchesList={}",
      blobInterval.intervalString(),
      trigger,
      compressedData.size,
      blobBatches.size,
      batchesLimit,
      blobBatches.toBlockIntervalsString()
    )
    blobHandler.handleBlob(blob)

    // Record the blob metrics
    recordBlobMetrics(blobInterval, compressedData.size)

    blobBatches = run {
      blobBatches.forEach { conflation ->
        blobBlockCounters.removeIf { conflation.blocksRange.contains(it.blockNumber) }
      }
      mutableListOf()
    }
    blobCalculator.reset()
    numberOfBatches = 0U
  }

  @Synchronized
  override fun onConflatedBatch(conflationConsumer: (ConflationCalculationResult) -> SafeFuture<*>) {
    this.conflationHandler = conflationConsumer
  }

  @Synchronized
  override fun onBlobCreation(blobHandler: BlobCreationHandler) {
    this.blobHandler = blobHandler
  }
}
