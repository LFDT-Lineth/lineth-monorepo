package net.consensys.zkevm.persistence.dao.aggregation

import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import linea.persistence.ftx.ForcedTransactionsDao
import linea.persistence.ftx.PostgresForcedTransactionsDao
import net.consensys.FakeFixedClock
import net.consensys.linea.async.get
import net.consensys.zkevm.domain.ForcedTransactionRecordFactory
import net.consensys.zkevm.domain.createAggregation
import net.consensys.zkevm.domain.createBatch
import net.consensys.zkevm.domain.createBlobRecordFromBatches
import net.consensys.zkevm.ethereum.finalization.FinalizationMonitor
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import net.consensys.zkevm.persistence.dao.batch.persistence.BatchesPostgresDao
import net.consensys.zkevm.persistence.dao.batch.persistence.PostgresBatchesRepository
import net.consensys.zkevm.persistence.dao.blob.BlobsPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsRepositoryImpl
import net.consensys.zkevm.persistence.db.DbHelper
import net.consensys.zkevm.persistence.db.test.CleanDbTestSuiteParallel
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock

@ExtendWith(VertxExtension::class)
class RecordsCleanupFinalizationHandlerTest : CleanDbTestSuiteParallel() {
  init {
    target = "5"
  }

  override val databaseName = DbHelper.generateUniqueDbName("records-cleanup-on-finalization")
  private var fakeClock = FakeFixedClock(Clock.System.now())
  private lateinit var batchesRepository: BatchesRepository
  private lateinit var blobsRepository: BlobsRepository
  private lateinit var aggregationsRepository: AggregationsRepository
  private lateinit var forcedTransactionsDao: ForcedTransactionsDao

  private lateinit var recordsCleanupFinalizationHandler: RecordsCleanupFinalizationHandler

  @BeforeEach
  fun beforeEach() {
    fakeClock.setTimeTo(Clock.System.now())
    batchesRepository = PostgresBatchesRepository(
      BatchesPostgresDao(
        connection = sqlClient,
        clock = fakeClock,
      ),
    )
    blobsRepository = BlobsRepositoryImpl(
      BlobsPostgresDao(
        config = BlobsPostgresDao.Config(maxBlobsToReturn = 10u),
        connection = sqlClient,
        clock = fakeClock,
      ),
    )
    aggregationsRepository = AggregationsRepositoryImpl(
      PostgresAggregationsDao(
        connection = sqlClient,
        clock = fakeClock,
      ),
    )

    forcedTransactionsDao = PostgresForcedTransactionsDao(
      connection = sqlClient,
      clock = fakeClock,
    )

    recordsCleanupFinalizationHandler = RecordsCleanupFinalizationHandler(
      batchesRepository,
      blobsRepository,
      aggregationsRepository,
      forcedTransactionsDao,
    )
  }

  private fun batchesContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from ${BatchesPostgresDao.batchesTableName}")

  private fun blobsContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from ${BlobsPostgresDao.TableName}")

  private fun aggregationsContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from ${PostgresAggregationsDao.aggregationsTable}")

  val batch1 = createBatch(startBlockNumber = 1L, endBlockNumber = 10L)
  val batch2 = createBatch(startBlockNumber = 11L, endBlockNumber = 15L)
  val batch3 = createBatch(startBlockNumber = 16L, endBlockNumber = 20L)
  val batch4 = createBatch(startBlockNumber = 21L, endBlockNumber = 21L)

  val batches = listOf(batch1, batch2, batch3, batch4)

  val blob1 = createBlobRecordFromBatches(listOf(batch1, batch2))
  val blob2 = createBlobRecordFromBatches(listOf(batch3))
  val blob3 = createBlobRecordFromBatches(listOf(batch4))

  val blobs = listOf(blob1, blob2, blob3)

  val aggregation1 = createAggregation(
    startBlockNumber = blob1.startBlockNumber.toLong(),
    endBlockNumber = blob1.endBlockNumber.toLong(),
    batchCount = blob1.batchesCount.toLong(),
  )
  val aggregation2 = createAggregation(
    startBlockNumber = blob2.startBlockNumber.toLong(),
    endBlockNumber = blob2.endBlockNumber.toLong(),
    batchCount = blob2.batchesCount.toLong(),
  )

