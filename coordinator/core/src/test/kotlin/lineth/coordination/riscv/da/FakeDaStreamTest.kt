package lineth.coordination.riscv.da

import linea.da.CompressedConflation
import linea.da.DaConstants.BLOB_BYTES
import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaConstants.BLOB_PAYLOAD_TERMINAL
import linea.da.DaStream
import linea.da.DaStreamException
import linea.da.DaStreamPosition
import linea.da.DaStreamStatus
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.da.blobBytes
import linea.domain.createBlock
import linea.domain.toBesu
import linea.rlp.RLP
import lineth.coordination.riscv.rollup.Keccak256DataRollingHashCalculator
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class FakeDaStreamTest {
  private val genesisDataRollingHash = ByteArray(32) { 0x11 }

  private fun freshStream(): FakeDaStream = FakeDaStream(DaStreamPosition(genesisDataRollingHash))

  private fun blockRlp(number: ULong, extraDataSize: Int = 32): ByteArray =
    RLP.encodeBlock(createBlock(number = number, extraData = ByteArray(extraDataSize) { 0x2a }).toBesu())

  /** One-block conflation whose frame is exactly [frameSize] bytes, found by adjusting the block's extraData. */
  private fun blockRlpsWithFrameSize(number: ULong, frameSize: Int): List<ByteArray> {
    var extraDataSize = frameSize
    repeat(10) {
      val rlp = blockRlp(number, extraDataSize)
      val size = Int.SIZE_BYTES + rlp.size
      if (size == frameSize) return listOf(rlp)
      extraDataSize += frameSize - size
    }
    error("could not build a frame of $frameSize bytes")
  }

  private fun DaStream.appendAll(vararg conflations: List<ByteArray>): List<SealedBlob> =
    conflations.flatMap { append(compress(it)) }

  private fun assertBlobLayout(blob: SealedBlob, prevDataRollingHash: ByteArray) {
    val bytes = blob.blobBytes()
    assertThat(bytes).hasSize(BLOB_BYTES)
    assertThat(bytes[blob.payloadBytes]).isEqualTo(BLOB_PAYLOAD_TERMINAL)
    assertThat(bytes.copyOfRange(blob.payloadBytes + 1, BLOB_BYTES)).containsOnly(0)
    assertThat(blob.chunkHash).isEqualTo(FakeDaStream.fakeVersionedHash(bytes))
    assertThat(blob.prevDataRollingHash).isEqualTo(prevDataRollingHash)
    assertThat(blob.dataRollingHash)
      .isEqualTo(Keccak256DataRollingHashCalculator.fold(prevDataRollingHash, blob.chunkHash))
  }

  @Test
  fun `compress frames the concatenated block RLPs and reports the block interval`() {
    val rlps = listOf(blockRlp(10UL), blockRlp(11UL), blockRlp(12UL))

    val compressed = freshStream().compress(rlps)

    assertThat(compressed.blockInterval.startBlockNumber).isEqualTo(10UL)
    assertThat(compressed.blockInterval.endBlockNumber).isEqualTo(12UL)
    assertThat(compressed.uncompressedBytes).isEqualTo(rlps.sumOf { it.size })
    assertThat(compressed.compressedBytes).isEqualTo(Int.SIZE_BYTES + compressed.uncompressedBytes)
  }

  @Test
  fun `compress rejects empty, non-consecutive and undecodable conflations`() {
    val stream = freshStream()

    assertThatThrownBy { stream.compress(emptyList()) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.INVALID_ARGUMENT)
      }
    assertThatThrownBy { stream.compress(listOf(blockRlp(1UL), blockRlp(3UL))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.INVALID_ARGUMENT)
      }
    assertThatThrownBy { stream.compress(listOf(byteArrayOf(0x01, 0x02))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.INVALID_BLOCK_RLP)
      }
  }

  @Test
  fun `append does not seal a blob that still has room, and flush seals it with the frame's end`() {
    val stream = freshStream()
    val compressed = stream.compress(listOf(blockRlp(1UL)))

    assertThat(stream.append(compressed)).isEmpty()
    val blob = stream.flush()!!

    assertThat(blob.payloadBytes).isEqualTo(compressed.compressedBytes)
    assertThat(blob.segmentEnds).containsExactly(SegmentEnd(1UL, compressed.compressedBytes))
    assertBlobLayout(blob, prevDataRollingHash = genesisDataRollingHash)
    assertThat(stream.flush()).isNull()
  }

  @Test
  fun `a full blob is sealed lazily, only when the next frame needs space`() {
    val stream = freshStream()

    assertThat(stream.appendAll(blockRlpsWithFrameSize(1UL, BLOB_PAYLOAD_CAPACITY))).isEmpty()
    val sealed = stream.appendAll(listOf(blockRlp(2UL)))

    assertThat(sealed).hasSize(1)
    assertThat(sealed.single().payloadBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY)
    assertThat(sealed.single().segmentEnds).containsExactly(SegmentEnd(1UL, BLOB_PAYLOAD_CAPACITY))
    assertBlobLayout(sealed.single(), prevDataRollingHash = genesisDataRollingHash)
  }

  @Test
  fun `a blob holding several cuts reports them all in order`() {
    val stream = freshStream()
    val first = stream.compress(blockRlpsWithFrameSize(1UL, 40_000))
    val second = stream.compress(blockRlpsWithFrameSize(2UL, 50_000))
    val third = stream.compress(blockRlpsWithFrameSize(3UL, 60_000))

    stream.append(first)
    stream.append(second)
    val sealed = stream.append(third)

    assertThat(sealed).hasSize(1)
    assertThat(sealed.single().segmentEnds).containsExactly(SegmentEnd(1UL, 40_000), SegmentEnd(2UL, 90_000))
    assertThat(stream.flush()!!.segmentEnds).containsExactly(SegmentEnd(3UL, 90_000 + 60_000 - BLOB_PAYLOAD_CAPACITY))
  }

  @Test
  fun `a frame spanning several blobs leaves the middle blobs without segment ends`() {
    val stream = freshStream()
    stream.appendAll(blockRlpsWithFrameSize(1UL, 10_000))

    val sealed = stream.appendAll(blockRlpsWithFrameSize(2UL, 3 * BLOB_PAYLOAD_CAPACITY))

    assertThat(sealed).hasSize(3)
    assertThat(sealed[0].segmentEnds).containsExactly(SegmentEnd(1UL, 10_000))
    assertThat(sealed[1].segmentEnds).isEmpty()
    assertThat(sealed[2].segmentEnds).isEmpty()
    assertThat(sealed).allSatisfy { assertThat(it.payloadBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY) }
    assertThat(sealed[1].prevDataRollingHash).isEqualTo(sealed[0].dataRollingHash)
    assertThat(sealed[2].prevDataRollingHash).isEqualTo(sealed[1].dataRollingHash)
    assertThat(stream.flush()!!.segmentEnds).containsExactly(SegmentEnd(2UL, 10_000))
  }

  @Test
  fun `a frame cannot be appended twice and a closed stream rejects calls`() {
    val stream = freshStream()
    val compressed = stream.compress(listOf(blockRlp(1UL)))
    stream.append(compressed)

    assertThatThrownBy { stream.append(compressed) }.isInstanceOf(DaStreamException::class.java)

    stream.close()
    assertThatThrownBy { stream.flush() }.isInstanceOf(DaStreamException::class.java)
  }

  // --- resume inside a shared blob ---

  private val conflationA = blockRlpsWithFrameSize(1UL, 50_000)
  private val conflationB = blockRlpsWithFrameSize(2UL, 100_000)
  private val conflationC = blockRlpsWithFrameSize(3UL, 40_000)
  private val conflationD = blockRlpsWithFrameSize(4UL, 80_000)
  private val conflationE = blockRlpsWithFrameSize(5UL, 30_000)

  /** blob1 = [A, start of B], blob2 = [end of B, C, start of D], blob3 = [end of D, E] (flushed). */
  private fun uninterruptedRun(): List<SealedBlob> {
    val stream = freshStream()
    val sealed = stream.appendAll(conflationA, conflationB, conflationC, conflationD, conflationE)
    return sealed + stream.flush()!!
  }

  /** The stream position after a rollup ending at C's cut inside blob2. */
  private fun positionAfterC(blobs: List<SealedBlob>): DaStreamPosition {
    val blob2 = blobs[1]
    val cutAfterC = blob2.segmentEnds.single { it.conflationEndBlock == 3UL }
    return DaStreamPosition(
      dataRollingHash = blob2.dataRollingHash,
      tailBytes = blob2.payloadBytes - cutAfterC.offset,
      sharedBlob = blob2.blobBytes(),
      prevDataRollingHash = blob2.prevDataRollingHash,
    )
  }

  @Test
  fun `resuming inside a shared blob reproduces the uninterrupted run byte for byte`() {
    val uninterrupted = uninterruptedRun()
    val resumed = FakeDaStream(positionAfterC(uninterrupted))

    val blobs = resumed.appendAll(conflationD, conflationE) + resumed.flush()!!

    assertThat(blobs).hasSize(2)
    assertThat(blobs.map { it.blobBytes() }).containsExactlyElementsOf(uninterrupted.drop(1).map { it.blobBytes() })
    assertThat(
      blobs.map { it.dataRollingHash },
    ).containsExactlyElementsOf(uninterrupted.drop(1).map { it.dataRollingHash })
    // The re-sealed shared blob only reports cuts of frames appended after the resume: none here, as D spans into blob3.
    assertThat(blobs[0].segmentEnds).isEmpty()
    assertThat(blobs[1].segmentEnds).isEqualTo(uninterrupted[2].segmentEnds)
  }

  @Test
  fun `resuming with different bytes fails with SEEDED_BLOB_MISMATCH and leaves the stream unchanged`() {
    val uninterrupted = uninterruptedRun()
    val resumed = FakeDaStream(positionAfterC(uninterrupted))

    assertThatThrownBy { resumed.append(resumed.compress(blockRlpsWithFrameSize(4UL, 80_001))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.SEEDED_BLOB_MISMATCH)
      }

    val blobs = resumed.appendAll(conflationD, conflationE) + resumed.flush()!!
    assertThat(blobs.map { it.blobBytes() }).containsExactlyElementsOf(uninterrupted.drop(1).map { it.blobBytes() })
  }

  @Test
  fun `flush is rejected until the shared blob has been reproduced`() {
    val uninterrupted = uninterruptedRun()
    val resumed = FakeDaStream(positionAfterC(uninterrupted))

    assertThatThrownBy { resumed.flush() }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.INVALID_ARGUMENT)
      }
  }

  @Test
  fun `resuming inside a short flushed shared blob re-seals it short`() {
    val stream = freshStream()
    stream.appendAll(conflationA, conflationC)
    val flushed = stream.flush()!!
    val cutAfterA = flushed.segmentEnds.first()
    val resumed = FakeDaStream(
      DaStreamPosition(
        dataRollingHash = flushed.dataRollingHash,
        tailBytes = flushed.payloadBytes - cutAfterA.offset,
        sharedBlob = flushed.blobBytes(),
        prevDataRollingHash = flushed.prevDataRollingHash,
      ),
    )

    // C reproduces the short blob's tail; the next frame needs a new blob, so the short blob is re-sealed.
    assertThat(resumed.appendAll(conflationC)).isEmpty()
    val sealed = resumed.appendAll(conflationD)

    assertThat(sealed).hasSize(1)
    assertThat(sealed.single().blobBytes()).isEqualTo(flushed.blobBytes())
    assertThat(sealed.single().payloadBytes).isEqualTo(flushed.payloadBytes)
    assertThat(sealed.single().segmentEnds).containsExactly(SegmentEnd(3UL, flushed.payloadBytes))
  }

  @Test
  fun `opening at an inconsistent position is rejected`() {
    val uninterrupted = uninterruptedRun()
    val position = positionAfterC(uninterrupted)

    assertThatThrownBy { FakeDaStream(position.copy(prevDataRollingHash = ByteArray(32))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.INVALID_ARGUMENT)
      }
  }

  @Test
  fun `factory opens a fake stream at the given position`() {
    val stream = FakeDaStreamFactory().open(
      linea.da.DaCodecVersion.FAKE,
      java.nio.file.Path.of("unused"),
      DaStreamPosition(genesisDataRollingHash),
    )
    val compressed: CompressedConflation = stream.compress(listOf(blockRlp(1UL)))
    stream.append(compressed)

    assertThat(stream.flush()!!.prevDataRollingHash).isEqualTo(genesisDataRollingHash)
  }
}
