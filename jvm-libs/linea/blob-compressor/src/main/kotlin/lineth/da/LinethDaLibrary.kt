package lineth.da

import java.lang.foreign.Arena
import java.lang.foreign.FunctionDescriptor
import java.lang.foreign.Linker
import java.lang.foreign.MemoryLayout
import java.lang.foreign.MemoryLayout.PathElement
import java.lang.foreign.MemorySegment
import java.lang.foreign.StructLayout
import java.lang.foreign.SymbolLookup
import java.lang.foreign.ValueLayout.ADDRESS
import java.lang.foreign.ValueLayout.JAVA_BYTE
import java.lang.foreign.ValueLayout.JAVA_INT
import java.lang.foreign.ValueLayout.JAVA_LONG
import java.lang.invoke.MethodHandle
import java.nio.file.Path

/** The structs of `lineth_da.h`, laid out as the C compiler does. */
internal object NativeLayouts {
  private fun bytes(size: Long) = MemoryLayout.sequenceLayout(size, JAVA_BYTE)

  /** `lineth_da_bytes`: a borrowed byte range. */
  val bytes: StructLayout = MemoryLayout.structLayout(ADDRESS.withName("ptr"), JAVA_LONG.withName("len"))

  /** `lineth_da_compressed`. */
  val compressed: StructLayout = MemoryLayout.structLayout(ADDRESS.withName("frame"), JAVA_LONG.withName("frame_len"))

  /** `lineth_da_segment_end`. */
  val segmentEnd: StructLayout = MemoryLayout.structLayout(
    JAVA_LONG.withName("conflation_end_block"),
    JAVA_INT.withName("unused_tail_bytes"),
    MemoryLayout.paddingLayout(4),
  )

  /** `lineth_da_sealed_blob`. */
  val sealedBlob: StructLayout = MemoryLayout.structLayout(
    bytes(DaConstants.BLOB_BYTES.toLong()).withName("blob"),
    bytes(32).withName("chunk_hash"),
    bytes(DaConstants.KZG_POINT_BYTES.toLong()).withName("kzg_commitment"),
    bytes(DaConstants.KZG_POINT_BYTES.toLong()).withName("kzg_proof"),
    bytes(32).withName("prev_data_rolling_hash"),
    bytes(32).withName("data_rolling_hash"),
    JAVA_INT.withName("payload_bytes"),
    JAVA_INT.withName("segment_end_count"),
    ADDRESS.withName("segment_ends"),
  )

  /** `lineth_da_sealed_blobs`. */
  val sealedBlobs: StructLayout = MemoryLayout.structLayout(ADDRESS.withName("blobs"), JAVA_LONG.withName("count"))

  /** `lineth_da_stream_position`. */
  val streamPosition: StructLayout = MemoryLayout.structLayout(
    bytes(32).withName("data_rolling_hash"),
    bytes(32).withName("parent_blob_data_rolling_hash"),
    JAVA_INT.withName("shared_blob_carry_over_tail_bytes"),
    MemoryLayout.paddingLayout(4),
    ADDRESS.withName("shared_blob"),
  )
}

internal fun StructLayout.offsetOf(field: String): Long = byteOffset(PathElement.groupElement(field))

internal fun MemorySegment.putBytes(offset: Long, source: ByteArray) {
  MemorySegment.copy(source, 0, this, JAVA_BYTE, offset, source.size)
}

internal fun MemorySegment.getBytes(offset: Long, size: Int): ByteArray = asSlice(
  offset,
  size.toLong(),
).toArray(JAVA_BYTE)

/** `lineth_da_status`: `OK` is 0, and the failures follow in the order of [DaStreamErrorType]. */
internal const val STATUS_OK = 0

internal const val ERROR_BUFFER_BYTES = 1024L

internal fun daStreamException(status: Int, message: String): DaStreamException {
  val type = DaStreamErrorType.entries.getOrNull(status - 1)
    ?: return DaStreamException(DaStreamErrorType.INTERNAL, "unknown status $status: $message")
  return DaStreamException(type, message)
}

private fun closed(cause: Throwable? = null) = DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "closed", cause)

/**
 * Runs a call that passes this handle to native code. A handle whose arena is closed is refused by the Foreign
 * Function & Memory API before native code runs, which surfaces here as [DaStreamException] of INVALID_ARGUMENT.
 */
internal inline fun <T> MemorySegment.whileOpen(call: () -> T): T {
  if (!scope().isAlive) throw closed()
  try {
    return call()
  } catch (e: IllegalStateException) {
    if (!scope().isAlive) throw closed(e)
    throw e
  }
}

/**
 * One loaded `lineth_da` library: its symbols bound as downcall handles, after a check that it speaks the ABI this
 * binding is written for.
 */
