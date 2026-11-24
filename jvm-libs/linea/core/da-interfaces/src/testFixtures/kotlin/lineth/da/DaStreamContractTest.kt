package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

/**
 * The stream semantics every [DaStream] has, driven with synthetic opaque frames whose segments have exact sizes:
 * lazy sealing, cuts, segments that span blobs, resuming inside a shared blob and the errors of a stream.
 *
 * A subclass supplies the factory and the codec version, and [expectBlobLayout] for what the codec makes of a blob.
 */
abstract class DaStreamContractTest(
  private val factory: DaStreamFactory,
  private val version: DaCodecVersion,
) {
  /** Checks a sealed blob that carries [payload] of the DA stream; the data rolling hash chain is checked already. */
  protected open fun expectBlobLayout(blob: SealedBlob, payload: ByteArray) {}

  private class Conflation(val frame: ByteArray, val endBlock: ULong)

  private val genesis = ByteArray(32) { 0x11 }
  private val capacity = DaConstants.BLOB_PAYLOAD_CAPACITY

  private fun open(position: DaStreamPosition = DaStreamPosition.fresh(genesis)): DaStream =
    factory.open(version, position)

  /** A frame whose segment, the frame behind its four-byte length prefix, is [segmentLen] bytes. */
  private fun frameOfSegment(segmentLen: Int, seed: Int): ByteArray =
    ByteArray(segmentLen - DaConstants.SEGMENT_LENGTH_PREFIX_BYTES) { i -> (i * 131 + seed * 17 + (i shr 9)).toByte() }

  private fun segmentOf(frame: ByteArray): ByteArray =
    byteArrayOf(
      (frame.size ushr 24).toByte(),
      (frame.size ushr 16).toByte(),
      (frame.size ushr 8).toByte(),
      frame.size.toByte(),
    ) + frame

  /** Conflations of synthetic frames whose segments have the sizes [segmentSizes], ending at blocks 1, 2, ... */
  private fun synthetic(vararg segmentSizes: Int): List<Conflation> =
    segmentSizes.mapIndexed { index, size -> Conflation(frameOfSegment(size, index + 1), (index + 1).toULong()) }

  private fun streamBytes(conflations: List<Conflation>): ByteArray =
    conflations.fold(ByteArray(0)) { bytes, conflation -> bytes + segmentOf(conflation.frame) }

  private fun DaStream.append(conflation: Conflation): List<SealedBlob> =
    append(conflation.frame, conflation.endBlock)

  /** The blobs of a stream opened at [position] that appends [conflations] and flushes. */
  private fun run(position: DaStreamPosition, conflations: List<Conflation>): List<SealedBlob> =
    open(position).use { stream ->
      conflations.flatMap { stream.append(it) } + listOfNotNull(stream.flush())
    }

  /** Every blob is the next one of [streamBytes] folded onto [prev], and together they take all of it. */
  private fun expectBlobs(blobs: List<SealedBlob>, prev: ByteArray, streamBytes: ByteArray) {
    var rest = streamBytes
    var before = prev
    for (blob in blobs) {
      assertThat(blob.prevDataRollingHash).isEqualTo(before)
      val payloadBytes = blob.payloadBytes.toInt()
      assertThat(rest.size).isGreaterThanOrEqualTo(payloadBytes)
      expectBlobLayout(blob, rest.copyOfRange(0, payloadBytes))
      rest = rest.copyOfRange(payloadBytes, rest.size)
      before = blob.dataRollingHash
    }
    assertThat(rest).isEmpty()
  }

  private fun expectSameBlobBytes(expected: SealedBlob, actual: SealedBlob) {
    assertThat(actual.copy(segmentEnds = expected.segmentEnds)).isEqualTo(expected)
  }

  /** The position after a rollup that ended with the conflation ending at [endBlock]. */
  private fun positionAfter(blobs: List<SealedBlob>, endBlock: ULong): DaStreamPosition {
    val blob = blobs.first { candidate -> candidate.segmentEnds.any { it.conflationEndBlock == endBlock } }
    val tail = blob.segmentEnds.first { it.conflationEndBlock == endBlock }.unusedTailBytes
    return if (tail == 0u) {
      DaStreamPosition.fresh(blob.dataRollingHash)
    } else {
      DaStreamPosition(blob.dataRollingHash, blob.prevDataRollingHash, tail, blob.blob)
    }
  }

  private fun expectError(type: DaStreamErrorType, call: () -> Unit) {
    assertThatThrownBy(call)
      .isInstanceOfSatisfying(DaStreamException::class.java) { assertThat(it.errorType).isEqualTo(type) }
  }

  @Test
  fun `a stream has the version it was opened with`() {
    open().use { assertThat(it.version).isEqualTo(version) }
  }

  @Test
  fun `an append leaves a blob with room open, and a flush seals it with the end of the segment`() {
    val conflations = synthetic(1_000)

    open().use { stream ->
      assertThat(stream.append(conflations[0])).isEmpty()
      val blob = stream.flush()!!

      assertThat(blob.payloadBytes).isEqualTo(1_000u)
      assertThat(blob.segmentEnds).containsExactly(SegmentEnd(1UL, 0u))
      expectBlobs(listOf(blob), genesis, streamBytes(conflations))
      assertThat(stream.flush()).isNull()
    }
  }

  @Test
  fun `a full blob is sealed only when the next segment needs room`() {
    val conflations = synthetic(capacity, 500)

    open().use { stream ->
      assertThat(stream.append(conflations[0])).isEmpty()
      val sealed = stream.append(conflations[1])

      assertThat(sealed).hasSize(1)
      assertThat(sealed[0].payloadBytes).isEqualTo(capacity.toUInt())
      assertThat(sealed[0].segmentEnds).containsExactly(SegmentEnd(1UL, 0u))
      val last = stream.flush()!!
      assertThat(last.payloadBytes).isEqualTo(500u)
      assertThat(last.segmentEnds).containsExactly(SegmentEnd(2UL, 0u))
      expectBlobs(sealed + last, genesis, streamBytes(conflations))
    }
  }

  @Test
  fun `a blob that holds several cuts reports all of them in order`() {
    val conflations = synthetic(40_000, 50_000, 60_000)

    open().use { stream ->
      assertThat(stream.append(conflations[0])).isEmpty()
      assertThat(stream.append(conflations[1])).isEmpty()
      val sealed = stream.append(conflations[2])

      assertThat(sealed).hasSize(1)
      assertThat(sealed[0].segmentEnds)
        .containsExactly(SegmentEnd(1UL, (capacity - 40_000).toUInt()), SegmentEnd(2UL, (capacity - 90_000).toUInt()))
      val last = stream.flush()!!
      assertThat(last.segmentEnds).containsExactly(SegmentEnd(3UL, 0u))
      expectBlobs(sealed + last, genesis, streamBytes(conflations))
    }
  }

  @Test
  fun `a segment that spans several blobs leaves the blobs in between without ends`() {
    val conflations = synthetic(10_000, 3 * capacity)

    open().use { stream ->
      assertThat(stream.append(conflations[0])).isEmpty()
      val sealed = stream.append(conflations[1])

      assertThat(sealed).hasSize(3)
      assertThat(sealed[0].segmentEnds).containsExactly(SegmentEnd(1UL, (capacity - 10_000).toUInt()))
      assertThat(sealed[1].segmentEnds).isEmpty()
      assertThat(sealed[2].segmentEnds).isEmpty()
      assertThat(sealed.map { it.payloadBytes }).containsOnly(capacity.toUInt())
      val last = stream.flush()!!
      assertThat(last.segmentEnds).containsExactly(SegmentEnd(2UL, 0u))
      expectBlobs(sealed + last, genesis, streamBytes(conflations))
    }
  }

  @Test
  fun `an empty frame is rejected and the stream goes on`() {
    open().use { stream ->
      expectError(DaStreamErrorType.INVALID_ARGUMENT) { stream.append(ByteArray(0), 1UL) }
      assertThat(stream.flush()).isNull()

      val conflation = synthetic(100)[0]
      assertThat(stream.append(conflation)).isEmpty()
      expectBlobs(listOf(stream.flush()!!), genesis, streamBytes(listOf(conflation)))
    }
  }

  @Test
  fun `a fresh position starts the next blob after its data rolling hash`() {
    val conflations = synthetic(1_000)
    val other = ByteArray(32) { 0x77 }

    val blobs = run(DaStreamPosition.fresh(other), conflations)

    expectBlobs(blobs, other, streamBytes(conflations))
  }

  // The segments are cut so that blob 0 holds A and the start of B, blob 1 the end of B, C and the start of D, and
  // blob 2 the end of D and E.
  private val sizesAToE = intArrayOf(50_000, 100_000, 40_000, 80_000, 30_000)

  @Test
  fun `resuming inside a shared blob reproduces the uninterrupted run byte for byte`() {
    val conflations = synthetic(*sizesAToE)
    val uninterrupted = run(DaStreamPosition.fresh(genesis), conflations)
    assertThat(uninterrupted).hasSize(3)
    expectBlobs(uninterrupted, genesis, streamBytes(conflations))

    // The rollup ended with C, inside blob 1.
    val position = positionAfter(uninterrupted, 3UL)
    assertThat(position.sharedBlobCarryOverTailBytes).isGreaterThan(0u)
    val resumed = run(position, conflations.drop(3))

    assertThat(resumed).hasSize(2)
    resumed.zip(uninterrupted.drop(1)).forEach { (actual, expected) -> expectSameBlobBytes(expected, actual) }
    // The shared blob reports the cuts of the segments appended after the resume: none, D spans into blob 2.
    assertThat(resumed[0].segmentEnds).isEmpty()
    assertThat(resumed[1].segmentEnds).isEqualTo(uninterrupted[2].segmentEnds)
  }

  @Test
  fun `bytes that differ from the shared blob fail with a mismatch and leave the stream unchanged`() {
    val conflations = synthetic(*sizesAToE)
    val uninterrupted = run(DaStreamPosition.fresh(genesis), conflations)
    val position = positionAfter(uninterrupted, 3UL)

    open(position).use { stream ->
      // Another length, so another prefix.
      expectError(DaStreamErrorType.SEEDED_BLOB_MISMATCH) { stream.append(frameOfSegment(80_001, 4), 4UL) }
      // A byte changed in the part the shared blob holds.
      val tampered = conflations[3].frame.copyOf()
      val lastSharedByte = position.sharedBlobCarryOverTailBytes.toInt() - 5
      tampered[lastSharedByte] = (tampered[lastSharedByte].toInt() xor 1).toByte()
      expectError(DaStreamErrorType.SEEDED_BLOB_MISMATCH) { stream.append(tampered, 4UL) }

      val blobs = stream.append(conflations[3]) + stream.append(conflations[4]) + listOfNotNull(stream.flush())

      blobs.zip(uninterrupted.drop(1)).forEach { (actual, expected) -> expectSameBlobBytes(expected, actual) }
    }
  }

  @Test
  fun `a flush is refused until the shared blob is reproduced`() {
    val conflations = synthetic(2_000, 3_000)
    val uninterrupted = run(DaStreamPosition.fresh(genesis), conflations)
    val shared = uninterrupted.single()
    assertThat(shared.payloadBytes).isEqualTo(5_000u)

    open(positionAfter(uninterrupted, 1UL)).use { stream ->
      expectError(DaStreamErrorType.INVALID_ARGUMENT) { stream.flush() }

      assertThat(stream.append(conflations[1])).isEmpty()
      val flushed = stream.flush()!!

      expectSameBlobBytes(shared, flushed)
      assertThat(flushed.segmentEnds).containsExactly(SegmentEnd(2UL, 0u))
    }
  }

  @Test
  fun `a short shared blob is sealed short when the next segment needs room`() {
    val conflations = synthetic(2_000, 3_000, 6_000)
    val shared = run(DaStreamPosition.fresh(genesis), conflations.take(2)).single()

    open(positionAfter(listOf(shared), 1UL)).use { stream ->
      assertThat(stream.append(conflations[1])).isEmpty()
      val sealed = stream.append(conflations[2])

      assertThat(sealed).hasSize(1)
      expectSameBlobBytes(shared, sealed[0])
      assertThat(sealed[0].segmentEnds).containsExactly(SegmentEnd(2UL, 0u))
      val last = stream.flush()!!
      assertThat(last.prevDataRollingHash).isEqualTo(shared.dataRollingHash)
      assertThat(last.payloadBytes).isEqualTo(6_000u)
    }
  }

  @Test
  fun `a stream opens only at a position that its shared blob explains`() {
    val conflations = synthetic(2_000, 3_000)
    val shared = run(DaStreamPosition.fresh(genesis), conflations).single()
    val good = positionAfter(listOf(shared), 1UL)

    expectError(DaStreamErrorType.INVALID_ARGUMENT) {
      open(good.copy(parentBlobDataRollingHash = ByteArray(32)))
    }
    expectError(DaStreamErrorType.INVALID_ARGUMENT) {
      open(good.copy(dataRollingHash = ByteArray(32)))
    }
    expectError(DaStreamErrorType.INVALID_ARGUMENT) {
      open(good.copy(sharedBlobCarryOverTailBytes = shared.payloadBytes))
    }
    open(good.copy(sharedBlobCarryOverTailBytes = shared.payloadBytes - 1u)).close()
    listOf(0xff, 0x00).forEach { fill ->
      expectError(DaStreamErrorType.INVALID_ARGUMENT) {
        open(good.copy(sharedBlob = ByteArray(DaConstants.BLOB_BYTES) { fill.toByte() }))
      }
    }
  }

  @Test
  fun `calls after close are refused`() {
    val stream = open()
    stream.close()

    expectError(DaStreamErrorType.INVALID_ARGUMENT) { stream.append(synthetic(100)[0]) }
    expectError(DaStreamErrorType.INVALID_ARGUMENT) { stream.flush() }
    stream.close()
  }
}
