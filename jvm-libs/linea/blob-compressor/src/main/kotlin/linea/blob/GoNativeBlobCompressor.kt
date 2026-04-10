package linea.blob

import com.sun.jna.Library
import com.sun.jna.Native
import com.sun.jna.ptr.PointerByReference
import linea.jvm.ResourcesUtil.copyResourceToTmpDir

/** JVM-facing API of the native blob compressor library. Implemented by both the legacy JNA bindings. */
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

/**
 * JNA binding for the current native compressor library.
 * [Init] returns an integer handle identifying an independent compressor instance; multiple instances
 * can coexist within the same process. [Free] must be called when the instance is no longer needed.
 */
interface GoNativeBlobCompressorV4 {
  fun Init(dataLimit: Int, dictPath: String, errOut: PointerByReference): Int
  fun Free(handle: Int)
  fun Reset(handle: Int)
  fun StartNewBatch(handle: Int)
  fun Write(handle: Int, data: ByteArray, data_len: Int): Boolean
  fun CanWrite(handle: Int, data: ByteArray, data_len: Int): Boolean
  fun Error(handle: Int): String?
  fun Len(handle: Int): Int
  fun Bytes(handle: Int, out: ByteArray)
  fun WorstCompressedBlockSize(handle: Int, data: ByteArray, data_len: Int): Int
  fun WorstCompressedTxSize(handle: Int, data: ByteArray, data_len: Int): Int
  fun RawCompressedSize(handle: Int, data: ByteArray, data_len: Int): Int
}

interface GoNativeBlobCompressorLegacyJnaLib : GoNativeBlobCompressor, Library

interface GoNativeBlobCompressorV4JnaLib : GoNativeBlobCompressorV4, Library

enum class BlobCompressorVersion(val version: String) {
  V2("v2.1.0"),
  V3("v4.0.1"),
  V4("v4.0.1"),
}

/**
 * Loads and caches the native compressor shared library for each [BlobCompressorVersion].
 * Versions are cached in [loadedVersions].
 * The cached object is the raw JNA library binding — it is not an initialised compressor instance.
 */
class GoNativeBlobCompressorFactory {
  companion object {
    private const val DICTIONARY_NAME = "compressor-dictionaries/v2025-04-21.bin"
    val dictionaryPath =
      copyResourceToTmpDir(DICTIONARY_NAME, GoNativeBlobCompressorFactory::class.java.classLoader)

    private fun getLibFileName(version: String) = "blob_compressor_jna_$version"

    @JvmStatic
    private val loadedVersions = mutableMapOf<BlobCompressorVersion, Library>()

    @JvmStatic
    fun getLegacyInstance(version: BlobCompressorVersion): GoNativeBlobCompressor {
      require(version == BlobCompressorVersion.V2) {
        "$version uses the handle-based API; use getInstance() instead"
      }
      val lib = synchronized(loadedVersions) {
        loadedVersions[version]
          ?: loadLib(version, GoNativeBlobCompressorLegacyJnaLib::class.java)
            .also { loadedVersions[version] = it }
      }
      return lib as GoNativeBlobCompressor
    }

    @JvmStatic
    fun getInstance(version: BlobCompressorVersion): GoNativeBlobCompressorV4 {
      require(version != BlobCompressorVersion.V2) {
        "$version uses the legacy API; use getLegacyInstance() instead"
      }
      val lib = synchronized(loadedVersions) {
        loadedVersions[version]
          ?: loadLib(version, GoNativeBlobCompressorV4JnaLib::class.java)
            .also { loadedVersions[version] = it }
      }
      return lib as GoNativeBlobCompressorV4
    }

    private fun <T : Library> loadLib(version: BlobCompressorVersion, classOfT: Class<T>): T {
      val extractedLibFile = Native.extractFromResourcePath(
        getLibFileName(version.version),
        classOfT.classLoader,
      )

      return Native.load(
        /* name = */
        extractedLibFile.toString(),
        /* interfaceClass = */
        classOfT,
      )
    }
  }
}
