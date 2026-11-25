package lineth.da

import java.lang.foreign.Arena
import java.lang.foreign.MemorySegment
import java.lang.foreign.ValueLayout.ADDRESS
import java.lang.foreign.ValueLayout.JAVA_INT
import java.lang.foreign.ValueLayout.JAVA_LONG

/**
 * A `lineth_da_stream` behind its arena, like [ZigDaCompressor]. A stream is used by one thread at a time, so its
 * methods are synchronized: a second thread waits instead of racing on native memory.
 */
internal class ZigDaStream(
  override val version: DaCodecVersion,
  private val library: LinethDaLibrary,
  position: DaStreamPosition,
) : DaStream {
  private val arena = Arena.ofShared()
  private val handle: MemorySegment = try {
    open(position)
  } catch (e: Throwable) {
    arena.close()
    throw e
  }

  private fun open(position: DaStreamPosition): MemorySegment =
    Arena.ofConfined().use { call ->
      val layout = NativeLayouts.streamPosition
      val native = call.allocate(layout)
      native.putBytes(layout.offsetOf("data_rolling_hash"), position.dataRollingHash)
      native.putBytes(layout.offsetOf("parent_blob_data_rolling_hash"), position.parentBlobDataRollingHash)
      native.set(
        JAVA_INT,
        layout.offsetOf("shared_blob_carry_over_tail_bytes"),
        position.sharedBlobCarryOverTailBytes.toInt(),
      )
      position.sharedBlob?.let { sharedBlob ->
        val copy = call.allocate(sharedBlob.size.toLong())
        copy.putBytes(0, sharedBlob)
        native.set(ADDRESS, layout.offsetOf("shared_blob"), copy)
      }
      val out = call.allocate(ADDRESS)
      val err = call.allocate(ERROR_BUFFER_BYTES)
      val status = library.streamOpen(native, out, err)
      if (status != STATUS_OK) throw daStreamException(status, err.getString(0))
      out.get(ADDRESS, 0).reinterpret(0, arena) { library.streamClose(it) }
    }

  @Synchronized
  override fun append(frame: ByteArray, conflationEndBlock: ULong): List<SealedBlob> = handle.whileOpen {
    Arena.ofConfined().use { call ->
      val copy = call.allocate(frame.size.toLong())
      copy.putBytes(0, frame)
      sealing(call) { out, err ->
        library.streamAppend(handle, copy, frame.size.toLong(), conflationEndBlock.toLong(), out, err)
      }
    }
  }

  @Synchronized
  override fun flush(): SealedBlob? = handle.whileOpen {
    Arena.ofConfined().use { call ->
      sealing(call) { out, err -> library.streamFlush(handle, out, err) }.singleOrNull()
    }
  }

  @Synchronized
  override fun close() {
    if (handle.scope().isAlive) arena.close()
  }

  private inline fun sealing(call: Arena, native: (out: MemorySegment, err: MemorySegment) -> Int): List<SealedBlob> {
    val out = call.allocate(NativeLayouts.sealedBlobs)
    val err = call.allocate(ERROR_BUFFER_BYTES)
    try {
      val status = native(out, err)
      if (status != STATUS_OK) throw daStreamException(status, err.getString(0))
      return readSealedBlobs(out)
    } finally {
      library.sealedBlobsFree(out)
    }
  }

  private fun readSealedBlobs(sealed: MemorySegment): List<SealedBlob> {
    val count = sealed.get(JAVA_LONG, NativeLayouts.sealedBlobs.offsetOf("count"))
    if (count == 0L) return emptyList()
    val blobSize = NativeLayouts.sealedBlob.byteSize()
    val blobs = sealed.get(ADDRESS, NativeLayouts.sealedBlobs.offsetOf("blobs")).reinterpret(count * blobSize)
    return (0 until count).map { index -> readSealedBlob(blobs.asSlice(index * blobSize, blobSize)) }
  }

  private fun readSealedBlob(native: MemorySegment): SealedBlob {
    val layout = NativeLayouts.sealedBlob
    val endCount = native.get(JAVA_INT, layout.offsetOf("segment_end_count"))
    val endLayout = NativeLayouts.segmentEnd
    val ends = native.get(ADDRESS, layout.offsetOf("segment_ends")).reinterpret(endCount * endLayout.byteSize())
    return SealedBlob(
      blob = native.getBytes(layout.offsetOf("blob"), DaConstants.BLOB_BYTES),
      chunkHash = native.getBytes(layout.offsetOf("chunk_hash"), 32),
      kzgCommitment = native.getBytes(layout.offsetOf("kzg_commitment"), DaConstants.KZG_POINT_BYTES),
      kzgProof = native.getBytes(layout.offsetOf("kzg_proof"), DaConstants.KZG_POINT_BYTES),
      parentDataRollingHash = native.getBytes(layout.offsetOf("prev_data_rolling_hash"), 32),
      dataRollingHash = native.getBytes(layout.offsetOf("data_rolling_hash"), 32),
      payloadBytes = native.get(JAVA_INT, layout.offsetOf("payload_bytes")).toUInt(),
      segmentEnds = (0 until endCount).map { index ->
        val end = ends.asSlice(index * endLayout.byteSize(), endLayout.byteSize())
        SegmentEnd(
          conflationEndBlock = end.get(JAVA_LONG, endLayout.offsetOf("conflation_end_block")).toULong(),
          unusedTailBytes = end.get(JAVA_INT, endLayout.offsetOf("unused_tail_bytes")).toUInt(),
        )
      },
    )
  }
}
