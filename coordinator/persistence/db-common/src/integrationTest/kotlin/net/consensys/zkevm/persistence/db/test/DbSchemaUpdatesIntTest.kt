package net.consensys.zkevm.persistence.db.test

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.Pool
import io.vertx.sqlclient.SqlClient
import linea.kotlin.encodeHex
import net.consensys.linea.async.get
import net.consensys.zkevm.persistence.db.Db
import net.consensys.zkevm.persistence.db.DbHelper
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
    pool.close { ar: io.vertx.core.AsyncResult<Void?> ->
      if (ar.failed()) {
        System.err.println("Error closing connection pool: " + ar.cause().message)
      }
    }
    sqlClient.close { ar: io.vertx.core.AsyncResult<Void?> ->
      if (ar.failed()) {
        System.err.println("Error closing sqlclient " + ar.cause().message)
      }
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
}
