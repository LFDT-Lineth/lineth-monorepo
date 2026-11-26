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
import linea.da.DaStreamPosition
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.domain.BlockInterval
import linea.domain.createBlock
import linea.domain.toBesu
import linea.rlp.RLP
import lineth.coordination.riscv.rollup.Keccak256DataRollingHashCalculator
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.nio.file.Path

class FakeDaStreamTest {
  private val genesisDataRollingHash = ByteArray(32) { 0x11 }
  private val compressor = FakeDaCompressor()

  private fun freshStream(): FakeDaStream = FakeDaStream(DaStreamPosition.fresh(genesisDataRollingHash))

  private fun blockRlp(number: ULong, extraDataSize: Int = 32): ByteArray =
    RLP.encodeBlock(createBlock(number = number, extraData = ByteArray(extraDataSize) { 0x2a }).toBesu())

  /** One-block conflation whose segment is exactly [segmentSize] bytes, found by adjusting the block's extraData. */
  private fun blockRlpsWithSegmentSize(number: ULong, segmentSize: Int): List<ByteArray> {
    var extraDataSize = segmentSize
    repeat(10) {
      val rlp = blockRlp(number, extraDataSize)
      val size = SEGMENT_LENGTH_PREFIX_BYTES + rlp.size
      if (size == segmentSize) return listOf(rlp)
      extraDataSize += segmentSize - size
    }
    error("could not build a segment of $segmentSize bytes")
  }

  private fun DaStream.appendAll(vararg conflations: List<ByteArray>): List<SealedBlob> =
    conflations.flatMap { append(compressor.compress(it)) }

  private fun assertBlobLayout(blob: SealedBlob, parentDataRollingHash: ByteArray) {
    val bytes = blob.blob
    assertThat(bytes).hasSize(BLOB_BYTES)
    assertThat(bytes[blob.payloadBytes.toInt()]).isEqualTo(BLOB_PAYLOAD_TERMINAL)
    assertThat(bytes.copyOfRange(blob.payloadBytes.toInt() + 1, BLOB_BYTES)).containsOnly(0)
    assertThat(blob.chunkHash).isEqualTo(FakeDaStream.fakeVersionedHash(bytes))
    assertThat(blob.kzgCommitment).hasSize(KZG_POINT_BYTES).isEqualTo(FakeDaStream.fakeKzgCommitment(bytes))
    assertThat(blob.kzgProof).hasSize(KZG_POINT_BYTES).isEqualTo(FakeDaStream.fakeKzgProof(bytes))
    assertThat(blob.kzgCommitment).isNotEqualTo(blob.kzgProof)
    assertThat(blob.parentDataRollingHash).isEqualTo(parentDataRollingHash)
    assertThat(blob.dataRollingHash)
      .isEqualTo(Keccak256DataRollingHashCalculator.fold(parentDataRollingHash, blob.chunkHash))
  }

  @Test
  fun `compress concatenates the block RLPs into the frame and reports the block interval`() {
    val rlps = listOf(blockRlp(10UL), blockRlp(11UL), blockRlp(12UL))

    val compressed = compressor.compress(rlps)

    assertThat(compressed.blockInterval.startBlockNumber).isEqualTo(10UL)
    assertThat(compressed.blockInterval.endBlockNumber).isEqualTo(12UL)
    assertThat(compressed.compressedData).isEqualTo(rlps.reduce(ByteArray::plus))
  }

