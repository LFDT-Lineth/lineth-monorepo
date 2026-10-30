package linea.da

import linea.clients.ChunkWitness
import linea.domain.BlockInterval
import java.nio.ByteBuffer
import java.nio.file.Path

/**
 * The RISC-V DA stream (Level 1 of the blob compressor design): compresses conflations into frames, appends them to
 * the DA stream, packs and seals blobs, and computes each blob's KZG versioned hash and data rolling hash.
 *
 * The DA stream is `[u32 BE len][compressed truncated-block RLP]` per conflation, concatenated. Blobs are windows over
 * it: each holds up to [DaConstants.BLOB_PAYLOAD_CAPACITY] payload bytes plus a `0xff` terminal. Conflations may span
 * blobs, and adjacent rollup proofs may share a blob.
 *
 * Blobs are sealed **lazily**: a full blob is sealed only when the next frame needs a new blob, or on [flush].
 *
 * Threading: [compress] is pure and safe from any thread. [append] and [flush] must be confined to one thread.
 */
interface DaStream : AutoCloseable {
  val version: DaCodecVersion

  /**
   * Decodes, truncates, RLP-encodes and compresses one conflation into a frame. Order-free and pure: may run ahead of
   * time and in parallel. The frame records the conflation's block interval, reported back in [SegmentEnd]s.
   */
  fun compress(blockRlps: List<ByteArray>): CompressedConflation

  /**
   * Appends a frame. Frames must be appended in conflation order; ordering is the caller's responsibility. Consumes
   * the frame on success. On error the frame stays with the caller and the stream is unchanged.
   *
   * @return the blobs this call sealed, in order (0..n).
   */
  fun append(conflation: CompressedConflation): List<SealedBlob>

  /** Ends the open blob at the stream end. @return that blob, or null if no blob is open. */
  fun flush(): SealedBlob?
}

/** A conflation compressed into one frame of the DA stream, consumed by [DaStream.append]. */
interface CompressedConflation {
  val blockInterval: BlockInterval

  /** Bytes this conflation adds to the DA stream: the 4-byte length prefix plus the compressed frame. */
  val compressedBytes: Int

  /** Size of the truncated-block RLP the frame decompresses to. */
  val uncompressedBytes: Int
}

/** A sealed blob: immutable and independent of the stream. Share it by reference; it is never mutated. */
interface SealedBlob {
  /** Read-only view of the [DaConstants.BLOB_BYTES]-byte packed blob. */
  val blob: ByteBuffer

  /** KZG versioned hash of [blob]. */
  val chunkHash: ByteArray

  /** Data rolling hash before this blob. */
  val prevDataRollingHash: ByteArray

  /** Data rolling hash after this blob. */
  val dataRollingHash: ByteArray

  /** Payload bytes in this blob: [DaConstants.BLOB_PAYLOAD_CAPACITY] unless ended early by a flush. */
  val payloadBytes: Int

  /** Cut points inside this blob, in order: where each conflation frame ending in this blob ends. */
  val segmentEnds: List<SegmentEnd>
}

/** Copies the blob bytes out of [SealedBlob.blob]. */
fun SealedBlob.blobBytes(): ByteArray {
  val view = blob.duplicate()
  return ByteArray(view.remaining()).also { view.get(it) }
}

/**
 * A cut point: the frame of the conflation ending at [conflationEndBlock] ends at [offset] within the blob's payload.
 * The tail at this cut (discarded by the rollup ending here, taken by the next one) is `payloadBytes - offset`.
 */
data class SegmentEnd(
  val conflationEndBlock: ULong,
  val offset: Int,
)

/** Opens [DaStream]s. */
fun interface DaStreamFactory {
  fun open(version: DaCodecVersion, dictionary: Path, initialState: DaStreamPosition): DaStream
}

/** Identifies the DA codec build. The compression dictionary is configured independently. */
enum class DaCodecVersion(val version: String) {
  /** Pure-Kotlin stand-in with the real stream semantics; see `FakeDaStream`. */
  FAKE("fake-v1"),
}

object DaConstants {
  /** Size of a packed EIP-4844 blob: 4096 field elements of 32 bytes. */
  const val BLOB_BYTES = ChunkWitness.BLOB_BYTES_LENGTH

  /** Payload bytes per blob: 4096 field elements of 254 bits, minus the `0xff` terminal byte. */
  const val BLOB_PAYLOAD_CAPACITY = 4096 * 254 / 8 - 1

  /** Terminal byte that ends a blob's payload. */
  const val BLOB_PAYLOAD_TERMINAL: Byte = 0xff.toByte()
}

/** Error codes of the DA stream, mirroring `lineth_da_status`. */
enum class DaStreamStatus {
  INVALID_ARGUMENT,
  INVALID_DICTIONARY,
  INVALID_BLOCK_RLP,

  /** Restoring: the appended bytes differ from the shared blob the stream was opened inside. */
  SEEDED_BLOB_MISMATCH,
  INDEX_OUT_OF_RANGE,
  OUT_OF_MEMORY,
  INTERNAL,
}

class DaStreamException(
  val status: DaStreamStatus,
  message: String,
  cause: Throwable? = null,
) : RuntimeException("$status: $message", cause)
