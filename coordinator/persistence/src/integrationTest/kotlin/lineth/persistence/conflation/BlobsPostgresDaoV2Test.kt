package lineth.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import linea.domain.BlobRecordV2
import linea.domain.createBlobRecordV2
import linea.persistence.db.DbHelper
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Clock
import kotlin.time.Instant

class BlobsPostgresDaoV2Test : BlobsPostgresDaoTestBase<BlobRecordV2>() {
  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-blobs-dao-v2")

  override fun buildDao(maxBlobsToReturn: UInt, connection: SqlClient, clock: Clock): BlobsDaoG<BlobRecordV2> =
    BlobsPostgresDaoV2(
      config = BlobsPostgresDaoV2.Config(maxRollupProofsToReturn = maxBlobsToReturn),
      connection = connection,
      clock = clock,
    )

  override fun createBlob(startBlockNumber: ULong, endBlockNumber: ULong, startBlockTime: Instant): BlobRecordV2 =
    createBlobRecordV2(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      startBlockTimestamp = startBlockTime,
    )

  override fun parseRecord(row: Row): BlobRecordV2 = BlobsPostgresDaoV2.parseRecord(row)

  override fun endBlockTime(blob: BlobRecordV2): Instant = blob.endBlockTimestamp

  override fun startBlockNumber(blob: BlobRecordV2): ULong = blob.startBlockNumber

  override fun endBlockNumber(blob: BlobRecordV2): ULong = blob.endBlockNumber

  override fun assertAdditionalInsertedColumns(row: Row, blob: BlobRecordV2) {
    assertThat(row.getInteger("batches_count")).isEqualTo(blob.totalBatchesCount.toInt())
  }

  private val daoV2: BlobsDaoV2 get() = blobsPostgresDao as BlobsDaoV2

  @Test
  fun `a record starting inside a shared blob round-trips with its tail fields`() {
    val record = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      startBlockTimestamp = expectedStartBlockTime,
      parentDataTailTakeBytes = 100,
      finalDataTailDiscardBytes = 200,
    )
    daoV2.saveNewBlob(record).get()

    assertThat(daoV2.findBlobByEndBlockNumber(10UL).get()).isEqualTo(record)
  }

  @Test
  fun `findHighestConsecutiveEndBlockNumberFromBlockNumber returns the end of the gap-free run`() {
    listOf(1UL to 10UL, 11UL to 20UL, 21UL to 30UL, 41UL to 50UL).forEach { (start, end) ->
      daoV2.saveNewBlob(createBlob(start, end, expectedStartBlockTime)).get()
    }

    assertThat(daoV2.findHighestConsecutiveEndBlockNumberFromBlockNumber(1UL).get()).isEqualTo(30UL)
    assertThat(daoV2.findHighestConsecutiveEndBlockNumberFromBlockNumber(21UL).get()).isEqualTo(30UL)
    assertThat(daoV2.findHighestConsecutiveEndBlockNumberFromBlockNumber(41UL).get()).isEqualTo(50UL)
    assertThat(daoV2.findHighestConsecutiveEndBlockNumberFromBlockNumber(31UL).get()).isNull()
    assertThat(daoV2.findHighestConsecutiveEndBlockNumberFromBlockNumber(5UL).get()).isNull()
  }
}
