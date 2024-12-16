package linea.blob

import net.consensys.encodeHex
import net.consensys.linea.blob.BlobCompressorVersion
import net.consensys.linea.blob.GoNativeBlobCompressor
import net.consensys.linea.blob.GoNativeBlobCompressorFactory
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

  data class AppendResult(
    // returns false if last chunk would go over dataLimit. Does  not append last block.
    val blockAppended: Boolean,
    val compressedSizeBefore: Int,
    // even when block is not appended, compressedSizeAfter should as if it was appended
    val compressedSizeAfter: Int
  )
}

class GoBackedBlobCompressor private constructor(
  internal val goNativeBlobCompressor: GoNativeBlobCompressor
) : BlobCompressor {

  companion object {
    @Volatile
    private var instance: GoBackedBlobCompressor? = null

    fun getInstance(
      compressorVersion: BlobCompressorVersion = BlobCompressorVersion.V0_1_0,
      dataLimit: UInt
    ): GoBackedBlobCompressor {
      if (instance == null) {
        synchronized(this) {
          if (instance == null) {
            val goNativeBlobCompressor = GoNativeBlobCompressorFactory.getInstance(compressorVersion)
            val initialized = goNativeBlobCompressor.Init(
              dataLimit.toInt(),
              GoNativeBlobCompressorFactory.dictionaryPath.toString()
            )
            if (!initialized) {
              throw InstantiationException(goNativeBlobCompressor.Error())
            }
            instance = GoBackedBlobCompressor(goNativeBlobCompressor)
          } else {
            throw IllegalStateException("Compressor singleton instance already created")
          }
        }
      } else {
        throw IllegalStateException("Compressor singleton instance already created")
      }
      return instance!!
    }
  }

  private val log = LogManager.getLogger(GoBackedBlobCompressor::class.java)

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return goNativeBlobCompressor.CanWrite(blockRLPEncoded, blockRLPEncoded.size)
  }

  fun inflightBlobSize(): Int {
    return goNativeBlobCompressor.Len()
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
      1.0 - ((compressedSizeAfter - compressionSizeBefore).toDouble() / blockRLPEncoded.size)
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
}
