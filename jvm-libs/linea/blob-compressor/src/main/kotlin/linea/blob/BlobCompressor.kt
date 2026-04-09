package linea.blob

import com.sun.jna.ptr.PointerByReference

class BlobCompressionException(message: String) : RuntimeException(message)

interface BlobCompressor : AutoCloseable {

  val version: BlobCompressorVersion

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

object BlobCompressorFactory {
  @JvmStatic
  fun getInstance(compressorVersion: BlobCompressorVersion, dataLimit: Int): BlobCompressor {
    require(dataLimit > 0) { "dataLimit=$dataLimit must be greater than 0" }

    val dictPath = GoNativeBlobCompressorFactory.dictionaryPath.toString()
    val blobCompressor =
      when (compressorVersion) {
        BlobCompressorVersion.V2 -> {
          val lib = GoNativeBlobCompressorFactory.getLegacyInstance(compressorVersion)
          if (!lib.Init(dataLimit, dictPath)) throw InstantiationException(lib.Error())
          GoBackedBlobCompressor(lib, compressorVersion)
        }

        else -> {
          val lib = GoNativeBlobCompressorFactory.getInstance(compressorVersion)
          val errOut = PointerByReference()
          val handle = lib.Init(dataLimit, dictPath, errOut)
          if (handle == -1) {
            throw InstantiationException(
              errOut.value?.getString(0) ?: "Failed to initialize compressor",
            )
          }
          try {
            GoBackedBlobCompressorV4(lib, compressorVersion, handle)
          } catch (e: Throwable) {
            lib.Free(handle)
            throw e
          }
        }
      }
    return blobCompressor
  }
}
