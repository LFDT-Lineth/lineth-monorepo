package linea.da

import linea.domain.BlockInterval
import linea.kotlin.encodeHex
import java.nio.file.Path

/**
 * Compresses conflations into zstd frames for the RISC-V DA stream, with one codec version and dictionary.
 *
 * Stateless: [compress] is order-free and pure, so it may run ahead of [DaStream.append] and from any thread.
 */
interface DaCompressor : AutoCloseable {
  val version: DaCodecVersion

  /** Decodes, truncates, RLP-encodes and compresses one conflation into a zstd frame. */
  fun compress(blockRlps: List<ByteArray>): CompressedConflation
}

/**
 * The RISC-V DA stream: appends one segment per compressed conflation, packs and seals blobs, and computes each blob's
 * KZG versioned hash and data rolling hash.
 *
 * The DA stream is the concatenation of segments, one per conflation: `[u32 BE len][zstd frame]`, where the frame is the
 * compressed truncated-block RLP. Blobs are windows over
 * it: each holds up to [DaConstants.BLOB_PAYLOAD_CAPACITY] payload bytes plus a `0xff` terminal. Conflations may span
 * blobs, and adjacent rollup proofs may share a blob.
 *
 * Blobs are sealed **lazily**: a full blob is sealed only when the next segment needs a new blob, or on [flush].
 *
 * Not thread-safe: [append] and [flush] must be confined to one thread.
 */
interface DaStream : AutoCloseable {
  val version: DaCodecVersion

  /**
   * Appends the segment of a conflation compressed by a [DaCompressor] of the same [version]. Conflations must be
   * appended in order, each once; ordering is the caller's responsibility. On error the stream is unchanged.
   *
   * @return the blobs this call sealed, in order (0..n).
   */
  fun append(conflation: CompressedConflation): List<SealedBlob>

  /** Ends the open blob at the stream end. @return that blob, or null if no blob is open. */
  fun flush(): SealedBlob?
}

/**
 * A conflation compressed into one zstd frame: the body of its segment in the DA stream.
 *
 * @property blockInterval the conflation's blocks, reported back in [SegmentEnd]s
 * @property compressedData the zstd frame, without the length prefix: its segment takes
 *   [DaConstants.SEGMENT_LENGTH_PREFIX_BYTES] + `compressedData.size` bytes of the stream
 */
data class CompressedConflation(
  val blockInterval: BlockInterval,
  val compressedData: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as CompressedConflation

    if (blockInterval != other.blockInterval) return false
    if (!compressedData.contentEquals(other.compressedData)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blockInterval.hashCode()
    result = 31 * result + compressedData.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "CompressedConflation(blockInterval=${blockInterval.intervalString()}, " +
      "compressedData.size=${compressedData.size})"
  }
}

/**
 * A sealed blob: independent of the stream, never mutated by it. Shared by reference, e.g. by the two rollups of a
 * shared blob, so holders must not mutate its arrays either.
 *
 * @property blob the [DaConstants.BLOB_BYTES]-byte packed blob
 * @property chunkHash KZG versioned hash of [blob], derived from [kzgCommitment]
 * @property kzgCommitment [DaConstants.KZG_POINT_BYTES]-byte KZG commitment of [blob]
 * @property kzgProof [DaConstants.KZG_POINT_BYTES]-byte KZG proof for [blob] against [kzgCommitment]
 * @property prevDataRollingHash data rolling hash before this blob
 * @property dataRollingHash data rolling hash after this blob
 * @property payloadBytes payload bytes in this blob: [DaConstants.BLOB_PAYLOAD_CAPACITY] unless ended early by a flush
 * @property segmentEnds cut points inside this blob, in order: where each conflation segment ending in this blob ends
 */
