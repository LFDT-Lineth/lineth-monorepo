package lineth.persistence.conflation

import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import io.vertx.sqlclient.SqlClient
import linea.domain.BlobStatus
import linea.error.DuplicatedRecordException
import linea.kotlin.trimToMillisecondPrecision
import linea.kotlin.trimToSecondPrecision
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
import kotlin.time.Instant
import kotlin.time.toJavaDuration

/**
 * Shared integration tests for the [BlobsDaoG] implementations. Concrete subclasses only provide the
 * record-type-specific hooks (DAO construction, blob factory, record parsing and accessors).
 */
@ExtendWith(VertxExtension::class)
abstract class BlobsPostgresDaoTestBase<T> : CleanDbTestSuiteParallel() {
  init {
    target = "4"
  }

  protected val maxBlobsToReturn = 6u
  protected val fakeClock = FakeFixedClock()
  protected lateinit var blobsPostgresDao: BlobsDaoG<T>

  protected val expectedStartBlock = 1UL
  protected val expectedEndBlock = 100UL
  protected val expectedStartBlockTime = fakeClock.now().trimToSecondPrecision()
  protected val expectedEndBlockTime = fakeClock.now().plus(1200.seconds).trimToMillisecondPrecision()

  // --- record-type-specific hooks -----------------------------------------------------------------

  /** Build the concrete DAO under test. */
  protected abstract fun buildDao(maxBlobsToReturn: UInt, connection: SqlClient, clock: Clock): BlobsDaoG<T>

  /** Create a blob record of the type under test. */
  protected abstract fun createBlob(startBlockNumber: ULong, endBlockNumber: ULong, startBlockTime: Instant): T

  /** Parse a DB row into a blob record of the type under test. */
  protected abstract fun parseRecord(row: Row): T

  /** Access the end block time of a blob record. */
  protected abstract fun endBlockTime(blob: T): Instant

  /** Access the start block number of a blob record. */
  protected abstract fun startBlockNumber(blob: T): ULong

  /** Access the end block number of a blob record. */
  protected abstract fun endBlockNumber(blob: T): ULong

  /** Additional column assertions on a freshly inserted row (no-op by default). */
  protected open fun assertAdditionalInsertedColumns(row: Row, blob: T) {}

  // --- shared helpers ------------------------------------------------------------------------------

  private fun blobsContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from ${BlobsPostgresDaoG.TableName}")

  @BeforeEach
  fun beforeEach() {
    blobsPostgresDao = buildDao(maxBlobsToReturn, sqlClient, fakeClock)
  }

  private fun performInsertTest(blobRecord: T): RowSet<Row>? {
    blobsPostgresDao.saveNewBlob(blobRecord).get()
    val dbContent = blobsContentQuery().execute().get()
    val newlyInsertedRow =
      dbContent.find { it.getLong("created_epoch_milli") == fakeClock.now().toEpochMilliseconds() }
    assertThat(newlyInsertedRow).isNotNull

    assertThat(newlyInsertedRow!!.getLong("start_block_number"))
      .isEqualTo(startBlockNumber(blobRecord).toLong())
    assertThat(newlyInsertedRow.getLong("end_block_number"))
      .isEqualTo(endBlockNumber(blobRecord).toLong())
    assertThat(newlyInsertedRow.getInteger("status")).isEqualTo(
      BlobsPostgresDaoG.blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
    )
    assertAdditionalInsertedColumns(newlyInsertedRow, blobRecord)

    return dbContent
  }

  private fun saveBlobs(blobRecords: List<T>) {
    SafeFuture.collectAll(blobRecords.map(blobsPostgresDao::saveNewBlob).stream()).get()
  }

