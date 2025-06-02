package net.consensys.linea.blob

import com.sun.jna.Library
import com.sun.jna.Native
import linea.jvm.ResourcesUtil.copyResourceToTmpDir
import java.nio.file.Path

class DecompressionException(message: String) : RuntimeException(message)

interface BlobDecompressor {
  fun decompress(blob: ByteArray): ByteArray
}

internal class Adapter(
  private val delegate: GoNativeBlobDecompressorJnaBinding,
  private val maxExpectedCompressionRatio: Int = 20,
  dictionaries: List<Path>,
) : BlobDecompressor {
  init {
    delegate.Init()

    val paths = dictionaries.joinToString(separator = ":") { path -> path.toString() }

    if (delegate.LoadDictionaries(paths) != dictionaries.size) {
      throw DecompressionException("Failed to load dictionaries '$paths', error='${delegate.Error()}'")
    }
  }

  override fun decompress(blob: ByteArray): ByteArray {
    val decompressionBuffer = ByteArray(blob.size * maxExpectedCompressionRatio)
    val decompressedSize = delegate.Decompress(blob, blob.size, decompressionBuffer, decompressionBuffer.size)
    if (decompressedSize < 0) {
      throw DecompressionException("Decompression failed, error='${delegate.Error()}'")
    }
    return decompressionBuffer.copyOf(decompressedSize)
  }
}

internal interface GoNativeBlobDecompressorJnaBinding {

  /**
   * Init initializes the Decompressor. Must be run before anything else.
   */
  fun Init()

  /**
   * LoadDictionaries attempts to cache dictionaries from given paths, separated by colons,
   * e.g. "../compressor_dict.bin:./other_dict"
   * Returns the number of dictionaries successfully loaded, and -1 in case of failure, in which case Error() will
   * return a description of the error.
   *
   * @param dictPaths a colon-separated list of paths to dictionaries, to be loaded into the decompressor
   * @return the number of dictionaries loaded if successful, -1 if not.
   */
  fun LoadDictionaries(dictPaths: String): Int

  /**

   * Decompress processes a Linea blob and outputs an RLP encoded list of blocks.
   * Due to information loss during pre-compression encoding, two pieces of information are represented "hackily":
   * The block hash is in the ParentHash field.
   * The transaction from address is in the signature.R field.
   *
   * Returns the number of bytes in out, or -1 in case of failure
   * If -1 is returned, the Error() method will return a string describing the error.
   *
   * @param blob to be decompressed
   * @param blob_len length of the blob
   * @param out buffer to write the decompressed data
   * @param out_max_len maximum length of the out buffer
   * @return number of bytes in out, or -1 in case of failure
   */
  fun Decompress(blob: ByteArray, blob_len: Int, out: ByteArray, out_max_len: Int): Int

  /**
   * Error returns the last error message. Should be checked if Write returns false.
   */
  fun Error(): String?
}

internal interface GoNativeBlobDecompressorJnaLib : GoNativeBlobDecompressorJnaBinding, Library

enum class BlobDecompressorVersion(val version: String) {
  V1_2_0("v1.2.0"),
}

class GoNativeBlobDecompressorFactory {
  companion object {
    private val DICTIONARY_NAMES = arrayOf(
      "compressor-dictionaries/compressor_dict.bin",
      "compressor-dictionaries/v2025-04-21.bin",
    )
    private val dictionaryPaths = DICTIONARY_NAMES.map { dictionaryName ->
      copyResourceToTmpDir(dictionaryName, GoNativeBlobDecompressorFactory::class.java.classLoader)
    }

    private fun getLibFileName(version: String) = "blob_decompressor_jna_$version"

    fun getInstance(
      version: BlobDecompressorVersion,
    ): BlobDecompressor {
      val libFile = Native.extractFromResourcePath(
        getLibFileName(version.version),
        GoNativeBlobDecompressorFactory::class.java.classLoader,
      )
      return Native.load(
        libFile.toString(),
        GoNativeBlobDecompressorJnaLib::class.java,
      ).let {
        Adapter(delegate = it, dictionaries = dictionaryPaths)
      }
    }
  }
}
