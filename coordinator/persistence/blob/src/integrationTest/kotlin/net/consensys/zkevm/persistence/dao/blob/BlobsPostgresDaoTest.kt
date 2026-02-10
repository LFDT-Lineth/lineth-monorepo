package net.consensys.zkevm.persistence.dao.blob

import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import linea.kotlin.trimToMillisecondPrecision
import linea.kotlin.trimToSecondPrecision
import net.consensys.FakeFixedClock
import net.consensys.linea.async.get
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlobStatus
import net.consensys.zkevm.domain.createBlobRecord
import net.consensys.zkevm.persistence.db.DbHelper
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import net.consensys.zkevm.persistence.db.test.CleanDbTestSuiteParallel
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
class BlobsPostgresDaoTest : CleanDbTestSuiteParallel() {
  init {
    target = "4"
  }

  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-blobs-dao")
  private val maxBlobsToReturn = 6u
  private fun blobsContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from ${BlobsPostgresDao.TableName}")

  private val fakeClock = FakeFixedClock()
  private lateinit var blobsPostgresDao: BlobsPostgresDao

  private val expectedStartBlock = 1UL
  private val expectedEndBlock = 100UL
  private val expectedStartBlockTime = fakeClock.now().trimToSecondPrecision()
  private val expectedEndBlockTime = fakeClock.now().plus(1200.seconds).trimToMillisecondPrecision()

  @BeforeEach
  fun beforeEach() {
    blobsPostgresDao =
      BlobsPostgresDao(
        config = BlobsPostgresDao.Config(
          maxBlobsToReturn,
        ),
        connection = sqlClient,
        clock = fakeClock,
      )
  }

  private fun performInsertTest(blobRecord: BlobRecord): RowSet<Row>? {
    blobsPostgresDao.saveNewBlob(blobRecord).get()
    val dbContent = blobsContentQuery().execute().get()
    val newlyInsertedRow =
      dbContent.find { it.getLong("created_epoch_milli") == fakeClock.now().toEpochMilliseconds() }
    assertThat(newlyInsertedRow).isNotNull

    assertThat(newlyInsertedRow!!.getLong("start_block_number"))
      .isEqualTo(blobRecord.startBlockNumber.toLong())
    assertThat(newlyInsertedRow.getLong("end_block_number"))
      .isEqualTo(blobRecord.endBlockNumber.toLong())
    assertThat(newlyInsertedRow.getInteger("status")).isEqualTo(
      BlobsPostgresDao.blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
    )

    return dbContent
  }

  @Test
  fun `saveNewBlob inserts new blob to db`() {
    val blobRecord1 = createBlobRecord(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTime = expectedStartBlockTime,
    )
    fakeClock.setTimeTo(Clock.System.now())

    val dbContent1 = performInsertTest(blobRecord1)
    assertThat(dbContent1).size().isEqualTo(1)

    val blobRecord2 = createBlobRecord(
      startBlockNumber = expectedEndBlock + 1UL,
      endBlockNumber = expectedEndBlock + 100UL,
      startBlockTime = expectedStartBlockTime,
    )
    fakeClock.advanceBy(1.seconds)

    val dbContent2 = performInsertTest(blobRecord2)
    assertThat(dbContent2).size().isEqualTo(2)
  }

  @Test
  fun `saveNewBlob returns error when duplicated`() {
    val blobRecord1 = createBlobRecord(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTime = expectedStartBlockTime,
    )

    val dbContent1 =
      performInsertTest(blobRecord1)
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
    val expectedBlob = createBlobRecord(
      expectedStartBlock1,
      expectedEndBlock1,
      startBlockTime = expectedStartBlockTime,
    )

    blobsPostgresDao.saveNewBlob(expectedBlob).get()

    val actualBlobs =
      blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
        expectedStartBlock1,
        expectedEndBlockTime.plus(12.seconds),
      ).get()
    assertThat(actualBlobs).hasSameElementsAs(listOf(expectedBlob))
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber returns empty list if no matched`() {
    val blobRecord1 = createBlobRecord(
      expectedStartBlock,
      expectedEndBlock,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord2 = createBlobRecord(
      startBlockNumber = expectedEndBlock + 1UL,
      endBlockNumber = expectedEndBlock + 100UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord3 = createBlobRecord(
      startBlockNumber = expectedEndBlock + 101UL,
      endBlockNumber = expectedEndBlock + 200UL,
      startBlockTime = expectedStartBlockTime,
    )

    SafeFuture.collectAll(
      blobsPostgresDao.saveNewBlob(blobRecord1),
      blobsPostgresDao.saveNewBlob(blobRecord2),
      blobsPostgresDao.saveNewBlob(blobRecord3),
    ).get()

    blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
      expectedStartBlock + 1UL,
      blobRecord3.endBlockTime.plus(1.seconds),
    ).get().also { blobs ->
      assertThat(blobs).isEmpty()
    }
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber returns a sequence of blobs without gaps`() {
    val blobRecord1 = createBlobRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 40UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord2 = createBlobRecord(
      startBlockNumber = 41UL,
      endBlockNumber = 60UL,
      startBlockTime = blobRecord1.endBlockTime.plus(3.seconds),
    )
    val blobRecord3 = createBlobRecord(
      startBlockNumber = 61UL,
      endBlockNumber = 100UL,
      startBlockTime = blobRecord2.endBlockTime.plus(3.seconds),
    )
    val blobRecord4 = createBlobRecord(
      startBlockNumber = 101UL,
      endBlockNumber = 111UL,
      startBlockTime = blobRecord3.endBlockTime.plus(3.seconds),
    )
    val blobRecord5 = createBlobRecord(
      startBlockNumber = 112UL,
      endBlockNumber = 132UL,
      startBlockTime = blobRecord4.endBlockTime.plus(3.seconds),
    )
    val blobRecord6 = createBlobRecord(
      startBlockNumber = 134UL,
      endBlockNumber = 156UL,
      startBlockTime = blobRecord5.endBlockTime.plus(3.seconds),
    )
    val blobRecord7 = createBlobRecord(
      startBlockNumber = 157UL,
      endBlockNumber = 189UL,
      startBlockTime = blobRecord5.endBlockTime.plus(3.seconds),
    )
    val expectedBlobs = listOf(
      blobRecord3,
      blobRecord4,
      blobRecord5,
    )
    val otherBlobs = listOf(
      blobRecord1,
      blobRecord2,
      blobRecord6,
      blobRecord7,
    )

    saveBlobs(expectedBlobs + otherBlobs)

    fakeClock.setTimeTo(expectedBlobs.last().endBlockTime.plus(1.seconds))

    val actualBlobs =
      blobsPostgresDao
        .getConsecutiveBlobsFromBlockNumber(
          startingBlockNumberInclusive = expectedBlobs.first().startBlockNumber,
          endBlockCreatedBefore = expectedBlobs.last().endBlockTime.plus(1.seconds),
        ).get()
    assertThat(actualBlobs).hasSameElementsAs(expectedBlobs)
  }