  private fun allBlobRecords(): List<T> =
    blobsContentQuery().execute()
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(::parseRecord) }
      .get()

  // --- tests ---------------------------------------------------------------------------------------

  @Test
  fun `saveNewBlob inserts new blob to db`() {
    val blobRecord1 = createBlob(expectedStartBlock, expectedEndBlock, expectedStartBlockTime)
    fakeClock.setTimeTo(Clock.System.now())

    val dbContent1 = performInsertTest(blobRecord1)
    assertThat(dbContent1).size().isEqualTo(1)

    val blobRecord2 = createBlob(expectedEndBlock + 1UL, expectedEndBlock + 100UL, expectedStartBlockTime)
    fakeClock.advanceBy(1.seconds)

    val dbContent2 = performInsertTest(blobRecord2)
    assertThat(dbContent2).size().isEqualTo(2)
  }

  @Test
  fun `saveNewBlob returns error when duplicated`() {
    val blobRecord1 = createBlob(expectedStartBlock, expectedEndBlock, expectedStartBlockTime)

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
    val expectedBlob = createBlob(expectedStartBlock1, expectedEndBlock1, expectedStartBlockTime)

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
    val blobRecord1 = createBlob(expectedStartBlock, expectedEndBlock, expectedStartBlockTime)
    val blobRecord2 = createBlob(expectedEndBlock + 1UL, expectedEndBlock + 100UL, expectedStartBlockTime)
    val blobRecord3 = createBlob(expectedEndBlock + 101UL, expectedEndBlock + 200UL, expectedStartBlockTime)

    saveBlobs(listOf(blobRecord1, blobRecord2, blobRecord3))

    blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
      expectedStartBlock + 1UL,
      endBlockTime(blobRecord3).plus(1.seconds),
    ).get().also { blobs ->
      assertThat(blobs).isEmpty()
    }
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber returns a sequence of blobs without gaps`() {
    val blobRecord1 = createBlob(1UL, 40UL, expectedStartBlockTime)
    val blobRecord2 = createBlob(41UL, 60UL, endBlockTime(blobRecord1).plus(3.seconds))
    val blobRecord3 = createBlob(61UL, 100UL, endBlockTime(blobRecord2).plus(3.seconds))
    val blobRecord4 = createBlob(101UL, 111UL, endBlockTime(blobRecord3).plus(3.seconds))
    val blobRecord5 = createBlob(112UL, 132UL, endBlockTime(blobRecord4).plus(3.seconds))
    val blobRecord6 = createBlob(134UL, 156UL, endBlockTime(blobRecord5).plus(3.seconds))
    val blobRecord7 = createBlob(157UL, 189UL, endBlockTime(blobRecord5).plus(3.seconds))

    val expectedBlobs = listOf(blobRecord3, blobRecord4, blobRecord5)
    val otherBlobs = listOf(blobRecord1, blobRecord2, blobRecord6, blobRecord7)

    saveBlobs(expectedBlobs + otherBlobs)

    val actualBlobs =
      blobsPostgresDao
        .getConsecutiveBlobsFromBlockNumber(
          startingBlockNumberInclusive = startBlockNumber(expectedBlobs.first()),
          endBlockCreatedBefore = endBlockTime(expectedBlobs.last()).plus(1.seconds),
        ).get()
    assertThat(actualBlobs).hasSameElementsAs(expectedBlobs)
  }

  @Test
  fun `findBlobByXBlockNumber works correctly for 1 blob`() {
    val expectedBlob = createBlob(1UL, 90UL, expectedStartBlockTime)

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
    val blobRecord1 = createBlob(1UL, 40UL, expectedStartBlockTime)
    val blobRecord2 = createBlob(41UL, 60UL, expectedStartBlockTime)
    val blobRecord3 = createBlob(61UL, 100UL, expectedStartBlockTime)
    val blobRecord4 = createBlob(101UL, 111UL, expectedStartBlockTime)
    val blobRecord5 = createBlob(112UL, 132UL, expectedStartBlockTime)
    val blobRecord6 = createBlob(133UL, 156UL, expectedStartBlockTime)
    val blobRecord7 = createBlob(157UL, 189UL, expectedStartBlockTime)

    val expectedBlobs = listOf(blobRecord4, blobRecord5, blobRecord6, blobRecord7)
    val deletedBlobs = listOf(blobRecord1, blobRecord2, blobRecord3)

    expectedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }
    deletedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }

    blobsPostgresDao.deleteBlobsUpToEndBlockNumber(endBlockNumber(blobRecord3)).get()

    assertThat(allBlobRecords()).hasSameElementsAs(expectedBlobs)
  }

  @Test
  fun `deleteBlobsAfterBlockNumber deletes the target record correctly`() {
    val blobRecord1 = createBlob(1UL, 40UL, expectedStartBlockTime)
    val blobRecord2 = createBlob(41UL, 60UL, expectedStartBlockTime)
    val blobRecord3 = createBlob(61UL, 100UL, expectedStartBlockTime)
    val blobRecord4 = createBlob(101UL, 111UL, expectedStartBlockTime)
    val blobRecord5 = createBlob(112UL, 132UL, expectedStartBlockTime)
    val blobRecord6 = createBlob(133UL, 156UL, expectedStartBlockTime)
    val blobRecord7 = createBlob(157UL, 189UL, expectedStartBlockTime)

    val deletedBlobs = listOf(blobRecord4, blobRecord5, blobRecord6, blobRecord7)
    val expectedBlobs = listOf(blobRecord1, blobRecord2, blobRecord3)

    expectedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }
    deletedBlobs.forEach { blobsPostgresDao.saveNewBlob(it).get() }

    blobsPostgresDao.deleteBlobsAfterBlockNumber(endBlockNumber(blobRecord3)).get()

    assertThat(allBlobRecords()).hasSameElementsAs(expectedBlobs)
  }
}
