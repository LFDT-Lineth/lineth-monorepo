package lineth.conflation.calculators

import linea.da.DaStreamPosition
import linea.da.SealedBlob
import linea.da.SegmentEnd
import lineth.conflation.calculators.DaPipelineTestSupport.conflation
import lineth.conflation.calculators.DaPipelineTestSupport.genesisDataRollingHash
import lineth.coordination.riscv.da.FakeDaStream
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.nio.ByteBuffer

class RollupDaAssemblerTest {
  /** Seals blobs from real frames: blob1 = [1 (40k), 2 (50k), start of 3], blob2 = [end of 3]. */
  private fun sealedBlobs(): List<SealedBlob> {
    val stream = FakeDaStream(DaStreamPosition(genesisDataRollingHash))
    val conflations = listOf(conflation(1UL, 40_000), conflation(2UL, 50_000), conflation(3UL, 60_000))
    val sealed = conflations.flatMap {
      stream.append(stream.compress(it.blocks.map(DaPipelineTestSupport.encoder::encode)))
    }
    return sealed + stream.flush()!!
  }

  @Test
  fun `a fresh start rollup ending mid-blob hands the blob to the next rollup`() {
    val (blob1, blob2) = sealedBlobs()
    val assembler = RollupDaAssembler(DaStreamPosition(genesisDataRollingHash))

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
    assertThat(first.da.parentDataTailTakeBytes).isZero()
    assertThat(second.da.blobs).containsExactly(blob1)
    assertThat(second.da.parentDataTailTakeBytes).isEqualTo(blob1.payloadBytes - 40_000)
    assertThat(second.da.boundaryPrevDataRollingHash).isEqualTo(blob1.prevDataRollingHash)
    assertThat(third.da.blobs).containsExactly(blob1, blob2)
    assertThat(third.da.expectedEndDataRollingHash).isEqualTo(blob2.dataRollingHash)
    assertThat(third.da.expectedFinalDataTailDiscardBytes).isZero()
  }

  @Test
  fun `a rollup ending at a blob's payload end makes the next rollup start fresh`() {
    val (blob1, blob2) = sealedBlobs()
    val assembler = RollupDaAssembler(DaStreamPosition(genesisDataRollingHash))
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
        tailBytes = 100,
        sharedBlob = ByteArray(linea.da.DaConstants.BLOB_BYTES),
        prevDataRollingHash = blob1.prevDataRollingHash,
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
    val assembler = RollupDaAssembler(DaStreamPosition(genesisDataRollingHash))
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
    assertThatThrownBy { RollupDaAssembler(DaStreamPosition(genesisDataRollingHash)).endRollup(SegmentEnd(1UL, 1)) }
      .hasMessageContaining("without blobs")
  }

  @Test
  fun `witness equality compares blobs by reference and hashes by content`() {
    val blob = object : SealedBlob {
      override val blob: ByteBuffer = ByteBuffer.allocate(0)
      override val chunkHash = ByteArray(32)
      override val prevDataRollingHash = ByteArray(32)
      override val dataRollingHash = ByteArray(32)
      override val payloadBytes = 1
      override val segmentEnds = emptyList<SegmentEnd>()
    }
    fun witness() = RollupDaWitness(listOf(blob), ByteArray(32) { 1 }, 0, null, ByteArray(32) { 2 }, 0)

    assertThat(witness()).isEqualTo(witness())
    assertThat(witness().hashCode()).isEqualTo(witness().hashCode())
  }
}
