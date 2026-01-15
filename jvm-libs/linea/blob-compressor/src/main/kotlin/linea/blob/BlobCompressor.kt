package linea.blob

import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager

class BlobCompressionException(message: String) : RuntimeException(message)

interface BlobCompressor {

  /**
   * @Throws(BlobCompressionException::class) when blockRLPEncoded is invalid
   */
  fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean

  /**
   * @Throws(BlobCompressionException::class) when blockRLPEncoded is invalid
   */
  fun appendBlock(blockRLPEncoded: ByteArray): AppendResult

  fun startNewBatch()
  fun getCompressedData(): ByteArray
  fun reset()

  fun getCompressedDataAndReset(): ByteArray {
    val compressedData = getCompressedData()
    reset()
    return compressedData
  }

  data class AppendResult(
    // returns false if last chunk would go over dataLimit. Does  not append last block.
    val blockAppended: Boolean,
    val compressedSizeBefore: Int,
    // even when block is not appended, compressedSizeAfter should as if it was appended
    val compressedSizeAfter: Int,
  )

  fun compressedSize(data: ByteArray): Int
}

class GoBackedBlobCompressor private constructor(
  internal val goNativeBlobCompressor: GoNativeBlobCompressor,
) : BlobCompressor {

  companion object {
    @JvmStatic
    fun getInstance(compressorVersion: BlobCompressorVersion, dataLimit: Int): GoBackedBlobCompressor {
      require(dataLimit > 0) { "dataLimit=$dataLimit must be greater than 0" }

      val goNativeBlobCompressor = GoNativeBlobCompressorFactory.getInstance(compressorVersion)
      val initialized = goNativeBlobCompressor.Init(
        dataLimit = dataLimit,
        dictPath = GoNativeBlobCompressorFactory.dictionaryPath.toString(),
      )
      if (!initialized) {
        throw InstantiationException(goNativeBlobCompressor.Error())
      }
      return GoBackedBlobCompressor(goNativeBlobCompressor)
    }
  }

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
}
