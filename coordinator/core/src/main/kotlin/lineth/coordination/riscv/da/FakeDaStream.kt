package lineth.coordination.riscv.da

import linea.da.CompressedConflation
import linea.da.DaCodecVersion
import linea.da.DaConstants.BLOB_BYTES
import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaConstants.BLOB_PAYLOAD_TERMINAL
import linea.da.DaStream
import linea.da.DaStreamException
import linea.da.DaStreamFactory
import linea.da.DaStreamPosition
import linea.da.DaStreamStatus
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.domain.BlockInterval
import linea.domain.DataRollingHashCalculator
import lineth.coordination.riscv.rollup.Keccak256DataRollingHashCalculator
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.ethereum.rlp.RLP
import java.io.ByteArrayOutputStream
import java.nio.ByteBuffer
import java.nio.file.Path

/**
 * Pure-Kotlin [DaStream] with the real stream semantics and simple stand-ins for the codec. Used in tests, and in
 * production to exercise the rollup proof flow until the native `lineth_da` library is available.
 *
 * Real semantics:
 * - frames `[u32 BE len][frame]`, appended in order
 * - blobs of [BLOB_PAYLOAD_CAPACITY] payload bytes, sealed lazily (a full blob is sealed only when the next frame
 *   needs space, or on [flush])
 * - segment ends where each conflation's frame ends, including frames that span blobs
 * - resume inside a shared blob from [DaStreamPosition]: appended bytes must reproduce the shared blob's tail
 *   ([DaStreamStatus.SEEDED_BLOB_MISMATCH] otherwise), and flushing is rejected until they have
 *
 * Stand-ins (the guest will not accept these blobs):
 * - no zstd and no block truncation: the frame body is the concatenated block RLPs
 * - no 254-bit packing: the blob is the payload, the `0xff` terminal, then zero padding
 * - [chunkHasher] instead of a KZG versioned hash
 *
 * Not thread-safe: [append] and [flush] must be confined to one thread. [compress] is pure.
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
    val prevDataRollingHash: ByteArray,
    val reproduce: Reproduction? = null,
  ) {
    val payload = ByteArray(capacity)
    var used = 0
    val segmentEnds = mutableListOf<SegmentEnd>()

    fun isReproducing(): Boolean = reproduce != null && used < capacity
  }

  /** The shared blob a resumed stream must reproduce, and the rolling hash it must end at. */
  private class Reproduction(val sharedBlob: ByteArray, val dataRollingHash: ByteArray)

  /** Rolling hash after the last sealed blob. */
  private var dataRollingHash: ByteArray = initialState.dataRollingHash.copyOf()
  private var open: OpenBlob? = null
  private var closed = false

  init {
    if (initialState.tailBytes > 0) {
      open = openInsideSharedBlob(initialState)
    }
  }

  override fun compress(blockRlps: List<ByteArray>): CompressedConflation {
    checkNotClosed()
    if (blockRlps.isEmpty()) {
      throw DaStreamException(DaStreamStatus.INVALID_ARGUMENT, "a conflation must have at least one block")
    }
    val blockNumbers = blockRlps.map(::decodeBlockNumber)
    blockNumbers.zipWithNext().forEach { (current, next) ->
      if (next != current + 1UL) {
        throw DaStreamException(DaStreamStatus.INVALID_ARGUMENT, "blocks are not consecutive: $current then $next")
      }
    }
    val body = ByteArrayOutputStream().also { out -> blockRlps.forEach(out::writeBytes) }.toByteArray()
    val frame = ByteBuffer.allocate(Int.SIZE_BYTES + body.size).putInt(body.size).put(body).array()
    return FakeCompressedConflation(
      blockInterval = BlockInterval(blockNumbers.first(), blockNumbers.last()),
      frame = frame,
      uncompressedBytes = body.size,
    )
  }

  override fun append(conflation: CompressedConflation): List<SealedBlob> {
    checkNotClosed()
    val frame = conflation as? FakeCompressedConflation
      ?: throw DaStreamException(DaStreamStatus.INVALID_ARGUMENT, "frame was not produced by a FakeDaStream")
    if (frame.consumed) {
      throw DaStreamException(DaStreamStatus.INVALID_ARGUMENT, "frame ${frame.blockInterval} was already appended")
    }
    // Validate before mutating, so a failed append leaves the stream unchanged.
    open?.let { checkReproduces(it, frame.frame) }

    val sealed = mutableListOf<SealedBlob>()
    var written = 0
    while (written < frame.frame.size) {
      var blob = open ?: newBlob()
      if (blob.used == blob.capacity) {
        sealed += seal(blob)
        blob = newBlob()
      }
      open = blob
      val count = minOf(frame.frame.size - written, blob.capacity - blob.used)
      frame.frame.copyInto(
        blob.payload,
        destinationOffset = blob.used,
        startIndex = written,
        endIndex = written + count,
      )
      blob.used += count
      written += count
    }
    open!!.segmentEnds += SegmentEnd(frame.blockInterval.endBlockNumber, open!!.used)
    frame.consumed = true
    return sealed
  }

  override fun flush(): SealedBlob? {
    checkNotClosed()
    val blob = open ?: return null
    if (blob.isReproducing()) {
      throw DaStreamException(
        DaStreamStatus.INVALID_ARGUMENT,
        "cannot flush while reproducing the shared blob: ${blob.capacity - blob.used} bytes left to reproduce",
      )
    }
    return seal(blob)
  }

  override fun close() {
    closed = true
  }

  private fun newBlob(): OpenBlob = OpenBlob(capacity = BLOB_PAYLOAD_CAPACITY, prevDataRollingHash = dataRollingHash)

  private fun seal(blob: OpenBlob): SealedBlob {
    val blobBytes = ByteArray(BLOB_BYTES)
    blob.payload.copyInto(blobBytes, endIndex = blob.used)
    blobBytes[blob.used] = BLOB_PAYLOAD_TERMINAL
    val chunkHash = chunkHasher(blobBytes)
    val sealedDataRollingHash = dataRollingHashCalculator.fold(blob.prevDataRollingHash, chunkHash)
    blob.reproduce?.let { reproduction ->
      if (!blobBytes.contentEquals(reproduction.sharedBlob) ||
        !sealedDataRollingHash.contentEquals(reproduction.dataRollingHash)
      ) {
        throw DaStreamException(DaStreamStatus.SEEDED_BLOB_MISMATCH, "re-sealed shared blob differs from the original")
      }
    }
    dataRollingHash = sealedDataRollingHash
    open = null
    return FakeSealedBlob(
      bytes = blobBytes,
      chunkHash = chunkHash,
      prevDataRollingHash = blob.prevDataRollingHash,
      dataRollingHash = sealedDataRollingHash,
      payloadBytes = blob.used,
      segmentEnds = blob.segmentEnds.toList(),
    )
  }

  /** Throws if [frame]'s leading bytes don't reproduce the rest of the shared blob [blob] is resuming. */
  private fun checkReproduces(blob: OpenBlob, frame: ByteArray) {
    val reproduction = blob.reproduce ?: return
    val count = minOf(frame.size, blob.capacity - blob.used)
    for (i in 0 until count) {
      if (frame[i] != reproduction.sharedBlob[blob.used + i]) {
        throw DaStreamException(
          DaStreamStatus.SEEDED_BLOB_MISMATCH,
          "appended bytes differ from the shared blob at payload offset ${blob.used + i}",
        )
      }
    }
  }

  private fun openInsideSharedBlob(position: DaStreamPosition): OpenBlob {
    val sharedBlob = position.sharedBlob!!
    val prevDataRollingHash = position.prevDataRollingHash!!
    val payloadBytes = sharedBlob.indexOfLast { it != 0.toByte() }
    if (payloadBytes < 0 || sharedBlob[payloadBytes] != BLOB_PAYLOAD_TERMINAL) {
      throw DaStreamException(DaStreamStatus.INVALID_ARGUMENT, "shared blob has no payload terminal")
    }
    if (position.tailBytes >= payloadBytes) {
      throw DaStreamException(
        DaStreamStatus.INVALID_ARGUMENT,
        "tailBytes=${position.tailBytes} must be less than the shared blob's payload of $payloadBytes bytes",
      )
    }
    val sharedDataRollingHash = dataRollingHashCalculator.fold(prevDataRollingHash, chunkHasher(sharedBlob))
    if (!sharedDataRollingHash.contentEquals(position.dataRollingHash)) {
      throw DaStreamException(
        DaStreamStatus.INVALID_ARGUMENT,
        "shared blob does not fold from prevDataRollingHash to dataRollingHash",
      )
    }
    return OpenBlob(
      capacity = payloadBytes,
      prevDataRollingHash = prevDataRollingHash,
      reproduce = Reproduction(sharedBlob = sharedBlob, dataRollingHash = position.dataRollingHash),
    ).also { blob ->
      blob.used = payloadBytes - position.tailBytes
      sharedBlob.copyInto(blob.payload, endIndex = blob.used)
    }
  }

  private fun checkNotClosed() {
    if (closed) throw DaStreamException(DaStreamStatus.INVALID_ARGUMENT, "stream is closed")
  }

  private class FakeCompressedConflation(
    override val blockInterval: BlockInterval,
    val frame: ByteArray,
    override val uncompressedBytes: Int,
  ) : CompressedConflation {
    override val compressedBytes: Int = frame.size
    var consumed = false
  }

  private class FakeSealedBlob(
    private val bytes: ByteArray,
    override val chunkHash: ByteArray,
    override val prevDataRollingHash: ByteArray,
    override val dataRollingHash: ByteArray,
    override val payloadBytes: Int,
    override val segmentEnds: List<SegmentEnd>,
  ) : SealedBlob {
    override val blob: ByteBuffer get() = ByteBuffer.wrap(bytes).asReadOnlyBuffer()
  }

  companion object {
    /** Header field index of the block number: after parentHash, ommersHash, coinbase, 3 roots, bloom, difficulty. */
    private const val HEADER_NUMBER_INDEX = 8

    /** A versioned-hash-shaped stand-in for the KZG versioned hash: `0x01 ‖ keccak256(blob)[1:]`. */
    fun fakeVersionedHash(blob: ByteArray): ByteArray =
      Hash.hash(Bytes.wrap(blob)).bytes.toArray().also { it[0] = 0x01 }

    private fun decodeBlockNumber(blockRlp: ByteArray): ULong {
      return try {
        val input = RLP.input(Bytes.wrap(blockRlp))
        input.enterList()
        input.enterList()
        repeat(HEADER_NUMBER_INDEX) { input.skipNext() }
        input.readLongScalar().toULong()
      } catch (e: Exception) {
        throw DaStreamException(DaStreamStatus.INVALID_BLOCK_RLP, "cannot decode block number: ${e.message}", e)
      }
    }
  }
}

/** Opens [FakeDaStream]s. The dictionary is ignored: the fake does not compress. */
class FakeDaStreamFactory(
  private val chunkHasher: (ByteArray) -> ByteArray = { blob -> FakeDaStream.fakeVersionedHash(blob) },
) : DaStreamFactory {
  override fun open(version: DaCodecVersion, dictionary: Path, initialState: DaStreamPosition): DaStream {
    if (version != DaCodecVersion.FAKE) {
      throw DaStreamException(
        DaStreamStatus.INVALID_ARGUMENT,
        "FakeDaStreamFactory only opens ${DaCodecVersion.FAKE}, got $version",
      )
    }
    return FakeDaStream(initialState, chunkHasher)
  }
}