  @Test
  fun `compress rejects empty, non-consecutive and undecodable conflations`() {
    assertThatThrownBy { compressor.compress(emptyList()) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_ARGUMENT)
      }
    assertThatThrownBy { compressor.compress(listOf(blockRlp(1UL), blockRlp(3UL))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_ARGUMENT)
      }
    assertThatThrownBy { compressor.compress(listOf(byteArrayOf(0x01, 0x02))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_BLOCK_RLP)
      }
  }

  @Test
  fun `append does not seal a blob that still has room, and flush seals it with the segment's end`() {
    val stream = freshStream()
    val compressed = compressor.compress(listOf(blockRlp(1UL)))

    assertThat(stream.append(compressed)).isEmpty()
    val blob = stream.flush()!!

    assertThat(blob.payloadBytes).isEqualTo((SEGMENT_LENGTH_PREFIX_BYTES + compressed.compressedData.size).toUInt())
    assertThat(blob.segmentEnds).containsExactly(SegmentEnd(1UL, unusedTailBytes = 0u))
    assertBlobLayout(blob, parentDataRollingHash = genesisDataRollingHash)
    assertThat(stream.flush()).isNull()
  }

  @Test
  fun `a full blob is sealed lazily, only when the next segment needs space`() {
    val stream = freshStream()

    assertThat(stream.appendAll(blockRlpsWithSegmentSize(1UL, BLOB_PAYLOAD_CAPACITY))).isEmpty()
    val sealed = stream.appendAll(listOf(blockRlp(2UL)))

    assertThat(sealed).hasSize(1)
    assertThat(sealed.single().payloadBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY.toUInt())
    assertThat(sealed.single().segmentEnds).containsExactly(SegmentEnd(1UL, unusedTailBytes = 0u))
    assertBlobLayout(sealed.single(), parentDataRollingHash = genesisDataRollingHash)
  }

  @Test
  fun `a blob holding several cuts reports them all in order`() {
    val stream = freshStream()
    val first = compressor.compress(blockRlpsWithSegmentSize(1UL, 40_000))
    val second = compressor.compress(blockRlpsWithSegmentSize(2UL, 50_000))
    val third = compressor.compress(blockRlpsWithSegmentSize(3UL, 60_000))

    stream.append(first)
    stream.append(second)
    val sealed = stream.append(third)

    assertThat(sealed).hasSize(1)
    assertThat(sealed.single().segmentEnds).containsExactly(
      SegmentEnd(1UL, unusedTailBytes = (BLOB_PAYLOAD_CAPACITY - 40_000).toUInt()),
      SegmentEnd(2UL, unusedTailBytes = (BLOB_PAYLOAD_CAPACITY - 90_000).toUInt()),
    )
    assertThat(stream.flush()!!.segmentEnds).containsExactly(SegmentEnd(3UL, unusedTailBytes = 0u))
  }

  @Test
  fun `a segment spanning several blobs leaves the middle blobs without segment ends`() {
    val stream = freshStream()
    stream.appendAll(blockRlpsWithSegmentSize(1UL, 10_000))

    val sealed = stream.appendAll(blockRlpsWithSegmentSize(2UL, 3 * BLOB_PAYLOAD_CAPACITY))

    assertThat(sealed).hasSize(3)
    assertThat(
      sealed[0].segmentEnds,
    ).containsExactly(SegmentEnd(1UL, unusedTailBytes = (BLOB_PAYLOAD_CAPACITY - 10_000).toUInt()))
    assertThat(sealed[1].segmentEnds).isEmpty()
    assertThat(sealed[2].segmentEnds).isEmpty()
    assertThat(sealed).allSatisfy { assertThat(it.payloadBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY.toUInt()) }
    assertThat(sealed[1].parentDataRollingHash).isEqualTo(sealed[0].dataRollingHash)
    assertThat(sealed[2].parentDataRollingHash).isEqualTo(sealed[1].dataRollingHash)
    assertThat(stream.flush()!!.segmentEnds).containsExactly(SegmentEnd(2UL, unusedTailBytes = 0u))
  }

  @Test
  fun `compressed conflations are equal by content`() {
    val rlps = listOf(blockRlp(1UL), blockRlp(2UL))
    val first = compressor.compress(rlps)
    val second = compressor.compress(rlps.map { it.copyOf() })

    assertThat(first).isEqualTo(second).hasSameHashCodeAs(second)
    assertThat(first).isNotEqualTo(first.copy(compressedData = first.compressedData + 0x00))
  }

  @Test
  fun `an empty frame and calls on a closed stream are rejected`() {
    val stream = freshStream()
    val empty = CompressedConflation(BlockInterval(1UL, 1UL), compressedData = ByteArray(0))

    assertThatThrownBy { stream.append(empty) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_ARGUMENT)
      }
    assertThat(stream.flush()).isNull()

    stream.close()
    assertThatThrownBy { stream.flush() }.isInstanceOf(DaStreamException::class.java)
  }

  // --- resume inside a shared blob ---

  private val conflationA = blockRlpsWithSegmentSize(1UL, 50_000)
  private val conflationB = blockRlpsWithSegmentSize(2UL, 100_000)
  private val conflationC = blockRlpsWithSegmentSize(3UL, 40_000)
  private val conflationD = blockRlpsWithSegmentSize(4UL, 80_000)
  private val conflationE = blockRlpsWithSegmentSize(5UL, 30_000)

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
      sharedBlobCarryOverTailBytes = cutAfterC.unusedTailBytes,
      sharedBlob = blob2.blob,
      parentBlobDataRollingHash = blob2.parentDataRollingHash,
    )
  }

  @Test
  fun `resuming inside a shared blob reproduces the uninterrupted run byte for byte`() {
    val uninterrupted = uninterruptedRun()
    val resumed = FakeDaStream(positionAfterC(uninterrupted))

    val blobs = resumed.appendAll(conflationD, conflationE) + resumed.flush()!!

    assertThat(blobs).hasSize(2)
    assertThat(blobs.map { it.blob }).containsExactlyElementsOf(uninterrupted.drop(1).map { it.blob })
    assertThat(
      blobs.map { it.dataRollingHash },
    ).containsExactlyElementsOf(uninterrupted.drop(1).map { it.dataRollingHash })
    assertThat(blobs.map { it.kzgCommitment }).containsExactlyElementsOf(uninterrupted.drop(1).map { it.kzgCommitment })
    assertThat(blobs.map { it.kzgProof }).containsExactlyElementsOf(uninterrupted.drop(1).map { it.kzgProof })
    // The re-sealed shared blob only reports cuts of segments appended after the resume: none here, as D spans into blob3.
    assertThat(blobs[0].segmentEnds).isEmpty()
    assertThat(blobs[1].segmentEnds).isEqualTo(uninterrupted[2].segmentEnds)
  }

  @Test
  fun `resuming with different bytes fails with SEEDED_BLOB_MISMATCH and leaves the stream unchanged`() {
    val uninterrupted = uninterruptedRun()
    val resumed = FakeDaStream(positionAfterC(uninterrupted))

    assertThatThrownBy { resumed.append(compressor.compress(blockRlpsWithSegmentSize(4UL, 80_001))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.SEEDED_BLOB_MISMATCH)
      }

    val blobs = resumed.appendAll(conflationD, conflationE) + resumed.flush()!!
    assertThat(blobs.map { it.blob }).containsExactlyElementsOf(uninterrupted.drop(1).map { it.blob })
  }

  @Test
  fun `flush is rejected until the shared blob has been reproduced`() {
    val uninterrupted = uninterruptedRun()
    val resumed = FakeDaStream(positionAfterC(uninterrupted))

    assertThatThrownBy { resumed.flush() }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_ARGUMENT)
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
        sharedBlobCarryOverTailBytes = cutAfterA.unusedTailBytes,
        sharedBlob = flushed.blob,
        parentBlobDataRollingHash = flushed.parentDataRollingHash,
      ),
    )

    // C reproduces the short blob's tail; the next segment needs a new blob, so the short blob is re-sealed.
    assertThat(resumed.appendAll(conflationC)).isEmpty()
    val sealed = resumed.appendAll(conflationD)

    assertThat(sealed).hasSize(1)
    assertThat(sealed.single().blob).isEqualTo(flushed.blob)
    assertThat(sealed.single().payloadBytes).isEqualTo(flushed.payloadBytes)
    assertThat(sealed.single().segmentEnds).containsExactly(SegmentEnd(3UL, unusedTailBytes = 0u))
  }

  @Test
  fun `opening at an inconsistent position is rejected`() {
    val uninterrupted = uninterruptedRun()
    val position = positionAfterC(uninterrupted)

    assertThatThrownBy { FakeDaStream(position.copy(parentBlobDataRollingHash = ByteArray(32))) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_ARGUMENT)
      }
  }

  @Test
  fun `factories create a fake compressor and open a fake stream at the given position`() {
    val compressor = FakeDaCompressorFactory().create(DaCodecVersion.FAKE, Path.of("unused"))
    val stream = FakeDaStreamFactory().open(DaCodecVersion.FAKE, DaStreamPosition.fresh(genesisDataRollingHash))

    stream.append(compressor.compress(listOf(blockRlp(1UL))))

    assertThat(stream.flush()!!.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
  }
}
