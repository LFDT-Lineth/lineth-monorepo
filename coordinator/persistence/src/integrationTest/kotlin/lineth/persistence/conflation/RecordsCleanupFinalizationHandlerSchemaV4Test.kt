package lineth.persistence.conflation

import io.vertx.junit5.VertxExtension
import linea.domain.createAggregation
import linea.domain.createBatch
import linea.domain.createBlobRecordFromBatches
import linea.persistence.db.DbHelper
import linea.persistence.db.test.CleanDbTestSuiteParallel
import lineth.finalization.FinalizationMonitor
import lineth.persistence.ftx.PostgresForcedTransactionsDao
import net.consensys.linea.async.get
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith

/** Finalization cleanup on schema 4, which has no RISC-V rollup tables, so the handler gets no rollups DAO. */
@ExtendWith(VertxExtension::class)
class RecordsCleanupFinalizationHandlerSchemaV4Test : CleanDbTestSuiteParallel() {
  init {
    target = "4"
  }

  override val databaseName = DbHelper.generateUniqueDbName("records-cleanup-schema-v4")

  @Test
  fun `cleanup without a rollups DAO cleans up batches, blobs and aggregations`() {
    val batchesRepository = PostgresBatchesRepository(BatchesPostgresDao(connection = sqlClient, schemaVersion = 4))
    val blobsRepository = BlobsRepositoryImpl(
      BlobsPostgresDao(config = BlobsPostgresDao.Config(maxBlobsToReturn = 10u), connection = sqlClient),
    )
    val aggregationsRepository = AggregationsRepositoryImpl(PostgresAggregationsDao(connection = sqlClient))
    val handler = RecordsCleanupFinalizationHandler(
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository,
      // never called: the update below carries no forced transaction
      forcedTransactionsDao = PostgresForcedTransactionsDao(connection = sqlClient),
      riscvRollupsDao = null,
    )
    val batch1 = createBatch(startBlockNumber = 1L, endBlockNumber = 10L)
    val batch2 = createBatch(startBlockNumber = 11L, endBlockNumber = 20L)
    val blob1 = createBlobRecordFromBatches(listOf(batch1))
    val blob2 = createBlobRecordFromBatches(listOf(batch2))
    listOf(batch1, batch2).forEach { batchesRepository.saveNewBatch(it).get() }
    listOf(blob1, blob2).forEach { blobsRepository.saveNewBlob(it).get() }
    listOf(blob1, blob2).forEach {
      aggregationsRepository.saveNewAggregation(
        createAggregation(
          startBlockNumber = it.startBlockNumber.toLong(),
          endBlockNumber = it.endBlockNumber.toLong(),
          batchCount = it.batchesCount.toLong(),
        ),
      ).get()
    }

    handler.handleUpdate(
      FinalizationMonitor.FinalizationUpdate(
        blockNumber = 20u,
        blockHash = Bytes32.random(),
        forcedTransactionNumber = 0UL,
      ),
    ).get()

    assertThat(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L).get()).isNull()
    assertThat(blobsRepository.findBlobByStartBlockNumber(1L).get()).isNull()
    assertThat(blobsRepository.findBlobByStartBlockNumber(11L).get()).isEqualTo(blob2)
    assertThat(aggregationsRepository.findAggregationProofByEndBlockNumber(10L).get()).isNull()
    assertThat(aggregationsRepository.findAggregationProofByEndBlockNumber(20L).get()).isNotNull()
  }
}
