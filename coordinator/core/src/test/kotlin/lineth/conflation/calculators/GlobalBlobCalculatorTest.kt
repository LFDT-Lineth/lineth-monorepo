package lineth.conflation.calculators

import linea.da.CompressedConflation
import linea.da.DaCodecVersion
import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaConstants.KZG_POINT_BYTES
import linea.da.DaStream
import linea.da.DaStreamException
import linea.da.DaStreamPosition
import linea.da.DaStreamStatus
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.da.blobBytes
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.createBlock
import linea.domain.toBesu
import linea.rlp.RLP
import lineth.coordination.riscv.da.FakeDaStream
import lineth.encoding.BlockEncoder
import net.consensys.linea.traces.TracesCountersV5
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.nio.ByteBuffer
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class GlobalBlobCalculatorTest {
  private val encoder = BlockEncoder { RLP.encodeBlock(it.toBesu()) }
  private val baseTime = Instant.parse("2026-10-01T00:00:00Z")
  private val genesisDataRollingHash = ByteArray(32) { 0x11 }
  private val events = mutableListOf<BlobEvent>()

  /** One-block conflation whose DA frame is exactly [frameSize] bytes. */
  private fun conflation(
    number: ULong,
    frameSize: Int,
    timestamp: Instant = baseTime + (number.toLong() * 2).seconds,
  ): BlocksConflation = conflation(number, number, frameSize, timestamp)

  /**
   * Conflation of blocks [startNumber]..[endNumber], 2 seconds apart from [firstTimestamp], whose DA frame is exactly
   * [frameSize] bytes. The last block's extraData pads the frame.
   */
  private fun conflation(
    startNumber: ULong,
    endNumber: ULong,
    frameSize: Int,
    firstTimestamp: Instant = baseTime + (startNumber.toLong() * 2).seconds,
  ): BlocksConflation {
    fun timestamp(number: ULong) = firstTimestamp + ((number - startNumber).toLong() * 2).seconds
    val leading = (startNumber until endNumber).map { createBlock(number = it, timestamp = timestamp(it)) }
    val leadingBytes = Int.SIZE_BYTES + leading.sumOf { encoder.encode(it).size }
    var extraDataSize = frameSize - leadingBytes
    repeat(10) {
      val last = createBlock(
        number = endNumber,
        timestamp = timestamp(endNumber),
        extraData = ByteArray(extraDataSize) { 0x2a },
      )
      val size = leadingBytes + encoder.encode(last).size
      if (size == frameSize) {
        return BlocksConflation(
          leading + last,
          ConflationCalculationResult(
            startNumber,
            endNumber,
            ConflationTrigger.BLOCKS_LIMIT,
            TracesCountersV5.EMPTY_TRACES_COUNT,
          ),
        )
      }
      extraDataSize += frameSize - size
    }
    error("could not build a frame of $frameSize bytes")
  }

  private fun calculator(
    stream: DaStream = FakeDaStream(DaStreamPosition(genesisDataRollingHash)),
    triggers: List<BlobTriggerCalculator> = emptyList(),
    blobDeadline: kotlin.time.Duration? = null,
    sharedBlobTailBytes: Int = 0,
  ) = GlobalBlobCalculator(stream, triggers, encoder, { events += it }, blobDeadline, sharedBlobTailBytes)

  private fun BlobEvent.conflationEnds(): List<ULong> = conflations.map { it.conflation.conflation.endBlockNumber }

  // A = 50k, B = 100k, C = 40k, D = 80k: blob1 = [A, start of B], blob2 = [end of B, C, start of D], blob3 = [end of D].
  private val a = conflation(1UL, 50_000)
  private val b = conflation(2UL, 100_000)
  private val c = conflation(3UL, 40_000)
  private val d = conflation(4UL, 80_000)

  @Test
  fun `pairs each sealed blob's cut points with the oldest queued conflations and reports next`() {
    val calculator = calculator()

    calculator.newConflation(a)
    assertThat(events).isEmpty()
    calculator.newConflation(b)
    assertThat(events).hasSize(1)
    calculator.newConflation(c)
    assertThat(events).hasSize(1)
    calculator.newConflation(d)

    assertThat(events).hasSize(2)
    assertThat(events[0].conflationEnds()).containsExactly(1UL)
    assertThat(
      events[0].conflations.single().cut,
    ).isEqualTo(SegmentEnd(1UL, tailBytes = BLOB_PAYLOAD_CAPACITY - 50_000))
    assertThat(events[0].next!!.conflation).isEqualTo(b)
    assertThat(events[0].flushed).isFalse()
    assertThat(events[1].conflationEnds()).containsExactly(2UL, 3UL)
    assertThat(events[1].conflations.map { it.cut.tailBytes })
      .containsExactly(BLOB_PAYLOAD_CAPACITY - 19_953, BLOB_PAYLOAD_CAPACITY - 59_953)
    assertThat(events[1].next!!.conflation).isEqualTo(d)
    assertThat(events[1].conflations.first().conflation.compressed.compressedBytes).isEqualTo(100_000)
  }

  @Test
  fun `a conflation spanning blobs produces events without conflations, then ends in a later blob`() {
    val calculator = calculator()
    val spanning = conflation(2UL, 3 * BLOB_PAYLOAD_CAPACITY)

    calculator.newConflation(conflation(1UL, 10_000))
    calculator.newConflation(spanning)
    assertThat(calculator.flush()).isTrue()

    assertThat(events.map { it.conflationEnds() }).containsExactly(listOf(1UL), emptyList(), emptyList(), listOf(2UL))
    assertThat(events.take(3).map { it.next!!.conflation }).containsOnly(spanning)
    assertThat(events.last().flushed).isTrue()
    assertThat(events.last().next).isNull()
  }

  @Test
  fun `a target block number flushes the blob right after that conflation`() {
    val calculator = calculator(triggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(2UL))))

    calculator.newConflation(conflation(1UL, 10_000))
    assertThat(events).isEmpty()
    calculator.newConflation(conflation(2UL, 10_000))

    assertThat(events).hasSize(1)
    assertThat(events.single().conflationEnds()).containsExactly(1UL, 2UL)
    assertThat(events.single().flushed).isTrue()
    assertThat(events.single().next).isNull()
  }

  @Test
  fun `a hard fork flushes the blob before the first conflation after the fork`() {
    val fork = baseTime + 1.minutes
    val calculator = calculator(
      triggers = listOf(BlobTriggerCalculatorByHardForkTimestamp(listOf(fork), initialTimestamp = baseTime)),
    )

    calculator.newConflation(conflation(1UL, 10_000, timestamp = fork - 10.seconds))
    calculator.newConflation(conflation(2UL, 10_000, timestamp = fork - 5.seconds))
    assertThat(events).isEmpty()
    calculator.newConflation(conflation(3UL, 10_000, timestamp = fork))

    assertThat(events).hasSize(1)
    assertThat(events.single().conflationEnds()).containsExactly(1UL, 2UL)
    assertThat(events.single().flushed).isTrue()
    assertThat(calculator.flush()).isTrue()
    assertThat(events.last().conflationEnds()).containsExactly(3UL)
  }

  @Test
  fun `the blob deadline flushes once the oldest unsealed conflation is old enough`() {
    val calculator = calculator(blobDeadline = 1.minutes)
    val first = conflation(1UL, 10_000, timestamp = baseTime)
    calculator.newConflation(first)

    assertThat(calculator.checkDeadline(baseTime + 59.seconds)).isFalse()
    assertThat(events).isEmpty()
    assertThat(calculator.checkDeadline(baseTime + 60.seconds)).isTrue()
    assertThat(events.single().conflationEnds()).containsExactly(1UL)
    assertThat(calculator.checkDeadline(baseTime + 10.minutes)).isFalse()
  }

  @Test
  fun `multi-block conflations are paired with cut points by their end block`() {
    // Same layout as a, b, c, d: blob1 = [1..3, start of 4..7], blob2 = [end of 4..7, 8..9, start of 10..12].
    val first = conflation(1UL, 3UL, 50_000)
    val second = conflation(4UL, 7UL, 100_000)
    val third = conflation(8UL, 9UL, 40_000)
    val fourth = conflation(10UL, 12UL, 80_000)

    calculator().apply { listOf(first, second, third, fourth).forEach(::newConflation) }

    assertThat(events).hasSize(2)
    assertThat(events[0].conflations.single().conflation.conflation).isEqualTo(first)
    assertThat(
      events[0].conflations.single().cut,
    ).isEqualTo(SegmentEnd(3UL, tailBytes = BLOB_PAYLOAD_CAPACITY - 50_000))
    assertThat(events[0].next!!.conflation).isEqualTo(second)
    assertThat(events[1].conflations.map { it.conflation.conflation }).containsExactly(second, third)
    assertThat(events[1].conflations.map { it.cut.conflationEndBlock }).containsExactly(7UL, 9UL)
    assertThat(events[1].next!!.conflation).isEqualTo(fourth)
  }

  @Test
  fun `the blob deadline counts from the oldest conflation's first block, not its last`() {
    val calculator = calculator(blobDeadline = 1.minutes)
    // Blocks 1..5 at baseTime, +2s, ..., +8s.
    calculator.newConflation(conflation(1UL, 5UL, 10_000, firstTimestamp = baseTime))

    assertThat(calculator.checkDeadline(baseTime + 59.seconds)).isFalse()
    assertThat(calculator.checkDeadline(baseTime + 60.seconds)).isTrue()
    assertThat(events.single().conflationEnds()).containsExactly(5UL)
  }

  @Test
  fun `a target block number flushes only when a multi-block conflation ends at it`() {
    val calculator = calculator(triggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(5UL, 7UL))))

    // 5 is inside 4..6: no flush.
    calculator.newConflation(conflation(1UL, 3UL, 10_000))
    calculator.newConflation(conflation(4UL, 6UL, 10_000))
    assertThat(events).isEmpty()
    // 7 ends 7..7: the blob is flushed right after it.
    calculator.newConflation(conflation(7UL, 7UL, 10_000))

    assertThat(events.single().conflationEnds()).containsExactly(3UL, 6UL, 7UL)
    assertThat(events.single().flushed).isTrue()
  }

  @Test
  fun `a hard fork compares the last block of the previous conflation with the first block of the next`() {
    val fork = baseTime + 1.minutes
    val calculator = calculator(
      triggers = listOf(BlobTriggerCalculatorByHardForkTimestamp(listOf(fork), initialTimestamp = baseTime)),
    )

    // Blocks 1..3 at fork-20s, -18s, -16s; blocks 4..6 at fork-6s, -4s, -2s: no fork between them.
    calculator.newConflation(conflation(1UL, 3UL, 10_000, firstTimestamp = fork - 20.seconds))
    calculator.newConflation(conflation(4UL, 6UL, 10_000, firstTimestamp = fork - 6.seconds))
    assertThat(events).isEmpty()
    // Blocks 7..9 start at the fork: the open blob is flushed before them.
    calculator.newConflation(conflation(7UL, 9UL, 10_000, firstTimestamp = fork))

    assertThat(events.single().conflationEnds()).containsExactly(3UL, 6UL)
    assertThat(events.single().flushed).isTrue()
    assertThat(calculator.flush()).isTrue()
    assertThat(events.last().conflationEnds()).containsExactly(9UL)
  }

  @Test
  fun `flush with no open blob does nothing`() {
    val calculator = calculator()

    assertThat(calculator.flush()).isFalse()
    assertThat(events).isEmpty()
  }

  @Test
  fun `after resuming inside a shared blob, flushing is skipped until the shared blob is re-sealed`() {
    calculator().apply { listOf(a, b, c, d).forEach(::newConflation) }
    val blob2 = events[1].blob
    val cutAfterC = events[1].conflations.last().cut
    val position = DaStreamPosition(
      dataRollingHash = blob2.dataRollingHash,
      tailBytes = cutAfterC.tailBytes,
      sharedBlob = blob2.blobBytes(),
      prevDataRollingHash = blob2.prevDataRollingHash,
    )
    events.clear()
    val resumed = calculator(
      stream = FakeDaStream(position),
      sharedBlobTailBytes = position.tailBytes,
      blobDeadline = 1.seconds,
    )

    assertThat(resumed.flush()).isFalse()
    assertThat(resumed.checkDeadline(baseTime + 1000.minutes)).isFalse()
    resumed.newConflation(d)

    assertThat(events).hasSize(1)
    assertThat(events.single().blob.blobBytes()).isEqualTo(blob2.blobBytes())
    assertThat(events.single().conflations).isEmpty()
    assertThat(events.single().next!!.conflation).isEqualTo(d)
    assertThat(resumed.flush()).isTrue()
    assertThat(events.last().conflationEnds()).containsExactly(4UL)
  }

  /** Seals a short (flushed) blob [a, short], and returns the stream position after a's cut inside it. */
  private fun positionInsideShortFlushedBlob(short: BlocksConflation): Pair<DaStreamPosition, BlobEvent> {
    calculator().apply {
      newConflation(a)
      newConflation(short)
      flush()
    }
    val flushed = events.single()
    events.clear()
    val position = DaStreamPosition(
      dataRollingHash = flushed.blob.dataRollingHash,
      tailBytes = flushed.conflations.first().cut.tailBytes,
      sharedBlob = flushed.blob.blobBytes(),
      prevDataRollingHash = flushed.blob.prevDataRollingHash,
    )
    return position to flushed
  }

  @Test
  fun `once a resumed short shared blob is reproduced in full, flush re-seals it without waiting for more data`() {
    val short = conflation(2UL, 30_000)
    val (position, original) = positionInsideShortFlushedBlob(short)
    val resumed = calculator(stream = FakeDaStream(position), sharedBlobTailBytes = position.tailBytes)

    assertThat(resumed.flush()).isFalse()
    // The frame fills the shared blob exactly, so append does not seal it: the blob stays open until flushed.
    resumed.newConflation(short)
    assertThat(events).isEmpty()

    assertThat(resumed.flush()).isTrue()
    assertThat(events.single().blob.blobBytes()).isEqualTo(original.blob.blobBytes())
    assertThat(events.single().flushed).isTrue()
    assertThat(events.single().conflationEnds()).containsExactly(2UL)
  }

  @Test
  fun `once a resumed short shared blob is reproduced in full, the blob deadline re-seals it`() {
    val short = conflation(2UL, 30_000)
    val (position, original) = positionInsideShortFlushedBlob(short)
    val resumed = calculator(
      stream = FakeDaStream(position),
      sharedBlobTailBytes = position.tailBytes,
      blobDeadline = 1.seconds,
    )

    resumed.newConflation(short)

    assertThat(resumed.checkDeadline(baseTime + 1000.minutes)).isTrue()
    assertThat(events.single().blob.blobBytes()).isEqualTo(original.blob.blobBytes())
  }

  @Test
  fun `a failed append leaves the conflation unqueued`() {
    calculator().apply { listOf(a, b, c, d).forEach(::newConflation) }
    val blob2 = events[1].blob
    val position = DaStreamPosition(
      dataRollingHash = blob2.dataRollingHash,
      tailBytes = events[1].conflations.last().cut.tailBytes,
      sharedBlob = blob2.blobBytes(),
      prevDataRollingHash = blob2.prevDataRollingHash,
    )
    events.clear()
    val resumed = calculator(stream = FakeDaStream(position), sharedBlobTailBytes = position.tailBytes)

    assertThatThrownBy { resumed.newConflation(conflation(4UL, 80_001)) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.status).isEqualTo(DaStreamStatus.SEEDED_BLOB_MISMATCH)
      }
    resumed.newConflation(d)
    resumed.flush()

    assertThat(events.last().conflationEnds()).containsExactly(4UL)
  }

  @Test
  fun `a cut that does not match the oldest queued conflation fails`() {
    val stream = ScriptedDaStream(sealOnAppend = listOf(blobWithCuts(SegmentEnd(99UL, 10))))
    val calculator = calculator(stream = stream)

    assertThatThrownBy { calculator.newConflation(conflation(1UL, 10_000)) }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("does not match the oldest queued conflation")
  }

  @Test
  fun `a flushed blob without conflations fails`() {
    val stream = ScriptedDaStream(flushResult = blobWithCuts())
    val calculator = calculator(stream = stream)

    assertThatThrownBy { calculator.flush() }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("a flushed blob must end at least one conflation")
  }

  private fun blobWithCuts(vararg cuts: SegmentEnd): SealedBlob = object : SealedBlob {
    override val blob: ByteBuffer = ByteBuffer.allocate(0)
    override val chunkHash = ByteArray(32)
    override val kzgCommitment = ByteArray(KZG_POINT_BYTES)
    override val kzgProof = ByteArray(KZG_POINT_BYTES)
    override val prevDataRollingHash = ByteArray(32)
    override val dataRollingHash = ByteArray(32)
    override val payloadBytes = BLOB_PAYLOAD_CAPACITY
    override val segmentEnds = cuts.toList()
  }

  /** Compresses like [FakeDaStream], but returns scripted blobs from append and flush. */
  private inner class ScriptedDaStream(
    private val sealOnAppend: List<SealedBlob> = emptyList(),
    private val flushResult: SealedBlob? = null,
  ) : DaStream {
    private val delegate = FakeDaStream(DaStreamPosition(genesisDataRollingHash))
    override val version = DaCodecVersion.FAKE
    override fun compress(blockRlps: List<ByteArray>): CompressedConflation = delegate.compress(blockRlps)
    override fun append(conflation: CompressedConflation): List<SealedBlob> = sealOnAppend
    override fun flush(): SealedBlob? = flushResult
    override fun close() = Unit
  }
}
