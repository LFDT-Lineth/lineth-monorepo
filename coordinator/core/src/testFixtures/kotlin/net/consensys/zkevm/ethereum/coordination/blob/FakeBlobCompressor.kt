package net.consensys.zkevm.ethereum.coordination.blob

import linea.blob.BlobCompressor
import linea.blob.BlobCompressorVersion
import org.apache.logging.log4j.LogManager
import kotlin.random.Random

class FakeBlobCompressor(
  private val dataLimit: Int,
  // 1 means no compression
  // 0.5 means compressed data is 50% smaller than original
  // 1.1 means compressed data is 10% bigger than original
  private val fakeCompressionRatio: Double = 1.0,
) : BlobCompressor {
  val log = LogManager.getLogger(FakeBlobCompressor::class.java)

  override val version: BlobCompressorVersion = BlobCompressorVersion.V3

  init {
    require(dataLimit > 0) { "dataLimit must be greater than 0" }
    require(fakeCompressionRatio > 0.0) { "fakeCompressionRatio must be greater than 0" }
  }

  private var compressedData = byteArrayOf()

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return compressedData.size + blockRLPEncoded.size * fakeCompressionRatio <= dataLimit
  }

  override fun appendBlock(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    return buildAppendResult(blockRLPEncoded)
  }

  private fun buildAppendResult(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    val compressedBlock = Random.nextBytes((blockRLPEncoded.size * fakeCompressionRatio).toInt())
    val compressedSizeBefore = compressedData.size
    val compressedSizeAfter = compressedData.size + compressedBlock.size
    return if (compressedSizeAfter > dataLimit) {
      BlobCompressor.AppendResult(
        false,
        compressedSizeBefore,
        compressedSizeAfter,
      )
    } else {
      compressedData += compressedBlock
      BlobCompressor.AppendResult(true, compressedSizeBefore, compressedSizeAfter)
    }
  }

  override fun startNewBatch() {
    // we don't care.
  }

  override fun getCompressedData(): ByteArray {
    return compressedData
  }

  override fun reset() {
    log.trace("resetting to empty state")
    compressedData = byteArrayOf()
  }

  override fun compressedSize(data: ByteArray): Int {
    return (data.size * fakeCompressionRatio).toInt()
  }

  override fun close() {
    // Nothing to do
  }
}
