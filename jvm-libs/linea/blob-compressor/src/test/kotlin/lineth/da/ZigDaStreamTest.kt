package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.util.concurrent.Callable
import java.util.concurrent.Executors

/** The compressor and the stream of the library end to end, over consecutive Linea mainnet blocks. */
class ZigDaStreamTest {
  private val genesis = ByteArray(32) { 0x11 }
  private val firstConflation by lazy { MainnetWindow.blocks(from = 0, count = 2) }
  private val secondConflation by lazy { MainnetWindow.blocks(from = 2, count = 2) }
  private val firstEnd by lazy { MainnetWindow.blockNumber(1) }
  private val secondEnd by lazy { MainnetWindow.blockNumber(3) }

  @BeforeEach
  fun requireInputs() {
    MainnetWindow.assumeAvailable()
  }

  private fun compress(conflation: List<ByteArray>): ByteArray =
    ZigDaFactory.bundled.create(DaCodecVersion.V1, MainnetWindow.chainId)
      .use { it.compress(conflation) }

  private fun segmentSize(frame: ByteArray): Int = DaConstants.SEGMENT_LENGTH_PREFIX_BYTES + frame.size

  private fun sealAll(position: DaStreamPosition, frames: List<Pair<ByteArray, ULong>>): List<SealedBlob> =
    ZigDaFactory.bundled.open(DaCodecVersion.V1, position).use { stream ->
      frames.flatMap { (frame, endBlock) -> stream.append(frame, endBlock) } + listOfNotNull(stream.flush())
    }

  @Test
  fun `two compressed conflations are sealed in a blob that holds their segments and ends at the given blocks`() {
    val first = compress(firstConflation)
    val second = compress(secondConflation)

    val blob = sealAll(DaStreamPosition.fresh(genesis), listOf(first to firstEnd, second to secondEnd)).single()

    assertThat(blob.payloadBytes).isEqualTo((segmentSize(first) + segmentSize(second)).toUInt())
    assertThat(blob.segmentEnds)
      .containsExactly(SegmentEnd(firstEnd, segmentSize(second).toUInt()), SegmentEnd(secondEnd, 0u))
    assertThat(blob.parentDataRollingHash).isEqualTo(genesis)
    listOf(blob.chunkHash, blob.kzgCommitment, blob.kzgProof, blob.dataRollingHash).forEach { bytes ->
      assertThat(bytes).isNotEqualTo(ByteArray(bytes.size))
    }
  }

  @Test
  fun `a stream resumed from the blob of the first conflation reproduces it`() {
    val first = compress(firstConflation)
    val second = compress(secondConflation)
    val blob = sealAll(DaStreamPosition.fresh(genesis), listOf(first to firstEnd, second to secondEnd)).single()
    val position = DaStreamPosition(
      dataRollingHash = blob.dataRollingHash,
      parentBlobDataRollingHash = blob.parentDataRollingHash,
      sharedBlobCarryOverTailBytes = segmentSize(second).toUInt(),
      sharedBlob = blob.blob,
    )

    val resumed = sealAll(position, listOf(second to secondEnd)).single()

    assertThat(resumed).isEqualTo(blob.copy(segmentEnds = listOf(SegmentEnd(secondEnd, 0u))))
  }

  @Test
  fun `streams on separate threads are independent`() {
    val first = compress(firstConflation)
    val second = compress(secondConflation)
    val frames = listOf(first to firstEnd, second to secondEnd)
    val expected = sealAll(DaStreamPosition.fresh(genesis), frames)
    val threads = 4
    val pool = Executors.newFixedThreadPool(threads)
    try {
      val results = pool.invokeAll(
        List(threads) { Callable { sealAll(DaStreamPosition.fresh(genesis), frames) } },
      ).map { it.get() }

      assertThat(results).hasSize(threads).allSatisfy { assertThat(it).isEqualTo(expected) }
    } finally {
      pool.shutdownNow()
    }
  }
}