  @Test
  fun `findBlobByXBlockNumber works correctly for 1 blob`() {
    val expectedBlob = createBlobRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 90UL,
      startBlockTime = expectedStartBlockTime,
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
    val blobRecord1 = createBlobRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 40UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord2 = createBlobRecord(
      startBlockNumber = 41UL,
      endBlockNumber = 60UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord3 = createBlobRecord(
      startBlockNumber = 61UL,
      endBlockNumber = 100UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord4 = createBlobRecord(
      startBlockNumber = 101UL,
      endBlockNumber = 111UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord5 = createBlobRecord(
      startBlockNumber = 112UL,
      endBlockNumber = 132UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord6 = createBlobRecord(
      startBlockNumber = 133UL,
      endBlockNumber = 156UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord7 = createBlobRecord(
      startBlockNumber = 157UL,
      endBlockNumber = 189UL,
      startBlockTime = expectedStartBlockTime,
    )
    val expectedBlobs = listOf(
      blobRecord4,
      blobRecord5,
      blobRecord6,
      blobRecord7,
    )
    val deletedBlobs = listOf(
      blobRecord1,
      blobRecord2,
      blobRecord3,
    )

    expectedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }
    deletedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }

    blobsPostgresDao.deleteBlobsUpToEndBlockNumber(
      blobRecord3.endBlockNumber,
    ).get()

    val existedBlobRecords = blobsContentQuery().execute()
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.map(BlobsPostgresDao::parseRecord)
      }.get()

    assertThat(existedBlobRecords).hasSameElementsAs(expectedBlobs)
  }

  @Test
  fun `deleteBlobsAfterBlockNumber deletes the target record correctly`() {
    val blobRecord1 = createBlobRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 40UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord2 = createBlobRecord(
      startBlockNumber = 41UL,
      endBlockNumber = 60UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord3 = createBlobRecord(
      startBlockNumber = 61UL,
      endBlockNumber = 100UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord4 = createBlobRecord(
      startBlockNumber = 101UL,
      endBlockNumber = 111UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord5 = createBlobRecord(
      startBlockNumber = 112UL,
      endBlockNumber = 132UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord6 = createBlobRecord(
      startBlockNumber = 133UL,
      endBlockNumber = 156UL,
      startBlockTime = expectedStartBlockTime,
    )
    val blobRecord7 = createBlobRecord(
      startBlockNumber = 157UL,
      endBlockNumber = 189UL,
      startBlockTime = expectedStartBlockTime,
    )
    val deletedBlobs = listOf(
      blobRecord4,
      blobRecord5,
      blobRecord6,
      blobRecord7,
    )
    val expectedBlobs = listOf(
      blobRecord1,
      blobRecord2,
      blobRecord3,
    )

    expectedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }
    deletedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }

    blobsPostgresDao.deleteBlobsAfterBlockNumber(
      blobRecord3.endBlockNumber,
    ).get()

    val existedBlobRecords = blobsContentQuery().execute()
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.map(BlobsPostgresDao::parseRecord)
      }.get()

    assertThat(existedBlobRecords).hasSameElementsAs(expectedBlobs)
  }

  private fun saveBlobs(blobRecords: List<BlobRecord>) {
    SafeFuture.collectAll(blobRecords.map(blobsPostgresDao::saveNewBlob).stream()).get()
  }
}
