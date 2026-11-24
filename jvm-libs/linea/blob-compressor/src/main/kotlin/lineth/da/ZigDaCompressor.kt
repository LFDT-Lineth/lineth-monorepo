package lineth.da

import java.lang.foreign.Arena
import java.lang.foreign.MemorySegment
import java.lang.foreign.ValueLayout.ADDRESS
import java.lang.foreign.ValueLayout.JAVA_BYTE
import java.lang.foreign.ValueLayout.JAVA_LONG

/**
 * A `lineth_da_compressor` behind its arena: closing the arena frees the native compressor, and FFM refuses to close
 * it while a downcall that received the handle is in flight.
 *
 * [compress] runs on any number of threads at once, as the library allows.
 */
internal class ZigDaCompressor(
  override val version: DaCodecVersion,
  private val library: LinethDaLibrary,
  dictionary: ByteArray,
  chainId: ULong,
) : DaCompressor {
  private val arena = Arena.ofShared()
  private val handle: MemorySegment = try {
    create(dictionary, chainId)
  } catch (e: Throwable) {
    arena.close()
    throw e
  }

  private fun create(dictionary: ByteArray, chainId: ULong): MemorySegment =
    Arena.ofConfined().use { call ->
      val dictionaryCopy = call.allocate(dictionary.size.toLong())
      dictionaryCopy.putBytes(0, dictionary)
      val out = call.allocate(ADDRESS)
      val err = call.allocate(ERROR_BUFFER_BYTES)
      val status =
        library.compressorNew(dictionaryCopy, dictionary.size.toLong(), chainId.toLong(), out, err)
      if (status != STATUS_OK) throw daStreamException(status, err.getString(0))
      out.get(ADDRESS, 0).reinterpret(0, arena) { library.compressorFree(it) }
    }

  override fun compress(blockRlps: List<ByteArray>): ByteArray = handle.whileOpen {
    Arena.ofConfined().use { call ->
      val inputs = call.allocate(NativeLayouts.bytes, blockRlps.size.toLong())
      val inputSize = NativeLayouts.bytes.byteSize()
      blockRlps.forEachIndexed { index, rlp ->
        val copy = call.allocate(rlp.size.toLong())
        copy.putBytes(0, rlp)
        inputs.set(ADDRESS, index * inputSize + NativeLayouts.bytes.offsetOf("ptr"), copy)
        inputs.set(JAVA_LONG, index * inputSize + NativeLayouts.bytes.offsetOf("len"), rlp.size.toLong())
      }
      val out = call.allocate(NativeLayouts.compressed)
      val err = call.allocate(ERROR_BUFFER_BYTES)
      try {
        val status = library.compress(handle, inputs, blockRlps.size.toLong(), out, err)
        if (status != STATUS_OK) throw daStreamException(status, err.getString(0))
        val frameLen = out.get(JAVA_LONG, NativeLayouts.compressed.offsetOf("frame_len"))
        out.get(ADDRESS, NativeLayouts.compressed.offsetOf("frame"))
          .reinterpret(frameLen)
          .toArray(JAVA_BYTE)
      } finally {
        library.compressedFree(out)
      }
    }
  }

  override fun close() {
    if (handle.scope().isAlive) arena.close()
  }
}
