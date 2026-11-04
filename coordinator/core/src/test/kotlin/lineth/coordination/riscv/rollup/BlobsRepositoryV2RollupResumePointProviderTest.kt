package lineth.coordination.riscv.rollup

import linea.da.DaConstants
import linea.da.DaStreamPosition
import linea.da.RollupResumePoint
import linea.domain.BlobRecordV2
import linea.domain.createBlobData
import linea.domain.createBlobRecordV2
import lineth.persistence.BlobsRepositoryV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class BlobsRepositoryV2RollupResumePointProviderTest {
  private val cutover = Instant.parse("2026-10-01T00:00:00Z")
  private val genesisDataRollingHash = ByteArray(32) { 0x11 }
  private val rows = mutableMapOf<ULong, BlobRecordV2>()
  private lateinit var provider: BlobsRepositoryV2RollupResumePointProvider

  private inner class FakeRepository : BlobsRepositoryV2 {
    override fun findBlobByEndBlockNumber(endBlockNumber: Long): SafeFuture<BlobRecordV2?> =
      SafeFuture.completedFuture(rows[endBlockNumber.toULong()])

    override fun findHighestConsecutiveEndBlockNumberFromBlockNumber(
      startingBlockNumberInclusive: ULong,
    ): SafeFuture<ULong?> {
      var next = startingBlockNumberInclusive
      var lastEnd: ULong? = null
      while (true) {
        val row = rows.values.find { it.startBlockNumber == next } ?: break
        lastEnd = row.endBlockNumber
        next = row.endBlockNumber + 1UL
      }
      return SafeFuture.completedFuture(lastEnd)
    }

    override fun saveNewBlob(blobRecord: BlobRecordV2): SafeFuture<Unit> = throw UnsupportedOperationException()

    override fun getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive: Long,
      endBlockCreatedBefore: Instant,
    ): SafeFuture<List<BlobRecordV2>> = throw UnsupportedOperationException()

    override fun findBlobByStartBlockNumber(startBlockNumber: Long): SafeFuture<BlobRecordV2?> =
      throw UnsupportedOperationException()

    override fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> =
      throw UnsupportedOperationException()

    override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> =
      throw UnsupportedOperationException()
  }

  @BeforeEach
  fun setUp() {
    provider = BlobsRepositoryV2RollupResumePointProvider(
      blobsRepository = FakeRepository(),
      riscvCutoverTimestamp = cutover,
      genesisDataRollingHashProvider = { SafeFuture.completedFuture(genesisDataRollingHash) },
    )
  }

  private fun addRow(start: ULong, end: ULong, finalDataTailDiscardBytes: Int = 0): BlobRecordV2 {
    val record = if (finalDataTailDiscardBytes > 0) {
      val parent = Random.nextBytes(32)
      val drh = Random.nextBytes(32)
      createBlobRecordV2(
        startBlockNumber = start,
        endBlockNumber = end,
        parentDataRollingHash = parent,
        dataRollingHash = drh,
        finalDataTailDiscardBytes = finalDataTailDiscardBytes,
        blobsData = listOf(createBlobData(parent, drh, blobBytes = Random.nextBytes(DaConstants.BLOB_BYTES))),
      )
    } else {
      createBlobRecordV2(startBlockNumber = start, endBlockNumber = end)
    }
    rows[end] = record
    return record
  }

  private fun resume(lastFinalizedBlock: ULong, timestamp: Instant): RollupResumePoint =
    provider.getResumePoint(lastFinalizedBlock, timestamp).get()

  @Test
  fun `before the cutover with no rows it resumes at the genesis position`() {
    assertThat(resume(100UL, cutover - 1.seconds))
      .isEqualTo(RollupResumePoint(100UL, DaStreamPosition(genesisDataRollingHash)))
  }

  @Test
  fun `before the cutover it resumes after the rows that follow the finalized block`() {
    addRow(101UL, 110UL)
    val last = addRow(111UL, 120UL)
    addRow(131UL, 140UL) // after a gap

    assertThat(resume(100UL, cutover - 1.seconds))
      .isEqualTo(RollupResumePoint(120UL, DaStreamPosition(last.dataRollingHash)))
  }

  @Test
  fun `after the cutover it resumes after the row ending at the finalized block`() {
    val anchor = addRow(91UL, 100UL)

    assertThat(resume(100UL, cutover))
      .isEqualTo(RollupResumePoint(100UL, DaStreamPosition(anchor.dataRollingHash)))
  }

  @Test
  fun `a last row ending inside a blob resumes inside that shared blob`() {
    addRow(91UL, 100UL)
    val last = addRow(101UL, 110UL, finalDataTailDiscardBytes = 500)

    val lastBlob = last.blobsData.last()
    assertThat(resume(100UL, cutover + 1.seconds)).isEqualTo(
      RollupResumePoint(
        110UL,
        DaStreamPosition(
          dataRollingHash = last.dataRollingHash,
          tailBytes = 500,
          sharedBlob = lastBlob.blobBytes,
          prevDataRollingHash = lastBlob.prevDataRollingHash,
        ),
      ),
    )
  }

  @Test
  fun `after the cutover a missing row at the finalized block is an error`() {
    addRow(101UL, 110UL)

    assertThatThrownBy { resume(100UL, cutover) }
      .hasCauseInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("L1 finalized block 100 is past the RISC-V cutover but no rollup row ends at it")
  }
}
