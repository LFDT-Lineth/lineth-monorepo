package linea.blob

import com.sun.jna.Library
import com.sun.jna.Native
import linea.jvm.ResourcesUtil.copyResourceToTmpDir

interface GoNativeBlobCompressor {

  /**
   * Init initializes the compressor. dataLimit is the maximum number of bytes of compressed data
   *
   * @param dataLimit Size limit for compressed data in bytes
   * @param dictPath Path to the compression dictionary
   * @return returns true if the compressor was successfully intialized else false
   */
  fun Init(dataLimit: Int, dictPath: String): Boolean

  /**
   * Reset clears the compressor
   */
  fun Reset()

  /**
   * StartNewBatch starts a new batch. Must be called between batches in the same blob
   */
  fun StartNewBatch()

  /**
   * Write appends rlp encoded block to the compressor. Returns true if the
   * data was successfully written, false if the compressor is full (see
   * dataLimit in Init) or if the data is invalid, in which case Error() will
   * return a description of the error.
   *
   * @param data  bytes of the rlp encoded block
   * @param data_len  number of bytes
   * @return true if data was successfully compressed else false
   */
  fun Write(data: ByteArray, data_len: Int): Boolean

  /**
   * CanWrite behaves like Write but does not actually write the data. It can
   * be used to check if the data is valid and if it would fit in the
   * compressor.
   *
   * @param data  bytes of the rlp encoded block
   * @param data_len  number of bytes
   * @return true if data was successfully compressed else false
   */
  fun CanWrite(data: ByteArray, data_len: Int): Boolean

  /**
   * Error returns the last error message. Should be checked if Write returns false.
   */
  fun Error(): String?

  /**
   * Len returns the number of bytes of compressed data.
   */
  fun Len(): Int

  /**
   * Bytes fills out with the compressed data. The caller must allocate out
   * and ensure that len(out) == Len()
   *
   * @param out The ByteArray to be filled with compressed data
   */
  fun Bytes(out: ByteArray)

  /** WorstCompressedBlockSize returns the size of the given block, as compressed by an "empty" blob maker.
   That is, with more context, blob maker could compress the block further, but this function
   returns the maximum size that can be achieved.

   @param data bytes of the rlp encoded block
   @param data_len number of bytes
   @return size of the compressed data in bytes, or -1 if an error occurred
   */
  fun WorstCompressedBlockSize(data: ByteArray, data_len: Int): Int

  /**
   * WorstCompressedTxSize returns the size of the given transaction, as compressed by an "empty" blob maker.
   * That is, with more context, blob maker could compress the transaction further, but this function
   * returns the maximum size that can be achieved.
   *
   * @param data bytes of the rlp encoded transaction
   * @param data_len number of bytes
   * @return size of the compressed data in bytes, or -1 if an error occurred
   */
  fun WorstCompressedTxSize(data: ByteArray, data_len: Int): Int

  /**
   * RawCompressedSize returns the size of the raw data, compressed, in bytes
   *
   * @param data bytes of the (uncompressed) data
   * @param data_len  number of bytes (must be less than 256kB)
   * @return size of the compressed data in bytes
   */
  fun RawCompressedSize(data: ByteArray, data_len: Int): Int
}

interface GoNativeBlobCompressorJnaLib : GoNativeBlobCompressor, Library

enum class BlobCompressorVersion(val version: String) {
  V1_2("v1.2.0"),
  V2("v2.1.0"),
}

class GoNativeBlobCompressorFactory {
  companion object {
    private const val DICTIONARY_NAME = "compressor-dictionaries/v2025-04-21.bin"
    val dictionaryPath =
      copyResourceToTmpDir(DICTIONARY_NAME, GoNativeBlobCompressorFactory::class.java.classLoader)

    private fun getLibFileName(version: String) = "blob_compressor_jna_$version"

    @JvmStatic
    private val loadedVersions = mutableMapOf<BlobCompressorVersion, GoNativeBlobCompressor>()

    @JvmStatic
    fun getInstance(version: BlobCompressorVersion): GoNativeBlobCompressor {
      synchronized(loadedVersions) {
        return loadedVersions[version]
          ?: loadLib(version)
            .also { loadedVersions[version] = it }
      }
    }

    private fun loadLib(version: BlobCompressorVersion): GoNativeBlobCompressor {
      val extractedLibFile = Native.extractFromResourcePath(
        getLibFileName(version.version),
        GoNativeBlobCompressorFactory::class.java.classLoader,
      )

      return Native.load(
        /* name = */
        extractedLibFile.toString(),
        /* interfaceClass = */
        GoNativeBlobCompressorJnaLib::class.java,
      )
    }
  }
}
