package linea.persistence.db.test

import io.vertx.core.Vertx
import io.vertx.sqlclient.Pool
import io.vertx.sqlclient.SqlClient
import linea.persistence.db.Db
import linea.persistence.db.Db.vertxConnectionPool
import linea.persistence.db.Db.vertxSqlClient
import linea.persistence.db.DbHelper
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.postgresql.ds.PGSimpleDataSource
import java.util.concurrent.TimeUnit
import javax.sql.DataSource

abstract class CleanDbTestSuiteParallel {
  private val host = "localhost"
  private val port = 5432
  private val username = "postgres"
  private val password = "postgres"
  abstract val databaseName: String
  lateinit var pool: Pool
  lateinit var sqlClient: SqlClient
  var target: String = "1"
  var migrationLocations: String = "classpath:db/"

  private fun createDataSource(databaseName: String): DataSource {
    return PGSimpleDataSource().also {
      it.serverNames = arrayOf(host)
      it.portNumbers = intArrayOf(port)
      it.databaseName = databaseName
      it.user = username
      it.password = password
    }
  }

  @BeforeEach
  open fun setUp(vertx: Vertx) {
    val dbCreationDataSource = createDataSource("postgres")
    DbHelper.createDataBase(dbCreationDataSource, databaseName)
    val dataSource = createDataSource(databaseName)

    pool = vertxConnectionPool(vertx, host, port, databaseName, username, password)
    sqlClient = vertxSqlClient(vertx, host, port, databaseName, username, password)
    // drop flyway db metadata table as well
    // to recreate new db tables;
    Db.applyDbMigrations(dataSource, target, migrationLocations)
  }

  @AfterEach
  open fun tearDown() {
    val closes = listOf(
      pool.close().onFailure { System.err.println("Error closing connection pool: " + it.message) },
      sqlClient.close().onFailure { System.err.println("Error closing sqlclient " + it.message) },
    )
    closes.forEach { runCatching { it.toCompletionStage().toCompletableFuture().get(10, TimeUnit.SECONDS) } }
    // Each test creates its own database, so drop it: otherwise they pile up and exhaust Postgres' shared memory.
    DbHelper.dropDataBase(createDataSource("postgres"), databaseName)
  }
}
