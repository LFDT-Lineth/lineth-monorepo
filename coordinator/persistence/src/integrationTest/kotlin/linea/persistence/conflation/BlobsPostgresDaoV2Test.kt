package linea.persistence.conflation

import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import linea.domain.BlobRecordV2
import linea.domain.BlobStatus
import linea.domain.createBlobRecordV2
import linea.error.DuplicatedRecordException
import linea.kotlin.trimToMillisecondPrecision
import linea.kotlin.trimToSecondPrecision
import linea.persistence.db.DbHelper
import linea.persistence.db.test.CleanDbTestSuiteParallel
import net.consensys.FakeFixedClock
import net.consensys.linea.async.get
import net.consensys.linea.async.toSafeFuture
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class BlobsPostgresDaoV2Test : CleanDbTestSuiteParallel() {
  init {
    target = "4"
  }

  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-blobs-dao-v2")
  private val maxBlobsToReturn = 6u
  private fun blobsContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from ${BlobsPostgresDaoG.TableName}")

  private val fakeClock = FakeFixedClock()
  private lateinit var blobsPostgresDao: BlobsPostgresDaoV2

  private val expectedStartBlock = 1UL
  private val expectedEndBlock = 100UL
  private val expectedStartBlockTimestamp = fakeClock.now().trimToSecondPrecision()
  private val expectedEndBlockTimestamp = fakeClock.now().plus(1200.seconds).trimToMillisecondPrecision()

  @BeforeEach
  fun beforeEach() {
    blobsPostgresDao =
      BlobsPostgresDaoV2(
        config = BlobsPostgresDaoV2.Config(
          maxRollupProofsToReturn = maxBlobsToReturn,
        ),
        connection = sqlClient,
        clock = fakeClock,
      )
  }

  private fun performInsertTest(blobRecord: BlobRecordV2): RowSet<Row>? {
    blobsPostgresDao.saveNewBlob(blobRecord).get()
    val dbContent = blobsContentQuery().execute().get()
    val newlyInsertedRow =
      dbContent.find { it.getLong("created_epoch_milli") == fakeClock.now().toEpochMilliseconds() }
    assertThat(newlyInsertedRow).isNotNull

    assertThat(newlyInsertedRow!!.getLong("start_block_number"))
      .isEqualTo(blobRecord.startBlockNumber.toLong())
    assertThat(newlyInsertedRow.getLong("end_block_number"))
      .isEqualTo(blobRecord.endBlockNumber.toLong())
    assertThat(newlyInsertedRow.getInteger("batches_count"))
      .isEqualTo(blobRecord.totalBatchesCount.toInt())
    assertThat(newlyInsertedRow.getInteger("status")).isEqualTo(
      BlobsPostgresDaoG.blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
    )

    return dbContent
  }

  @Test
  fun `saveNewBlob inserts new blob to db`() {
    val blobRecord1 = createBlobRecordV2(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )
    fakeClock.setTimeTo(Clock.System.now())

    val dbContent1 = performInsertTest(blobRecord1)
    assertThat(dbContent1).size().isEqualTo(1)

    val blobRecord2 = createBlobRecordV2(
      startBlockNumber = expectedEndBlock + 1UL,
      endBlockNumber = expectedEndBlock + 100UL,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )
    fakeClock.advanceBy(1.seconds)

    val dbContent2 = performInsertTest(blobRecord2)
    assertThat(dbContent2).size().isEqualTo(2)
  }

  @Test
  fun `saveNewBlob returns error when duplicated`() {
    val blobRecord1 = createBlobRecordV2(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )

    val dbContent1 = performInsertTest(blobRecord1)
    assertThat(dbContent1).size().isEqualTo(1)

    assertThrows<ExecutionException> {
      blobsPostgresDao.saveNewBlob(blobRecord1).get()
    }.also { executionException ->
      assertThat(executionException.cause).isInstanceOf(DuplicatedRecordException::class.java)
      assertThat(executionException.cause!!.message)
        .isEqualTo(
          "Blob [1..100]100 is already persisted!",
        )
    }
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber works correctly for 1 blob`() {
    val expectedStartBlock1 = 1UL
    val expectedEndBlock1 = 90UL
    val expectedBlob = createBlobRecordV2(
      startBlockNumber = expectedStartBlock1,
      endBlockNumber = expectedEndBlock1,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )

    blobsPostgresDao.saveNewBlob(expectedBlob).get()

    val actualBlobs =
      blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
        expectedStartBlock1,
        expectedEndBlockTimestamp.plus(12.seconds),
      ).get()
    assertThat(actualBlobs).hasSameElementsAs(listOf(expectedBlob))
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber returns empty list if no matched`() {
    val blobRecord1 = createBlobRecordV2(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )
    val blobRecord2 = createBlobRecordV2(
      startBlockNumber = expectedEndBlock + 1UL,
      endBlockNumber = expectedEndBlock + 100UL,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )
    val blobRecord3 = createBlobRecordV2(
      startBlockNumber = expectedEndBlock + 101UL,
      endBlockNumber = expectedEndBlock + 200UL,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )

    SafeFuture.collectAll(
      blobsPostgresDao.saveNewBlob(blobRecord1),
      blobsPostgresDao.saveNewBlob(blobRecord2),
      blobsPostgresDao.saveNewBlob(blobRecord3),
    ).get()

    blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
      expectedStartBlock + 1UL,
      blobRecord3.endBlockTimestamp.plus(1.seconds),
    ).get().also { blobs ->
      assertThat(blobs).isEmpty()
    }
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber returns a sequence of blobs without gaps`() {
    val blobRecord1 = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 40UL,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )
    val blobRecord2 = createBlobRecordV2(
      startBlockNumber = 41UL,
      endBlockNumber = 60UL,
      startBlockTimestamp = blobRecord1.endBlockTimestamp.plus(3.seconds),
    )
    val blobRecord3 = createBlobRecordV2(
      startBlockNumber = 61UL,
      endBlockNumber = 100UL,
      startBlockTimestamp = blobRecord2.endBlockTimestamp.plus(3.seconds),
    )
    val blobRecord4 = createBlobRecordV2(
      startBlockNumber = 101UL,
      endBlockNumber = 111UL,
      startBlockTimestamp = blobRecord3.endBlockTimestamp.plus(3.seconds),
    )
    val blobRecord5 = createBlobRecordV2(
      startBlockNumber = 112UL,
      endBlockNumber = 132UL,
      startBlockTimestamp = blobRecord4.endBlockTimestamp.plus(3.seconds),
    )
    val blobRecord6 = createBlobRecordV2(
      startBlockNumber = 134UL,
      endBlockNumber = 156UL,
      startBlockTimestamp = blobRecord5.endBlockTimestamp.plus(3.seconds),
    )
    val blobRecord7 = createBlobRecordV2(
      startBlockNumber = 157UL,
      endBlockNumber = 189UL,
      startBlockTimestamp = blobRecord5.endBlockTimestamp.plus(3.seconds),
    )
    val expectedBlobs = listOf(blobRecord3, blobRecord4, blobRecord5)
    val otherBlobs = listOf(blobRecord1, blobRecord2, blobRecord6, blobRecord7)

    saveBlobs(expectedBlobs + otherBlobs)

    val actualBlobs =
      blobsPostgresDao
        .getConsecutiveBlobsFromBlockNumber(
          startingBlockNumberInclusive = expectedBlobs.first().startBlockNumber,
          endBlockCreatedBefore = expectedBlobs.last().endBlockTimestamp.plus(1.seconds),
        ).get()
    assertThat(actualBlobs).hasSameElementsAs(expectedBlobs)
  }

  @Test
  fun `findBlobByXBlockNumber works correctly for 1 blob`() {
    val expectedBlob = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 90UL,
      startBlockTimestamp = expectedStartBlockTimestamp,
    )

    blobsPostgresDao.saveNewBlob(expectedBlob).get()

    assertThat(blobsPostgresDao.findBlobByEndBlockNumber(90UL))
      .succeedsWithin(1.seconds.toJavaDuration())
      .isEqualTo(expectedBlob)

    assertThat(blobsPostgresDao.findBlobByEndBlockNumber(91UL))
      .succeedsWithin(1.seconds.toJavaDuration())
      .isNull()

    assertThat(blobsPostgresDao.findBlobByStartBlockNumber(1UL))
      .succeedsWithin(1.seconds.toJavaDuration())
      .isEqualTo(expectedBlob)

    assertThat(blobsPostgresDao.findBlobByStartBlockNumber(2UL))
      .succeedsWithin(1.seconds.toJavaDuration())
      .isNull()
  }

  @Test
  fun `deleteBlobsUpToEndBlockNumber deletes the target record correctly`() {
    val blobRecord1 = createBlobRecordV2(1UL, 40UL, expectedStartBlockTimestamp)
    val blobRecord2 = createBlobRecordV2(41UL, 60UL, expectedStartBlockTimestamp)
    val blobRecord3 = createBlobRecordV2(61UL, 100UL, expectedStartBlockTimestamp)
    val blobRecord4 = createBlobRecordV2(101UL, 111UL, expectedStartBlockTimestamp)
    val blobRecord5 = createBlobRecordV2(112UL, 132UL, expectedStartBlockTimestamp)
    val blobRecord6 = createBlobRecordV2(133UL, 156UL, expectedStartBlockTimestamp)
    val blobRecord7 = createBlobRecordV2(157UL, 189UL, expectedStartBlockTimestamp)

    val expectedBlobs = listOf(blobRecord4, blobRecord5, blobRecord6, blobRecord7)
    val deletedBlobs = listOf(blobRecord1, blobRecord2, blobRecord3)

    expectedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }
    deletedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }

    blobsPostgresDao.deleteBlobsUpToEndBlockNumber(blobRecord3.endBlockNumber).get()

    val existedBlobRecords = blobsContentQuery().execute()
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.map { row -> BlobsPostgresDaoV2.parseRecord(row) }
      }.get()

    assertThat(existedBlobRecords).hasSameElementsAs(expectedBlobs)
  }

  @Test
  fun `deleteBlobsAfterBlockNumber deletes the target record correctly`() {
    val blobRecord1 = createBlobRecordV2(1UL, 40UL, expectedStartBlockTimestamp)
    val blobRecord2 = createBlobRecordV2(41UL, 60UL, expectedStartBlockTimestamp)
    val blobRecord3 = createBlobRecordV2(61UL, 100UL, expectedStartBlockTimestamp)
    val blobRecord4 = createBlobRecordV2(101UL, 111UL, expectedStartBlockTimestamp)
    val blobRecord5 = createBlobRecordV2(112UL, 132UL, expectedStartBlockTimestamp)
    val blobRecord6 = createBlobRecordV2(133UL, 156UL, expectedStartBlockTimestamp)
    val blobRecord7 = createBlobRecordV2(157UL, 189UL, expectedStartBlockTimestamp)

    val deletedBlobs = listOf(blobRecord4, blobRecord5, blobRecord6, blobRecord7)
    val expectedBlobs = listOf(blobRecord1, blobRecord2, blobRecord3)

    expectedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }
    deletedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }

    blobsPostgresDao.deleteBlobsAfterBlockNumber(blobRecord3.endBlockNumber).get()

    val existedBlobRecords = blobsContentQuery().execute()
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.map { row -> BlobsPostgresDaoV2.parseRecord(row) }
      }.get()

    assertThat(existedBlobRecords).hasSameElementsAs(expectedBlobs)
  }

  private fun saveBlobs(blobRecords: List<BlobRecordV2>) {
    SafeFuture.collectAll(blobRecords.map(blobsPostgresDao::saveNewBlob).stream()).get()
  }
}