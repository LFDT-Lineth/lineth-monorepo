package lineth.conflation.calculators

import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.blobBytes
import lineth.conflation.calculators.DaPipelineTestSupport.Pipeline
import lineth.conflation.calculators.DaPipelineTestSupport.positionAfter
import lineth.conflation.calculators.DaTestConflations.baseTime
import lineth.conflation.calculators.DaTestConflations.conflation
import lineth.conflation.calculators.DaTestConflations.genesisDataRollingHash
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class GlobalRollupCalculatorV2Test {
  // blob1 = [A, start of B], blob2 = [end of B, C, start of D], blob3 = [middle of D], blob4 = [end of D, E].
  // blob3 starts and ends inside D, so it has no cut.
  private val a = conflation(1UL, 3UL, 50_000)
  private val b = conflation(4UL, 7UL, 100_000)
  private val c = conflation(8UL, 9UL, 40_000)
  private val d = conflation(10UL, 14UL, 70_094 + BLOB_PAYLOAD_CAPACITY + 9_906)
  private val e = conflation(15UL, 16UL, 30_000)

  private fun Rollup.ends(): List<ULong> = conflations.map { it.endBlockNumber }

  @Test
  fun `an inclusive trigger ends the rollup inside a blob, and the next rollup starts in that shared blob`() {
    val pipeline = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByConflationCount(3)))

    pipeline.feed(a, b, c, d)
    assertThat(pipeline.rollups).hasSize(1)
    pipeline.feed(e)
    pipeline.blobCalculator.flush()

    val (rollup1, rollup2) = pipeline.rollups
    assertThat(rollup1.ends()).containsExactly(3UL, 7UL, 9UL)
    assertThat(rollup1.da.blobs).hasSize(2)
    assertThat(rollup1.da.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
    assertThat(rollup1.da.parentDataTailTakeBytes).isZero()
    assertThat(rollup1.da.boundaryPrevDataRollingHash).isNull()
    assertThat(rollup1.da.expectedEndDataRollingHash).isEqualTo(rollup1.da.blobs[1].dataRollingHash)
    assertThat(rollup1.da.expectedFinalDataTailDiscardBytes).isEqualTo(70_094)

    assertThat(rollup2.ends()).containsExactly(14UL, 16UL)
    assertThat(rollup2.da.blobs).hasSize(3)
    assertThat(rollup2.da.blobs.first()).isSameAs(rollup1.da.blobs.last())
    assertThat(rollup2.da.blobs[1].segmentEnds).isEmpty()
    assertThat(rollup2.da.parentDataTailTakeBytes).isEqualTo(70_094)
    assertThat(rollup2.da.parentDataRollingHash).isEqualTo(rollup1.da.expectedEndDataRollingHash)
    assertThat(rollup2.da.boundaryPrevDataRollingHash).isEqualTo(rollup1.da.blobs[0].dataRollingHash)
    // A flushed blob ends the rollup at its last cut, which is the end of its payload.
    assertThat(rollup2.da.expectedFinalDataTailDiscardBytes).isZero()
  }

  @Test
  fun `an overflow trigger ends the rollup before the next conflation`() {
    val pipeline = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByDataLimit(120_000UL)))

    pipeline.feed(a, b)

    val rollup = pipeline.rollups.single()
    assertThat(rollup.ends()).containsExactly(3UL)
    assertThat(rollup.da.expectedFinalDataTailDiscardBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY - 50_000)
  }

  @Test
  fun `an oversized conflation forms a rollup on its own`() {
    val pipeline = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByDataLimit(100_000UL)))
    val oversized = conflation(4UL, 6UL, 200_000)
    val small = conflation(7UL, 8UL, 10_000)

    pipeline.feed(a, oversized, small)
    pipeline.blobCalculator.flush()

    assertThat(pipeline.rollups.map { it.ends() }).containsExactly(listOf(3UL), listOf(6UL), listOf(8UL))
  }

  @Test
  fun `a flushed blob ends the rollup at its last cut`() {
    val pipeline = Pipeline(blobTriggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(4UL))))

    pipeline.feed(conflation(1UL, 2UL, 10_000), conflation(3UL, 4UL, 10_000))

    val rollup = pipeline.rollups.single()
    assertThat(rollup.ends()).containsExactly(2UL, 4UL)
    assertThat(rollup.da.blobs.single().payloadBytes).isEqualTo(20_000)
    assertThat(rollup.da.expectedFinalDataTailDiscardBytes).isZero()
  }

  @Test
  fun `blobs filled by a spanning conflation join the open rollup without cutting it`() {
    val pipeline = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByConflationCount(1)))

    pipeline.feed(conflation(1UL, 2UL, 10_000), conflation(3UL, 6UL, 3 * BLOB_PAYLOAD_CAPACITY))
    pipeline.blobCalculator.flush()

    val (first, spanning) = pipeline.rollups
    assertThat(first.ends()).containsExactly(2UL)
    assertThat(spanning.ends()).containsExactly(6UL)
    assertThat(spanning.da.blobs).hasSize(4)
    assertThat(spanning.da.blobs.first()).isSameAs(first.da.blobs.single())
    assertThat(spanning.da.parentDataTailTakeBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY - 10_000)
  }

  @Test
  fun `a blob that starts and ends inside one conflation has no cut and stays in that conflation's rollup`() {
    // blob1 = [A, start of B], blob2 = [middle of B], blob3 = [end of B, C].
    val pipeline = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByDataLimit(50_000UL)))
    val small = conflation(1UL, 2UL, 10_000)
    val spanning = conflation(3UL, 7UL, 2 * BLOB_PAYLOAD_CAPACITY)
    val next = conflation(8UL, 9UL, 5_000)

    pipeline.feed(small, spanning, next)
    pipeline.blobCalculator.flush()

    val (first, middle, last) = pipeline.rollups
    assertThat(pipeline.rollups.map { it.ends() }).containsExactly(listOf(2UL), listOf(7UL), listOf(9UL))
    val (sharedStart, inside, sharedEnd) = middle.da.blobs
    assertThat(sharedStart).isSameAs(first.da.blobs.single())
    assertThat(inside.segmentEnds).isEmpty()
    assertThat(inside.payloadBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY)
    assertThat(sharedEnd).isSameAs(last.da.blobs.single())
    assertThat(middle.da.parentDataTailTakeBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY - 10_000)
    assertThat(middle.da.expectedFinalDataTailDiscardBytes).isEqualTo(5_000)
    assertThat(last.da.parentDataTailTakeBytes).isEqualTo(5_000)
  }

  @Test
  fun `several rollups can end in the same blob`() {
    val pipeline = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByConflationCount(1)))

    pipeline.feed(conflation(1UL, 2UL, 40_000), conflation(3UL, 5UL, 50_000), conflation(6UL, 8UL, 60_000))

    val (first, second) = pipeline.rollups
    assertThat(first.da.blobs.single()).isSameAs(second.da.blobs.single())
    assertThat(first.da.expectedFinalDataTailDiscardBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY - 40_000)
    assertThat(second.da.parentDataTailTakeBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY - 40_000)
    assertThat(second.da.expectedFinalDataTailDiscardBytes).isEqualTo(BLOB_PAYLOAD_CAPACITY - 90_000)
    assertThat(second.da.boundaryPrevDataRollingHash).isEqualTo(genesisDataRollingHash)
  }

  @Test
  fun `the rollup deadline flushes the open blob even before any conflation is sealed`() {
    val pipeline = Pipeline(rollupDeadline = 1.minutes)
    // Blocks 1..3 at baseTime, +20s, +40s: the deadline counts from the first block.
    pipeline.feed(conflation(1UL, 3UL, 10_000, firstTimestamp = baseTime, blockInterval = 20.seconds))

    assertThat(pipeline.rollupCalculator.checkDeadline(baseTime + 59.seconds)).isFalse()
    assertThat(pipeline.rollupCalculator.checkDeadline(baseTime + 60.seconds)).isTrue()

    assertThat(pipeline.rollups.single().ends()).containsExactly(3UL)
  }

  @Test
  fun `the rollup deadline uses the open rollup's oldest conflation and includes the open blob`() {
    val pipeline = Pipeline(rollupDeadline = 10.minutes)
    // The first conflation's blocks 1..3 are at baseTime, +4m, +8m: only its first block is past the deadline at +10m.
    // The second conflation, still in the open blob, is younger still.
    pipeline.feed(
      conflation(1UL, 3UL, 50_000, firstTimestamp = baseTime, blockInterval = 4.minutes),
      conflation(4UL, 6UL, 100_000, firstTimestamp = baseTime + 9.minutes),
    )

    assertThat(pipeline.rollupCalculator.checkDeadline(baseTime + 10.minutes - 1.seconds)).isFalse()
    assertThat(pipeline.rollupCalculator.checkDeadline(baseTime + 10.minutes)).isTrue()

    assertThat(pipeline.rollups.single().ends()).containsExactly(3UL, 6UL)
  }

  @Test
  fun `resuming after a rollup reproduces the uninterrupted next rollup`() {
    val uninterrupted = Pipeline(rollupTriggers = listOf(RollupTriggerCalculatorByConflationCount(3)))
    uninterrupted.feed(a, b, c, d, e)
    uninterrupted.blobCalculator.flush()
    val (rollup1, rollup2) = uninterrupted.rollups

    val resumed = Pipeline(
      position = positionAfter(rollup1),
      rollupTriggers = listOf(RollupTriggerCalculatorByConflationCount(3)),
    )
    assertThat(resumed.blobCalculator.flush()).isFalse() // the shared blob is not re-sealed yet
    resumed.feed(d, e)
    resumed.blobCalculator.flush()

    val resumedRollup2 = resumed.rollups.single()
    assertThat(resumedRollup2.da.blobs).hasSize(3) // the re-sealed shared blob, the blob inside D, and the last
    assertThat(resumedRollup2.conflations).isEqualTo(rollup2.conflations)
    assertThat(
      resumedRollup2.da.blobs.map {
        it.blobBytes()
      },
    ).containsExactlyElementsOf(rollup2.da.blobs.map { it.blobBytes() })
    assertThat(resumedRollup2.da.parentDataRollingHash).isEqualTo(rollup2.da.parentDataRollingHash)
    assertThat(resumedRollup2.da.parentDataTailTakeBytes).isEqualTo(rollup2.da.parentDataTailTakeBytes)
    assertThat(resumedRollup2.da.boundaryPrevDataRollingHash).isEqualTo(rollup2.da.boundaryPrevDataRollingHash)
    assertThat(resumedRollup2.da.expectedEndDataRollingHash).isEqualTo(rollup2.da.expectedEndDataRollingHash)
    assertThat(
      resumedRollup2.da.expectedFinalDataTailDiscardBytes,
    ).isEqualTo(rollup2.da.expectedFinalDataTailDiscardBytes)
  }
}
