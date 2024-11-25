package net.consensys.zkevm.persistence.db.test

import io.vertx.core.Vertx
import io.vertx.sqlclient.Pool
import io.vertx.sqlclient.SqlClient
import net.consensys.zkevm.persistence.db.Db
import net.consensys.zkevm.persistence.db.Db.vertxConnectionPool
import net.consensys.zkevm.persistence.db.Db.vertxSqlClient
import net.consensys.zkevm.persistence.db.DbHelper
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.postgresql.ds.PGSimpleDataSource
import javax.sql.DataSource

abstract class CleanDbTestSuiteParallel {
  private val host = "localhost"
  private val port = 5432
  private val username = "postgres"
  private val password = "postgres"
  abstract val databaseName: String
  private lateinit var dataSource: DataSource
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
    dataSource = createDataSource(databaseName)

    pool = vertxConnectionPool(vertx, host, port, databaseName, username, password)
    sqlClient = vertxSqlClient(vertx, host, port, databaseName, username, password)
    // drop flyway db metadata table as well
    // to recreate new db tables;
    Db.applyDbMigrations(dataSource, target, migrationLocations)
  }

  @AfterEach
  open fun tearDown() {
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
    DbHelper.resetAllConnections(dataSource, databaseName)
  }
}
