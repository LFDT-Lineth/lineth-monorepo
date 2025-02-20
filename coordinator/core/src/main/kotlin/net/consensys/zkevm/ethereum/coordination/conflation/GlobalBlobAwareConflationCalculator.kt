package net.consensys.zkevm.ethereum.coordination.conflation

import linea.domain.CommonDomainFunctions.blockIntervalString
import linea.domain.toBlockIntervalsString
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
  private val log: Logger = LogManager.getLogger(GlobalBlobAwareConflationCalculator::class.java)
) : TracesConflationCalculator {
  private var conflationHandler: (ConflationCalculationResult) -> SafeFuture<*> = NOOP_CONSUMER
  private var blobHandler: BlobCreationHandler = NOOP_BLOB_HANDLER
  private var blobBatches = mutableListOf<ConflationCalculationResult>()
  private var blobBlockCounters = mutableListOf<BlockCounters>()
  private var numberOfBatches = 0U
  override val lastBlockNumber: ULong
    get() = conflationCalculator.lastBlockNumber

  init {
    conflationCalculator.onConflatedBatch(this::handleBatchTrigger)
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
    val blob = Blob(
      conflations = blobBatches,
      compressedData = compressedData,
      startBlockTime = blobBlockCounters
        .find { it.blockNumber == blobBatches.first().startBlockNumber }!!.blockTimestamp,
      endBlockTime = blobBlockCounters
        .find { it.blockNumber == blobBatches.last().endBlockNumber }!!.blockTimestamp
    )
    log.info(
      "new blob: blob={} trigger={} blobSizeBytes={} blobBatchesCount={} blobBatchesLimit={} blobBatchesList={}",
      blockIntervalString(blobBatches.first().startBlockNumber, blobBatches.last().endBlockNumber),
      trigger,
      compressedData.size,
      blobBatches.size,
      batchesLimit,
      blobBatches.toBlockIntervalsString()
    )
    blobHandler.handleBlob(blob)
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