  val aggregation3 = createAggregation(
    startBlockNumber = blob3.startBlockNumber.toLong(),
    endBlockNumber = blob3.endBlockNumber.toLong(),
    batchCount = blob3.batchesCount.toLong(),
  )

  val aggregations = listOf(aggregation1, aggregation2, aggregation3)

  val ftx1 = ForcedTransactionRecordFactory.createForcedTransactionRecord(ftxNumber = 1UL)
  val ftx2 = ForcedTransactionRecordFactory.createForcedTransactionRecord(ftxNumber = 2UL)
  val ftx3 = ForcedTransactionRecordFactory.createForcedTransactionRecord(ftxNumber = 3UL)
  val ftx4 = ForcedTransactionRecordFactory.createForcedTransactionRecord(ftxNumber = 4UL)

  val forcedTransactions = listOf(ftx1, ftx2, ftx3, ftx4)

  private fun setup() {
    SafeFuture.allOf(
      batchesRepository.saveNewBatch(batch1),
      batchesRepository.saveNewBatch(batch2),
      batchesRepository.saveNewBatch(batch3),
      batchesRepository.saveNewBatch(batch4),

      blobsRepository.saveNewBlob(blob1),
      blobsRepository.saveNewBlob(blob2),
      blobsRepository.saveNewBlob(blob3),

      aggregationsRepository.saveNewAggregation(aggregation1),
      aggregationsRepository.saveNewAggregation(aggregation2),
      aggregationsRepository.saveNewAggregation(aggregation3),

      forcedTransactionsDao.save(ftx1),
      forcedTransactionsDao.save(ftx2),
      forcedTransactionsDao.save(ftx3),
      forcedTransactionsDao.save(ftx4),
    ).get()
  }

  @Test
  fun `verify that cleanup on block finalization does not delete last blob, aggregation, and ftx`() {
    setup()
    val update = FinalizationMonitor.FinalizationUpdate(
      blockNumber = 21u,
      blockHash = Bytes32.random(),
      forcedTransactionNumber = 3UL,
    )

    val batchesBeforeCleanup = batchesContentQuery().execute().get()
    val blobsBeforeCleanup = blobsContentQuery().execute().get()
    val aggregationsBeforeCleanup = aggregationsContentQuery().execute().get()
    val forcedTransactionsBeforeCleanup = forcedTransactionsDao.list().get()

    Assertions.assertThat(batchesBeforeCleanup.size()).isEqualTo(batches.size)
    Assertions.assertThat(blobsBeforeCleanup.size()).isEqualTo(blobs.size)
    Assertions.assertThat(aggregationsBeforeCleanup.size()).isEqualTo(aggregations.size)
    Assertions.assertThat(forcedTransactionsBeforeCleanup.size).isEqualTo(forcedTransactions.size)

    recordsCleanupFinalizationHandler.handleUpdate(update).get()

    val batchesAfterCleanup = batchesContentQuery().execute().get()
    Assertions.assertThat(batchesAfterCleanup.size()).isEqualTo(0)

    val blobsAfterCleanup = blobsContentQuery().execute().get()
      .map { BlobsPostgresDao.parseRecord(it) }
      .sortedBy { it.startBlockNumber }
    Assertions.assertThat(blobsAfterCleanup.size).isEqualTo(1)
    Assertions.assertThat(blobsAfterCleanup[0]).isEqualTo(blob3)

    val aggregationsAfterCleanup = aggregationsContentQuery().execute().get()
    Assertions.assertThat(aggregationsAfterCleanup.size()).isEqualTo(1)
    Assertions.assertThat(
      aggregationsRepository.findAggregationProofByEndBlockNumber(aggregation3.endBlockNumber.toLong()).get(),
    )
      .isNotNull()

    val forcedTransactionsAfterCleanup = forcedTransactionsDao.list().get()
    Assertions.assertThat(forcedTransactionsAfterCleanup.map { it.ftxNumber })
      .isEqualTo(listOf(3UL, 4UL))
  }
}
