package linea.blob

import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager

class GoBackedBlobCompressor internal constructor(
  private val goNativeBlobCompressor: GoNativeBlobCompressor,
  override val version: BlobCompressorVersion,
) : BlobCompressor {
  private val log = LogManager.getLogger(GoBackedBlobCompressor::class.java)

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return goNativeBlobCompressor.CanWrite(blockRLPEncoded, blockRLPEncoded.size)
  }

  override fun appendBlock(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    val compressionSizeBefore = goNativeBlobCompressor.Len()
    val appended = goNativeBlobCompressor.Write(blockRLPEncoded, blockRLPEncoded.size)
    val compressedSizeAfter = goNativeBlobCompressor.Len()
    log.trace(
      "block compressed: blockRlpSize={} compressionDataBefore={} compressionDataAfter={} compressionRatio={}",
      blockRLPEncoded.size,
      compressionSizeBefore,
      compressedSizeAfter,
      1.0 - ((compressedSizeAfter - compressionSizeBefore).toDouble() / blockRLPEncoded.size),
    )
    val error = goNativeBlobCompressor.Error()
    if (error != null) {
      log.error("Failure while writing the following RLP encoded block: {}", blockRLPEncoded.encodeHex())
      throw BlobCompressionException(error)
    }
    return BlobCompressor.AppendResult(appended, compressionSizeBefore, compressedSizeAfter)
  }

  override fun startNewBatch() {
    goNativeBlobCompressor.StartNewBatch()
  }

  override fun getCompressedData(): ByteArray {
    val compressedData = ByteArray(goNativeBlobCompressor.Len())
    goNativeBlobCompressor.Bytes(compressedData)
    return compressedData
  }

  override fun reset() {
    goNativeBlobCompressor.Reset()
  }

  override fun compressedSize(data: ByteArray): Int {
    return goNativeBlobCompressor.RawCompressedSize(data, data.size)
  }

  override fun close() {}
}

class GoBackedBlobCompressorV4 internal constructor(
  private val goNativeBlobCompressor: GoNativeBlobCompressorV4,
  override val version: BlobCompressorVersion,
  private val handle: Int,
) : BlobCompressor {
  private val log = LogManager.getLogger(GoBackedBlobCompressorV4::class.java)

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return goNativeBlobCompressor.CanWrite(handle, blockRLPEncoded, blockRLPEncoded.size)
  }

  override fun appendBlock(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    val compressionSizeBefore = goNativeBlobCompressor.Len(handle)
    val appended = goNativeBlobCompressor.Write(handle, blockRLPEncoded, blockRLPEncoded.size)
    val compressedSizeAfter = goNativeBlobCompressor.Len(handle)
    log.trace(
      "block compressed: blockRlpSize={} compressionDataBefore={} compressionDataAfter={} compressionRatio={}",
      blockRLPEncoded.size,
      compressionSizeBefore,
      compressedSizeAfter,
      1.0 - ((compressedSizeAfter - compressionSizeBefore).toDouble() / blockRLPEncoded.size),
    )
    val error = goNativeBlobCompressor.Error(handle)
    if (error != null) {
      log.error("Failure while writing the following RLP encoded block: {}", blockRLPEncoded.encodeHex())
      throw BlobCompressionException(error)
    }
    return BlobCompressor.AppendResult(appended, compressionSizeBefore, compressedSizeAfter)
  }

  override fun startNewBatch() {
    goNativeBlobCompressor.StartNewBatch(handle)
  }

  override fun getCompressedData(): ByteArray {
    val compressedData = ByteArray(goNativeBlobCompressor.Len(handle))
    goNativeBlobCompressor.Bytes(handle, compressedData)
    return compressedData
  }

  override fun reset() {
    goNativeBlobCompressor.Reset(handle)
  }

  override fun compressedSize(data: ByteArray): Int {
    return goNativeBlobCompressor.RawCompressedSize(handle, data, data.size)
  }

  override fun close() {
    goNativeBlobCompressor.Free(handle)
  }
}
