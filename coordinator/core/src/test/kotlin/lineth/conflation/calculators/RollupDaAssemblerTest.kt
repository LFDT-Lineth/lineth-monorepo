package lineth.conflation.calculators

import linea.da.DaConstants.BLOB_BYTES
import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaConstants.KZG_POINT_BYTES
import linea.da.DaStreamPosition
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.domain.BlocksConflation
import lineth.conflation.calculators.DaTestConflations.conflation
import lineth.conflation.calculators.DaTestConflations.encoder
import lineth.conflation.calculators.DaTestConflations.genesisDataRollingHash
import lineth.coordination.riscv.da.FakeDaCompressor
import lineth.coordination.riscv.da.FakeDaStream
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class RollupDaAssemblerTest {
  private val compressor = FakeDaCompressor()

  /** Seals blobs from real segments: blob1 = [1 (40k), 2 (50k), start of 3], blob2 = [end of 3]. */
  private fun sealedBlobs(): List<SealedBlob> {
    val stream = FakeDaStream(DaStreamPosition.fresh(genesisDataRollingHash))
    val conflations = listOf(conflation(1UL, 40_000), conflation(2UL, 50_000), conflation(3UL, 60_000))
    val sealed = conflations.flatMap {
      stream.append(compressor.compress(it.blocks.map(encoder::encode)))
    }
    return sealed + stream.flush()!!
  }

  /**
   * Feeds [conflations] through a [FakeDaStream] and the assembler in the order the rollup calculator does (each sealed
   * blob, then the conflations whose segment ends in it), ending a rollup after each conflation whose end block is in
   * [cutAfter]. The open blob is flushed at the end.
   */
  private fun assemble(conflations: List<BlocksConflation>, cutAfter: Set<ULong>): List<Rollup> {
    val stream = FakeDaStream(DaStreamPosition.fresh(genesisDataRollingHash))
    val assembler = RollupDaAssembler(DaStreamPosition.fresh(genesisDataRollingHash))
    val unsealed = ArrayDeque<BlocksConflation>()
    val rollups = mutableListOf<Rollup>()
    fun onBlob(blob: SealedBlob) {
      assembler.addBlob(blob)
      blob.segmentEnds.forEach { end ->
        assembler.addConflation(unsealed.removeFirst())
        if (end.conflationEndBlock in cutAfter) rollups += assembler.endRollup(end)
      }
    }
    conflations.forEach {
      unsealed += it
      stream.append(compressor.compress(it.blocks.map(encoder::encode))).forEach(::onBlob)
    }
    stream.flush()?.let(::onBlob)
    return rollups
  }

  /** Each rollup's blobs chain by rolling hash, and each rollup starts exactly where the previous one ends. */
  private fun assertChained(rollups: List<Rollup>) {
    rollups.forEach { rollup ->
      rollup.da.blobs.zipWithNext().forEach { (previous, next) ->
        assertThat(next.prevDataRollingHash).isEqualTo(previous.dataRollingHash)
      }
      assertThat(rollup.da.expectedEndDataRollingHash).isEqualTo(rollup.da.blobs.last().dataRollingHash)
    }
    assertThat(rollups.first().da.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
    rollups.zipWithNext().forEach { (previous, next) ->
      assertThat(next.da.parentDataRollingHash).isEqualTo(previous.da.expectedEndDataRollingHash)
      assertThat(next.da.parentDataTailTakeBytes).isEqualTo(previous.da.expectedFinalDataTailDiscardBytes)
      if (next.da.parentDataTailTakeBytes > 0u) {
        assertThat(next.da.blobs.first()).isSameAs(previous.da.blobs.last())
        assertThat(next.da.boundaryPrevDataRollingHash).isEqualTo(previous.da.blobs.last().prevDataRollingHash)
      } else {
        assertThat(next.da.boundaryPrevDataRollingHash).isEqualTo(next.da.parentDataRollingHash)
      }
    }
  }

  private fun Rollup.ends(): List<ULong> = conflations.map { it.endBlockNumber }

  @Test
  fun `a huge conflation spanning many blobs takes them all into its rollup`() {
    // blob1 = [1..2 (10k), start of 3..12], blob2..blob4 = [middle of 3..12], blob5 = [end of 3..12 (15k), 13..14].
    val small = conflation(1UL, 2UL, 10_000)
    val huge = conflation(3UL, 12UL, 4 * BLOB_PAYLOAD_CAPACITY + 5_000)
    val next = conflation(13UL, 14UL, 10_000)

    val rollups = assemble(listOf(small, huge, next), cutAfter = setOf(2UL, 12UL, 14UL))

    assertChained(rollups)
    val (first, spanning, last) = rollups
    assertThat(rollups.map { it.ends() }).containsExactly(listOf(2UL), listOf(12UL), listOf(14UL))
    assertThat(spanning.da.blobs).hasSize(5)
    assertThat(spanning.da.blobs.first()).isSameAs(first.da.blobs.single())
    assertThat(spanning.da.blobs.subList(1, 4)).allSatisfy { assertThat(it.segmentEnds).isEmpty() }
    assertThat(spanning.da.parentDataTailTakeBytes).isEqualTo((BLOB_PAYLOAD_CAPACITY - 10_000).toUInt())
    assertThat(spanning.da.expectedFinalDataTailDiscardBytes).isEqualTo(10_000u)
    assertThat(last.da.blobs).containsExactly(spanning.da.blobs.last())
    assertThat(last.da.expectedFinalDataTailDiscardBytes).isEqualTo(0u)
  }

  @Test
  fun `many small conflations in one blob split into several rollups sharing it`() {
    // Ten 2-block conflations of 10k each, all in one (flushed) blob; a rollup after every third.
    val conflations = (0 until 10).map { conflation(2UL * it.toULong() + 1UL, 2UL * it.toULong() + 2UL, 10_000) }

    val rollups = assemble(conflations, cutAfter = setOf(6UL, 12UL, 18UL, 20UL))

    assertChained(rollups)
    assertThat(rollups.map { it.ends() })
      .containsExactly(listOf(2UL, 4UL, 6UL), listOf(8UL, 10UL, 12UL), listOf(14UL, 16UL, 18UL), listOf(20UL))
    val blob = rollups.first().da.blobs.single()
    assertThat(blob.segmentEnds).hasSize(10)
    assertThat(rollups).allSatisfy { assertThat(it.da.blobs).containsExactly(blob) }
    assertThat(rollups.map { it.da.expectedFinalDataTailDiscardBytes }).containsExactly(70_000u, 40_000u, 10_000u, 0u)
  }

  @Test
  fun `one rollup can hold many small conflations and a huge one spanning blobs`() {
    // blob1 = [4 smalls (40k), start of 9..15], blob2 = [middle of 9..15], blob3 = [end of 9..15 (40k), 3 smalls].
    val smallsBefore = (0 until 4).map { conflation(2UL * it.toULong() + 1UL, 2UL * it.toULong() + 2UL, 10_000) }
    val huge = conflation(9UL, 15UL, 2 * BLOB_PAYLOAD_CAPACITY)
    val smallsAfter = (0 until 3).map { conflation(2UL * it.toULong() + 16UL, 2UL * it.toULong() + 17UL, 10_000) }
    // A first rollup cut after the second small conflation, so the mixed rollup starts inside blob1.
    val rollups = assemble(smallsBefore + huge + smallsAfter, cutAfter = setOf(4UL, 21UL))

    assertChained(rollups)
    val (first, mixed) = rollups
    assertThat(first.ends()).containsExactly(2UL, 4UL)
    assertThat(mixed.ends()).containsExactly(6UL, 8UL, 15UL, 17UL, 19UL, 21UL)
    val (blob1, blob2, blob3) = mixed.da.blobs
    assertThat(mixed.da.blobs).hasSize(3)
    assertThat(blob1).isSameAs(first.da.blobs.single())
    assertThat(blob1.segmentEnds.map { it.conflationEndBlock }).containsExactly(2UL, 4UL, 6UL, 8UL)
    assertThat(blob2.segmentEnds).isEmpty()
    assertThat(blob3.segmentEnds.map { it.conflationEndBlock }).containsExactly(15UL, 17UL, 19UL, 21UL)
    assertThat(mixed.da.parentDataTailTakeBytes).isEqualTo((BLOB_PAYLOAD_CAPACITY - 20_000).toUInt())
    assertThat(mixed.da.expectedFinalDataTailDiscardBytes).isEqualTo(0u)
  }

  @Test
  fun `a fresh start rollup ending mid-blob hands the blob to the next rollup`() {
    val (blob1, blob2) = sealedBlobs()
    val assembler = RollupDaAssembler(DaStreamPosition.fresh(genesisDataRollingHash))

    assembler.addBlob(blob1)
    assembler.addConflation(conflation(1UL, 40_000))
    val first = assembler.endRollup(blob1.segmentEnds[0])
    assembler.addConflation(conflation(2UL, 50_000))
    val second = assembler.endRollup(blob1.segmentEnds[1])
    assembler.addBlob(blob2)
    assembler.addConflation(conflation(3UL, 60_000))
    val third = assembler.endRollup(blob2.segmentEnds.single())

    assertThat(first.da.blobs).containsExactly(blob1)
    assertThat(first.da.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
    assertThat(first.da.parentDataTailTakeBytes).isEqualTo(0u)
    assertThat(second.da.blobs).containsExactly(blob1)
    assertThat(second.da.parentDataTailTakeBytes).isEqualTo(blob1.payloadBytes - 40_000u)
    assertThat(second.da.boundaryPrevDataRollingHash).isEqualTo(blob1.prevDataRollingHash)
    assertThat(third.da.blobs).containsExactly(blob1, blob2)
    assertThat(third.da.expectedEndDataRollingHash).isEqualTo(blob2.dataRollingHash)
    assertThat(third.da.expectedFinalDataTailDiscardBytes).isEqualTo(0u)
  }

  @Test
  fun `a rollup ending at a blob's payload end makes the next rollup start fresh`() {
    val (blob1, blob2) = sealedBlobs()
    val assembler = RollupDaAssembler(DaStreamPosition.fresh(genesisDataRollingHash))
    assembler.addBlob(blob1)
    assembler.addBlob(blob2)
    listOf(conflation(1UL, 40_000), conflation(2UL, 50_000), conflation(3UL, 60_000)).forEach(assembler::addConflation)

    assembler.endRollup(blob2.segmentEnds.single())
    assembler.addConflation(conflation(4UL, 1_000))

    assertThatThrownBy { assembler.addBlob(blob2) }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("must follow parentDataRollingHash")
  }

  @Test
  fun `restarting inside a shared blob requires that blob first`() {
    val (blob1, blob2) = sealedBlobs()
    val assembler = RollupDaAssembler(
      DaStreamPosition(
        dataRollingHash = blob1.dataRollingHash,
        parentBlobDataRollingHash = blob1.prevDataRollingHash,
        sharedBlobCarryOverTailBytes = 100u,
        sharedBlob = ByteArray(BLOB_BYTES),
      ),
    )

    assertThatThrownBy { assembler.addBlob(blob2) }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("must be that blob")
    assembler.addBlob(blob1)
    assembler.addBlob(blob2)
  }

  @Test
  fun `blobs must chain by rolling hash and cuts must end the last conflation in the newest blob`() {
    val (blob1, blob2) = sealedBlobs()
    val assembler = RollupDaAssembler(DaStreamPosition.fresh(genesisDataRollingHash))
    assembler.addBlob(blob1)

    assertThatThrownBy { assembler.addBlob(blob1) }.hasMessageContaining("does not continue the open rollup")
    assembler.addConflation(conflation(1UL, 40_000))
    assertThatThrownBy {
      assembler.endRollup(
        blob2.segmentEnds.single(),
      )
    }.hasMessageContaining("not in the newest blob")
    assertThatThrownBy {
      assembler.endRollup(
        blob1.segmentEnds[1],
      )
    }.hasMessageContaining("does not end the open rollup")
    assertThatThrownBy {
      RollupDaAssembler(
        DaStreamPosition.fresh(genesisDataRollingHash),
      ).endRollup(SegmentEnd(1UL, 1u))
    }
      .hasMessageContaining("without blobs")
  }

  @Test
  fun `witness equality compares blobs and hashes by content`() {
    fun blob() = SealedBlob(
      blob = ByteArray(BLOB_BYTES),
      chunkHash = ByteArray(32),
      kzgCommitment = ByteArray(KZG_POINT_BYTES),
      kzgProof = ByteArray(KZG_POINT_BYTES),
      prevDataRollingHash = ByteArray(32),
      dataRollingHash = ByteArray(32),
      payloadBytes = 1u,
      segmentEnds = emptyList(),
    )
    fun witness() = RollupDaWitness(
      listOf(blob()),
      ByteArray(32) { 1 },
      0u,
      ByteArray(32) { 1 },
      ByteArray(32) { 2 },
      0u,
    )

    assertThat(witness()).isEqualTo(witness())
    assertThat(witness().hashCode()).isEqualTo(witness().hashCode())
  }
}