data class SealedBlob(
  val blob: ByteArray,
  val chunkHash: ByteArray,
  val kzgCommitment: ByteArray,
  val kzgProof: ByteArray,
  val prevDataRollingHash: ByteArray,
  val dataRollingHash: ByteArray,
  val payloadBytes: Int,
  val segmentEnds: List<SegmentEnd>,
) {
  init {
    require(blob.size == DaConstants.BLOB_BYTES) { "blob must be ${DaConstants.BLOB_BYTES} bytes, got ${blob.size}" }
    require(chunkHash.size == 32) { "chunkHash must be 32 bytes, got ${chunkHash.size}" }
    require(kzgCommitment.size == DaConstants.KZG_POINT_BYTES) {
      "kzgCommitment must be ${DaConstants.KZG_POINT_BYTES} bytes, got ${kzgCommitment.size}"
    }
    require(kzgProof.size == DaConstants.KZG_POINT_BYTES) {
      "kzgProof must be ${DaConstants.KZG_POINT_BYTES} bytes, got ${kzgProof.size}"
    }
    require(prevDataRollingHash.size == 32) { "prevDataRollingHash must be 32 bytes, got ${prevDataRollingHash.size}" }
    require(dataRollingHash.size == 32) { "dataRollingHash must be 32 bytes, got ${dataRollingHash.size}" }
    require(payloadBytes in 1..DaConstants.BLOB_PAYLOAD_CAPACITY) {
      "payloadBytes=$payloadBytes must be in 1..${DaConstants.BLOB_PAYLOAD_CAPACITY}"
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as SealedBlob

    if (!blob.contentEquals(other.blob)) return false
    if (!chunkHash.contentEquals(other.chunkHash)) return false
    if (!kzgCommitment.contentEquals(other.kzgCommitment)) return false
    if (!kzgProof.contentEquals(other.kzgProof)) return false
    if (!prevDataRollingHash.contentEquals(other.prevDataRollingHash)) return false
    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    if (payloadBytes != other.payloadBytes) return false
    if (segmentEnds != other.segmentEnds) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blob.contentHashCode()
    result = 31 * result + chunkHash.contentHashCode()
    result = 31 * result + kzgCommitment.contentHashCode()
    result = 31 * result + kzgProof.contentHashCode()
    result = 31 * result + prevDataRollingHash.contentHashCode()
    result = 31 * result + dataRollingHash.contentHashCode()
    result = 31 * result + payloadBytes
    result = 31 * result + segmentEnds.hashCode()
    return result
  }

  override fun toString(): String {
    return "SealedBlob(chunkHash=${chunkHash.encodeHex()}, prevDataRollingHash=${prevDataRollingHash.encodeHex()}, " +
      "dataRollingHash=${dataRollingHash.encodeHex()}, payloadBytes=$payloadBytes, segmentEnds=$segmentEnds)"
  }
}

/**
 * A cut point: the segment of the conflation ending at [conflationEndBlock] ends inside this blob, followed by
 * [unusedTailBytes] payload bytes. A rollup ending here leaves them unused (`finalDataTailDiscardBytes`), and the next
 * rollup takes them (`parentDataTailTakeBytes`).
 */
data class SegmentEnd(
  val conflationEndBlock: ULong,
  val unusedTailBytes: Int,
) {
  init {
    require(unusedTailBytes in 0..DaConstants.MAX_TAIL_BYTES) {
      "unusedTailBytes=$unusedTailBytes must be in 0..${DaConstants.MAX_TAIL_BYTES}"
    }
  }
}

/** Creates [DaCompressor]s. */
fun interface DaCompressorFactory {
  fun create(version: DaCodecVersion, dictionary: Path): DaCompressor
}

/** Opens [DaStream]s. */
fun interface DaStreamFactory {
  fun open(version: DaCodecVersion, initialState: DaStreamPosition): DaStream
}

/** Identifies the DA codec build. The compression dictionary is configured independently. */
enum class DaCodecVersion(val version: String) {
  /** Pure-Kotlin stand-in with the real stream semantics; see `FakeDaStream`. */
  FAKE("fake-v1"),
}

object DaConstants {
  /** Size of a packed EIP-4844 blob: 4096 field elements of 32 bytes. */
  const val BLOB_BYTES = 4096 * 32

  /** Compressed BLS12-381 G1 point: the size of a KZG commitment and of a KZG proof. */
  const val KZG_POINT_BYTES = 48

  /** Payload bytes per blob: 4096 field elements of 254 bits, minus the `0xff` terminal byte. */
  const val BLOB_PAYLOAD_CAPACITY = 4096 * 254 / 8 - 1

  /**
   * Upper bound of a blob tail byte count (`tailCount` in the V1 schemas): a tail shared with the next rollup is strictly
   * shorter than a blob's payload, otherwise the whole blob would belong to the next rollup.
   */
  const val MAX_TAIL_BYTES = BLOB_PAYLOAD_CAPACITY - 1

  /** Size of a segment's big-endian `u32` length prefix, written before its zstd frame. */
  const val SEGMENT_LENGTH_PREFIX_BYTES = 4

  /** Terminal byte that ends a blob's payload. */
  const val BLOB_PAYLOAD_TERMINAL: Byte = 0xff.toByte()
}

/** Kinds of DA stream and compressor failures, mirroring the error codes of `lineth_da_status`. */
enum class DaStreamErrorType {
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
  val errorType: DaStreamErrorType,
  message: String,
  cause: Throwable? = null,
) : RuntimeException("$errorType: $message", cause)