internal class LinethDaLibrary private constructor(private val path: Path) {
  private val linker = Linker.nativeLinker()
  private val lookup = SymbolLookup.libraryLookup(path, Arena.global())

  private fun bind(name: String, result: MemoryLayout?, vararg arguments: MemoryLayout): MethodHandle {
    val symbol = lookup.find(name).orElseThrow {
      DaStreamException(DaStreamErrorType.INTERNAL, "the library $path has no symbol $name")
    }
    val descriptor =
      if (result == null) FunctionDescriptor.ofVoid(*arguments) else FunctionDescriptor.of(result, *arguments)
    return linker.downcallHandle(symbol, descriptor)
  }

  private val abiVersionHandle = bind("lineth_da_abi_version", JAVA_INT)
  private val compressorNewHandle =
    bind("lineth_da_compressor_new", JAVA_INT, ADDRESS, JAVA_LONG, JAVA_LONG, ADDRESS, ADDRESS, JAVA_LONG)
  private val compressorFreeHandle = bind("lineth_da_compressor_free", null, ADDRESS)
  private val compressHandle =
    bind("lineth_da_compress", JAVA_INT, ADDRESS, ADDRESS, JAVA_LONG, ADDRESS, ADDRESS, JAVA_LONG)
  private val compressedFreeHandle = bind("lineth_da_compressed_free", null, ADDRESS)
  private val streamOpenHandle = bind("lineth_da_stream_open", JAVA_INT, ADDRESS, ADDRESS, ADDRESS, JAVA_LONG)
  private val streamCloseHandle = bind("lineth_da_stream_close", null, ADDRESS)
  private val streamAppendHandle =
    bind("lineth_da_stream_append", JAVA_INT, ADDRESS, ADDRESS, JAVA_LONG, JAVA_LONG, ADDRESS, ADDRESS, JAVA_LONG)
  private val streamFlushHandle = bind("lineth_da_stream_flush", JAVA_INT, ADDRESS, ADDRESS, ADDRESS, JAVA_LONG)
  private val sealedBlobsFreeHandle = bind("lineth_da_sealed_blobs_free", null, ADDRESS)

  fun abiVersion(): Int = abiVersionHandle.invoke() as Int

  fun compressorNew(
    dictionary: MemorySegment,
    dictionaryLen: Long,
    chainId: Long,
    out: MemorySegment,
    err: MemorySegment,
  ): Int = compressorNewHandle.invoke(dictionary, dictionaryLen, chainId, out, err, ERROR_BUFFER_BYTES) as Int

  fun compressorFree(compressor: MemorySegment) {
    compressorFreeHandle.invoke(compressor)
  }

  fun compress(
    compressor: MemorySegment,
    blockRlps: MemorySegment,
    blockCount: Long,
    out: MemorySegment,
    err: MemorySegment,
  ): Int = compressHandle.invoke(compressor, blockRlps, blockCount, out, err, ERROR_BUFFER_BYTES) as Int

  fun compressedFree(compressed: MemorySegment) {
    compressedFreeHandle.invoke(compressed)
  }

  fun streamOpen(position: MemorySegment, out: MemorySegment, err: MemorySegment): Int =
    streamOpenHandle.invoke(position, out, err, ERROR_BUFFER_BYTES) as Int

  fun streamClose(stream: MemorySegment) {
    streamCloseHandle.invoke(stream)
  }

  fun streamAppend(
    stream: MemorySegment,
    frame: MemorySegment,
    frameLen: Long,
    conflationEndBlock: Long,
    out: MemorySegment,
    err: MemorySegment,
  ): Int = streamAppendHandle.invoke(stream, frame, frameLen, conflationEndBlock, out, err, ERROR_BUFFER_BYTES) as Int

  fun streamFlush(stream: MemorySegment, out: MemorySegment, err: MemorySegment): Int =
    streamFlushHandle.invoke(stream, out, err, ERROR_BUFFER_BYTES) as Int

  fun sealedBlobsFree(sealed: MemorySegment) {
    sealedBlobsFreeHandle.invoke(sealed)
  }

  companion object {
    /** The `LINETH_DA_ABI_VERSION` of the header this binding is written for. */
    const val ABI_VERSION = 1

    fun load(path: Path, abiVersion: Int = ABI_VERSION): LinethDaLibrary {
      val library = try {
        LinethDaLibrary(path)
      } catch (e: IllegalArgumentException) {
        throw DaStreamException(DaStreamErrorType.INTERNAL, "cannot load the library $path", e)
      }
      val actual = library.abiVersion()
      if (actual != abiVersion) {
        throw DaStreamException(
          DaStreamErrorType.INTERNAL,
          "the library $path has ABI version $actual, the binding is written for $abiVersion",
        )
      }
      return library
    }
  }
}
