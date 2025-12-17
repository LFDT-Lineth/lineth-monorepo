package net.consensys.zkevm.ethereum.coordination.conflation

import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressor
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class ConflationCalculatorByDataCompressed(
  private val blobCompressor: BlobCompressor,
  private val log: Logger = LogManager.getLogger(ConflationCalculatorByDataCompressed::class.java),
) : ConflationCalculator {
  override val id: String = ConflationTrigger.DATA_LIMIT.name
  internal var dataSizeUpToLastBatch: UInt = 0u
  internal var dataSize: UInt = 0u
  private var dataDrained = false
  private var lastValidatedBlock: ByteArray? = null

  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? {
    lastValidatedBlock = blockCounters.blockRLPEncoded
    val overflowResult =
      if (blobCompressor.canAppendBlock(blockCounters.blockRLPEncoded)) {
        null
      } else {
        // if single block cannot fill in blob, then it is oversized and we are in trouble!
        val blockOverSized = dataSize == 0u
        ConflationCalculator.OverflowTrigger(ConflationTrigger.DATA_LIMIT, blockOverSized)
      }
    log.trace(
      "checkOverflow: blockNumber={} blobSize={} dataSizeUpToLastBatch={} overflowResult={}",
      blockCounters.blockNumber,
      dataSize,
      dataSizeUpToLastBatch,
      overflowResult,
    )
    return overflowResult
  }

  override fun appendBlock(blockCounters: BlockCounters) {
    if (!blockCounters.blockRLPEncoded.contentEquals(lastValidatedBlock)) {
      // If this happens caller violates the interface contract, so this should never happen.
      // Just a safeguard to catch bugs on caller side because otherwise it can be a very nasty bug.
      throw IllegalStateException("Trying to append unvalidated block. Please call checkOverflow first.")
    }
    val appendResult = blobCompressor.appendBlock(blockCounters.blockRLPEncoded)
    val compressedDataSize = appendResult.compressedSizeAfter - appendResult.compressedSizeBefore
    val compressionRatio = 1.0 - compressedDataSize.toDouble().div(blockCounters.blockRLPEncoded.size)
    log.debug(
      "compression result: blockNumber={} blockRlpSize={} blobSizeBefore={} " +
        "blobSizeAfter={} blockCompressionRatio={}",
      blockCounters.blockNumber,
      blockCounters.blockRLPEncoded.size,
      appendResult.compressedSizeBefore,
      appendResult.compressedSizeAfter,
      compressionRatio,
    )

    if (!appendResult.blockAppended) {
      throw IllegalStateException("Trying to append a block that does not fit in the blob.")
    }
    dataSize = appendResult.compressedSizeAfter.toUInt()
    dataDrained = false
  }

  override fun reset() {
    if (dataDrained) {
      blobCompressor.reset()
      dataSizeUpToLastBatch = 0u
      dataSize = 0u
    }
  }

  override fun copyCountersTo(counters: ConflationCounters) {
    counters.dataSize = dataSize - dataSizeUpToLastBatch
  }

  fun startNewBatch() {
    dataSizeUpToLastBatch = dataSize
    blobCompressor.startNewBatch()
  }

  fun getCompressedDataSizeInCurrentBatch(): UInt {
    return dataSize - dataSizeUpToLastBatch
  }

  fun getCompressedData(): ByteArray {
    val data = blobCompressor.getCompressedData()
    dataDrained = true
    return data
  }
}
