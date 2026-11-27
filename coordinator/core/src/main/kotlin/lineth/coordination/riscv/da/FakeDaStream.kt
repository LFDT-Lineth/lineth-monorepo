package lineth.coordination.riscv.da

import linea.da.CompressedConflation
import linea.da.DaCodecVersion
import linea.da.DaConstants.BLOB_BYTES
import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaConstants.BLOB_PAYLOAD_TERMINAL
import linea.da.DaConstants.KZG_POINT_BYTES
import linea.da.DaConstants.SEGMENT_LENGTH_PREFIX_BYTES
import linea.da.DaStream
import linea.da.DaStreamErrorType
import linea.da.DaStreamException
import linea.da.DaStreamFactory
import linea.da.DaStreamPosition
import linea.da.SealedBlob
import linea.da.SegmentEnd
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.datatypes.Hash
import java.nio.ByteBuffer

/**
 * Pure-Kotlin [DaStream] with the real stream semantics and simple stand-ins for the codec. Used in tests, and in
 * production to exercise the rollup proof flow until the native `lineth_da` library is available. Its conflations are
 * compressed by [FakeDaCompressor].
 *
 * Real semantics:
 * - segments `[u32 BE len][frame]`, appended in order
 * - blobs of [BLOB_PAYLOAD_CAPACITY] payload bytes, sealed lazily (a full blob is sealed only when the next segment
 *   needs space, or on [flush])
 * - segment ends where each conflation's segment ends, including segments that span blobs
 * - resume inside a shared blob from [DaStreamPosition]: appended bytes must reproduce the shared blob's tail
 *   ([DaStreamErrorType.SEEDED_BLOB_MISMATCH] otherwise), and flushing is rejected until they have
 *
 * Stand-ins (the guest will not accept these blobs):
 * - no 254-bit packing: the blob is the payload, the `0xff` terminal, then zero padding
 * - [chunkHasher] instead of a KZG versioned hash
 * - [fakeKzgCommitment] and [fakeKzgProof] instead of a KZG commitment and proof
 *
 * Not thread-safe: [append] and [flush] must be confined to one thread.
 */
