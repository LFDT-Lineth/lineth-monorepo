package lineth.coordination.riscv.rollup

import linea.da.DaConstants
import linea.da.DaStreamPosition
import linea.da.RollupResumePoint
import linea.domain.RiscvRollupRecord
import linea.domain.createBlobData
import linea.domain.createRiscvRollupRecord
import lineth.persistence.FakeRiscvRollupsDao
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class RiscvRollupsResumePointProviderTest {
  private val cutover = Instant.parse("2026-10-01T00:00:00Z")
  private val genesisDataRollingHash = ByteArray(32) { 0x11 }
  private val rollupsDao = FakeRiscvRollupsDao()
  private lateinit var provider: RiscvRollupsResumePointProvider

  @BeforeEach
  fun setUp() {
    provider = RiscvRollupsResumePointProvider(
      rollupsDao = rollupsDao,
      riscvCutoverTimestamp = cutover,
      genesisDataRollingHashProvider = { SafeFuture.completedFuture(genesisDataRollingHash) },
    )
  }

  private fun addRow(start: ULong, end: ULong, finalSharedBlobCarryOverTailBytes: UInt = 0u): RiscvRollupRecord {
    val record = if (finalSharedBlobCarryOverTailBytes > 0u) {
      val parent = Random.nextBytes(32)
      val drh = Random.nextBytes(32)
      createRiscvRollupRecord(
        startBlockNumber = start,
        endBlockNumber = end,
        parentDataRollingHash = parent,
        dataRollingHash = drh,
        finalSharedBlobCarryOverTailBytes = finalSharedBlobCarryOverTailBytes,
        blobsData = listOf(createBlobData(parent, drh, blobBytes = Random.nextBytes(DaConstants.BLOB_BYTES))),
      )
    } else {
      createRiscvRollupRecord(startBlockNumber = start, endBlockNumber = end)
    }
    rollupsDao.saveRollupWithBlobs(record).get()
    return record
  }

  private fun resume(lastFinalizedBlock: ULong, timestamp: Instant): RollupResumePoint =
    provider.getResumePoint(lastFinalizedBlock, timestamp).get()

  @Test
  fun `before the cutover with no rows it resumes at the genesis position`() {
    assertThat(resume(100UL, cutover - 1.seconds))
      .isEqualTo(RollupResumePoint(100UL, DaStreamPosition.fresh(genesisDataRollingHash)))
  }

  @Test
  fun `before the cutover it resumes after the rows that follow the finalized block`() {
    addRow(101UL, 110UL)
    val last = addRow(111UL, 120UL)
    addRow(131UL, 140UL) // after a gap

    assertThat(resume(100UL, cutover - 1.seconds))
      .isEqualTo(RollupResumePoint(120UL, DaStreamPosition.fresh(last.dataRollingHash)))
  }

  @Test
  fun `after the cutover it resumes after the row ending at the finalized block`() {
    val anchor = addRow(91UL, 100UL)

    assertThat(resume(100UL, cutover))
      .isEqualTo(RollupResumePoint(100UL, DaStreamPosition.fresh(anchor.dataRollingHash)))
  }

  @Test
  fun `a last row ending inside a blob resumes inside that shared blob`() {
    addRow(91UL, 100UL)
    val last = addRow(101UL, 110UL, finalSharedBlobCarryOverTailBytes = 500u)

    val lastBlob = last.blobsData.last()
    assertThat(resume(100UL, cutover + 1.seconds)).isEqualTo(
      RollupResumePoint(
        110UL,
        DaStreamPosition(
          dataRollingHash = last.dataRollingHash,
          parentBlobDataRollingHash = lastBlob.parentDataRollingHash,
          sharedBlobCarryOverTailBytes = 500u,
          sharedBlob = lastBlob.blobBytes,
        ),
      ),
    )
  }

  @Test
  fun `after the cutover with no rows following the finalized block a missing row at it is an error`() {
    assertThatThrownBy { resume(100UL, cutover) }
      .hasCauseInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("L1 finalized block 100 is past the RISC-V cutover but no rollup row ends at it")
  }

  @Test
  fun `after the cutover rows following the finalized block are used without reading the row at it`() {
    addRow(101UL, 110UL)
    val last = addRow(111UL, 120UL)

    assertThat(resume(100UL, cutover))
      .isEqualTo(RollupResumePoint(120UL, DaStreamPosition.fresh(last.dataRollingHash)))
  }
}
