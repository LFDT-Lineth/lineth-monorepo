package lineth.persistence.test

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.Pool
import io.vertx.sqlclient.SqlClient
import linea.domain.Batch
import linea.kotlin.encodeHex
import linea.persistence.db.Db
import linea.persistence.db.DbHelper
import lineth.persistence.conflation.BatchesPostgresDao
import net.consensys.linea.async.get
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.postgresql.ds.PGSimpleDataSource
import java.util.concurrent.ExecutionException
import javax.sql.DataSource
import kotlin.time.Clock

@ExtendWith(VertxExtension::class)
class DbSchemaUpdatesIntTest {

  private val host = "localhost"
  private val port = 5432
  private val databaseName = DbHelper.generateUniqueDbName("coordinator-db-migration-tests")
  private val username = "postgres"
  private val password = "postgres"

  private lateinit var dataSource: DataSource
  private lateinit var pool: Pool
  private lateinit var sqlClient: SqlClient

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    val dbCreationDataSource = PGSimpleDataSource().also {
      it.serverNames = arrayOf(host)
      it.portNumbers = intArrayOf(port)
      it.databaseName = "postgres"
      it.user = username
      it.password = password
    }
    DbHelper.createDataBase(dbCreationDataSource, databaseName)
    dataSource =
      PGSimpleDataSource().also {
        it.serverNames = arrayOf(host)
        it.portNumbers = intArrayOf(port)
        it.databaseName = databaseName
        it.user = username
        it.password = password
      }
    pool = Db.vertxConnectionPool(vertx, host, port, databaseName, username, password)
    sqlClient = Db.vertxSqlClient(vertx, host, port, databaseName, username, password)
  }

  @AfterEach
  fun tearDown() {
    DbHelper.resetAllConnections(dataSource, databaseName)
    pool.close()
      .onFailure { th ->
        System.err.println("Error closing connection pool: " + th.message)
      }
    sqlClient.close().onFailure { th ->
      System.err.println("Error closing sqlclient " + th.message)
    }
  }

  @Test
  fun migrateDbWithSpecificVersion1() {
    val schemaTarget = "1"

    DbHelper.dropAllTables(dataSource)
    Db.applyDbMigrations(
      dataSource = dataSource,
      target = schemaTarget,
    )

    val paramsV1 = listOf(
      Clock.System.now().toEpochMilliseconds(),
      0L,
      1L,
      "0.1.0",
      1,
    )

    val paramsV2 = listOf(
      Clock.System.now().toEpochMilliseconds(),
      0L,
      1L,
      "0.1.0",
      "0.1.0",
      1,
    )

    DbQueries.insertBatch(sqlClient, DbQueries.insertBatchQueryV1, paramsV1).get()
    assertThat(DbQueries.getTableContent(sqlClient, DbQueries.batchesTable).execute().get().size()).isEqualTo(1)

    assertThrows<ExecutionException> {
      DbQueries.insertBatch(sqlClient, DbQueries.insertBatchQueryV2, paramsV2).get()
    }
    assertThat(DbQueries.getTableContent(sqlClient, DbQueries.batchesTable).execute().get().size()).isEqualTo(1)
  }

  @Test
  fun migrateDbWithSpecificVersion2() {
    val schemaTarget = "2"

    DbHelper.dropAllTables(dataSource)
    Db.applyDbMigrations(
      dataSource = dataSource,
      target = schemaTarget,
    )

    val batchParamsV2 = listOf(
      Clock.System.now().toEpochMilliseconds(),
      0L,
      1L,
      "0.1.0",
      "0.1.0",
      1,
    )

    DbQueries.insertBatch(sqlClient, DbQueries.insertBatchQueryV2, batchParamsV2).get()
    assertThat(DbQueries.getTableContent(sqlClient, DbQueries.batchesTable).execute().get().size()).isEqualTo(1)

    assertThat(DbQueries.getTableContent(sqlClient, DbQueries.batchesTable).execute().get().size()).isEqualTo(1)

    val blobParams = listOf(
      Clock.System.now().toEpochMilliseconds(),
      0L,
      1L,
      "0.1.0",
      "12345",
      1,
      Clock.System.now().toEpochMilliseconds(),
      Clock.System.now().toEpochMilliseconds(),
      3,
      ByteArray(32).encodeHex(),
      "{}",
    )

    DbQueries.insertBlob(sqlClient, DbQueries.insertBlobQuery, blobParams).get()
    assertThat(DbQueries.getTableContent(sqlClient, DbQueries.blobsTable).execute().get().size()).isEqualTo(1)
  }

  @Test
  fun `revert schemaVersion from 5 to 4 keeps db usable with v4 batches dao`() {
    val proofIndexHash = ByteArray(32) { it.toByte() }

    DbHelper.dropAllTables(dataSource)
    Db.applyDbMigrations(dataSource = dataSource, target = "5")
    BatchesPostgresDao(connection = sqlClient, schemaVersion = 5)
      .saveNewBatch(Batch(startBlockNumber = 1UL, endBlockNumber = 10UL, proofIndexHash = proofIndexHash))
      .get()

    // simulates coordinator restart with schemaVersion = 4: must not fail and must not undo V5
    Db.applyDbMigrations(dataSource = dataSource, target = "4")
    val appliedVersions = sqlClient
      .preparedQuery("select version from schema_version where success order by installed_rank")
      .execute()
      .get()
      .map { it.getString("version") }
    assertThat(appliedVersions).contains("005")

    val batchesDaoV4 = BatchesPostgresDao(connection = sqlClient, schemaVersion = 4)
    batchesDaoV4
      .saveNewBatch(Batch(startBlockNumber = 11UL, endBlockNumber = 20UL, proofIndexHash = proofIndexHash))
      .get()
    batchesDaoV4
      .saveNewBatch(Batch(startBlockNumber = 21UL, endBlockNumber = 30UL))
      .get()

    // v5 column is still there; rows written by the v4 dao leave it null
    assertThat(DbQueries.getBatches(sqlClient).sortedBy { it.startBlockNumber }.map { it.proofIndexHash?.encodeHex() })
      .containsExactly(proofIndexHash.encodeHex(), null, null)

    assertThat(batchesDaoV4.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L).get()).isEqualTo(30L)
    assertThat(batchesDaoV4.deleteBatchesAfterBlockNumber(startingBlockNumberInclusive = 21L).get()).isEqualTo(1)
    assertThat(batchesDaoV4.deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive = 10L).get()).isEqualTo(1)
    assertThat(DbQueries.getBatches(sqlClient).map { it.startBlockNumber }).containsExactly(11UL)

    // rolling forward to 5 again after the revert also works
    Db.applyDbMigrations(dataSource = dataSource, target = "5")
    BatchesPostgresDao(connection = sqlClient, schemaVersion = 5)
      .saveNewBatch(Batch(startBlockNumber = 31UL, endBlockNumber = 40UL, proofIndexHash = proofIndexHash))
      .get()
    assertThat(DbQueries.getBatches(sqlClient).first { it.startBlockNumber == 31UL }.proofIndexHash)
      .isEqualTo(proofIndexHash)
  }
}