class FakeDaStream(
  initialState: DaStreamPosition,
  private val chunkHasher: (ByteArray) -> ByteArray = { blob -> fakeVersionedHash(blob) },
  private val dataRollingHashCalculator: DataRollingHashCalculator = Keccak256DataRollingHashCalculator,
) : DaStream {
  override val version: DaCodecVersion = DaCodecVersion.FAKE

  /** The blob being filled. [capacity] is smaller than [BLOB_PAYLOAD_CAPACITY] only for a short (flushed) shared blob. */
  private class OpenBlob(
    val capacity: Int,
    val parentDataRollingHash: ByteArray,
    val reproduce: Reproduction? = null,
  ) {
    val payload = ByteArray(capacity)
    var used = 0

    /** Conflation end block and payload offset where its segment ends; turned into [SegmentEnd]s when sealed. */
    val segmentEnds = mutableListOf<Pair<ULong, Int>>()

    fun isReproducing(): Boolean = reproduce != null && used < capacity
  }

  /** The shared blob a resumed stream must reproduce, and the rolling hash it must end at. */
  private class Reproduction(val sharedBlob: ByteArray, val dataRollingHash: ByteArray)

  /** Rolling hash after the last sealed blob. */
  private var dataRollingHash: ByteArray = initialState.dataRollingHash.copyOf()
  private var open: OpenBlob? = null
  private var closed = false

  init {
    if (initialState.sharedBlobCarryOverTailBytes > 0u) {
      open = openInsideSharedBlob(initialState)
    }
  }

  override fun append(conflation: CompressedConflation): List<SealedBlob> {
    checkNotClosed()
    val data = conflation.compressedData
    if (data.isEmpty()) {
      throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "the frame of ${conflation.blockInterval} is empty")
    }
    val segment = ByteBuffer.allocate(SEGMENT_LENGTH_PREFIX_BYTES + data.size).putInt(data.size).put(data).array()
    // Validate before mutating, so a failed append leaves the stream unchanged.
    open?.let { checkReproduces(it, segment) }

    val sealed = mutableListOf<SealedBlob>()
    var written = 0
    while (written < segment.size) {
      var blob = open ?: newBlob()
      if (blob.used == blob.capacity) {
        sealed += seal(blob)
        blob = newBlob()
      }
      open = blob
      val count = minOf(segment.size - written, blob.capacity - blob.used)
      segment.copyInto(
        blob.payload,
        destinationOffset = blob.used,
        startIndex = written,
        endIndex = written + count,
      )
      blob.used += count
      written += count
    }
    open!!.segmentEnds += conflation.blockInterval.endBlockNumber to open!!.used
    return sealed
  }

  override fun flush(): SealedBlob? {
    checkNotClosed()
    val blob = open ?: return null
    if (blob.isReproducing()) {
      throw DaStreamException(
        DaStreamErrorType.INVALID_ARGUMENT,
        "cannot flush while reproducing the shared blob: ${blob.capacity - blob.used} bytes left to reproduce",
      )
    }
    return seal(blob)
  }

  override fun close() {
    closed = true
  }

  private fun newBlob(): OpenBlob = OpenBlob(
    capacity = BLOB_PAYLOAD_CAPACITY,
    parentDataRollingHash = dataRollingHash,
  )

  private fun seal(blob: OpenBlob): SealedBlob {
    val blobBytes = ByteArray(BLOB_BYTES)
    blob.payload.copyInto(blobBytes, endIndex = blob.used)
    blobBytes[blob.used] = BLOB_PAYLOAD_TERMINAL
    val chunkHash = chunkHasher(blobBytes)
    val sealedDataRollingHash = dataRollingHashCalculator.fold(blob.parentDataRollingHash, chunkHash)
    // Invariant: append checks every reproduced byte before writing it, so a re-sealed shared blob equals the original.
    blob.reproduce?.let { reproduction ->
      check(
        blobBytes.contentEquals(reproduction.sharedBlob) &&
          sealedDataRollingHash.contentEquals(reproduction.dataRollingHash),
      ) { "re-sealed shared blob differs from the original" }
    }
    dataRollingHash = sealedDataRollingHash
    open = null
    return SealedBlob(
      blob = blobBytes,
      chunkHash = chunkHash,
      kzgCommitment = fakeKzgCommitment(blobBytes),
      kzgProof = fakeKzgProof(blobBytes),
      parentDataRollingHash = blob.parentDataRollingHash,
      dataRollingHash = sealedDataRollingHash,
      payloadBytes = blob.used.toUInt(),
      segmentEnds = blob.segmentEnds.map { (endBlock, segmentEnd) ->
        SegmentEnd(
          endBlock,
          unusedTailBytes = (blob.used - segmentEnd).toUInt(),
        )
      },
    )
  }

  /** Throws if [segment]'s leading bytes don't reproduce the rest of the shared blob [blob] is resuming. */
  private fun checkReproduces(blob: OpenBlob, segment: ByteArray) {
    val reproduction = blob.reproduce ?: return
    val count = minOf(segment.size, blob.capacity - blob.used)
    for (i in 0 until count) {
      if (segment[i] != reproduction.sharedBlob[blob.used + i]) {
        throw DaStreamException(
          DaStreamErrorType.SEEDED_BLOB_MISMATCH,
          "appended bytes differ from the shared blob at payload offset ${blob.used + i}",
        )
      }
    }
  }

  private fun openInsideSharedBlob(position: DaStreamPosition): OpenBlob {
    val sharedBlob = position.sharedBlob!!
    val parentDataRollingHash = position.parentBlobDataRollingHash
    val payloadBytes = sharedBlob.indexOfLast { it != 0.toByte() }
    if (payloadBytes < 0 || sharedBlob[payloadBytes] != BLOB_PAYLOAD_TERMINAL) {
      throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "shared blob has no payload terminal")
    }
    val carryOver = position.sharedBlobCarryOverTailBytes.toInt()
    if (carryOver >= payloadBytes) {
      throw DaStreamException(
        DaStreamErrorType.INVALID_ARGUMENT,
        "sharedBlobCarryOverTailBytes=${position.sharedBlobCarryOverTailBytes} must be less than the shared " +
          "blob's payload of $payloadBytes bytes",
      )
    }
    val sharedDataRollingHash = dataRollingHashCalculator.fold(parentDataRollingHash, chunkHasher(sharedBlob))
    if (!sharedDataRollingHash.contentEquals(position.dataRollingHash)) {
      throw DaStreamException(
        DaStreamErrorType.INVALID_ARGUMENT,
        "shared blob does not fold from parentBlobDataRollingHash to dataRollingHash",
      )
    }
    return OpenBlob(
      capacity = payloadBytes,
      parentDataRollingHash = parentDataRollingHash,
      reproduce = Reproduction(sharedBlob = sharedBlob, dataRollingHash = position.dataRollingHash),
    ).also { blob ->
      blob.used = payloadBytes - carryOver
      sharedBlob.copyInto(blob.payload, endIndex = blob.used)
    }
  }

  private fun checkNotClosed() {
    if (closed) throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "stream is closed")
  }

  /** Folds a chunk hash into the data rolling hash. */
  fun interface DataRollingHashCalculator {
    fun fold(parentDataRollingHash: ByteArray, chunkHash: ByteArray): ByteArray
  }

  /** DRH_n = keccak256(DRH_{n-1} || chunkHash_n) */
  object Keccak256DataRollingHashCalculator : DataRollingHashCalculator {
    override fun fold(parentDataRollingHash: ByteArray, chunkHash: ByteArray): ByteArray {
      require(parentDataRollingHash.size == 32) {
        "parentDataRollingHash must be 32 bytes, got ${parentDataRollingHash.size}"
      }
      require(chunkHash.size == 32) { "chunkHash must be 32 bytes, got ${chunkHash.size}" }
      return Hash.hash(Bytes.wrap(Bytes.wrap(parentDataRollingHash), Bytes.wrap(chunkHash))).bytes.toArray()
    }
  }

  companion object {
    /** A versioned-hash-shaped stand-in for the KZG versioned hash: `0x01 ‖ keccak256(blob)[1:]`. */
    fun fakeVersionedHash(blob: ByteArray): ByteArray =
      Hash.hash(Bytes.wrap(blob)).bytes.toArray().also { it[0] = 0x01 }

    /** A deterministic [KZG_POINT_BYTES]-byte stand-in for the KZG commitment of [blob]. */
    fun fakeKzgCommitment(blob: ByteArray): ByteArray = fakeKzgPoint(blob, domain = 0x01)

    /** A deterministic [KZG_POINT_BYTES]-byte stand-in for the KZG proof of [blob]. */
    fun fakeKzgProof(blob: ByteArray): ByteArray = fakeKzgPoint(blob, domain = 0x02)

    /** `keccak256(domain ‖ blob) ‖ keccak256(keccak256(domain ‖ blob))[0:16]`, so commitment and proof differ. */
    private fun fakeKzgPoint(blob: ByteArray, domain: Byte): ByteArray {
      val first = Hash.hash(Bytes.concatenate(Bytes.of(domain), Bytes.wrap(blob))).bytes
      val second = Hash.hash(first).bytes
      return Bytes.concatenate(first, second.slice(0, KZG_POINT_BYTES - first.size())).toArray()
    }
  }
}

/** Opens [FakeDaStream]s. */
class FakeDaStreamFactory(
  private val chunkHasher: (ByteArray) -> ByteArray = { blob -> FakeDaStream.fakeVersionedHash(blob) },
) : DaStreamFactory {
  override fun open(version: DaCodecVersion, initialState: DaStreamPosition): DaStream {
    if (version != DaCodecVersion.FAKE) {
      throw DaStreamException(
        DaStreamErrorType.INVALID_ARGUMENT,
        "FakeDaStreamFactory only opens ${DaCodecVersion.FAKE}, got $version",
      )
    }
    return FakeDaStream(initialState, chunkHasher)
  }
}
