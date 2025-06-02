package net.consensys.zkevm.persistence.db

import java.time.Clock
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import javax.sql.DataSource

class DuplicatedRecordException(message: String? = null, cause: Throwable? = null) : Exception(message, cause)

object DbHelper {
  private const val POSTGRES_MAX_DB_NAME_LENGTH = 63
  private var formatter: DateTimeFormatter = DateTimeFormatter.ofPattern("yyyyMMddHHmmss")

  fun generateUniqueDbName(
    prefix: String = "test",
    clock: Clock = Clock.systemUTC(),
  ): String {
    // Just time is not enough, as we can have multiple tests running in parallel
    val dateStr = formatter.format(clock.instant().atZone(ZoneId.of("UTC")))
    val randomSuffix = java.util.UUID.randomUUID().toString().take(8)
    val dbName = """${prefix}_${dateStr}_$randomSuffix""".replace("-", "_")

    require(dbName.length <= POSTGRES_MAX_DB_NAME_LENGTH) {
      "Generated db name is too long max $POSTGRES_MAX_DB_NAME_LENGTH: generatedName=$dbName(${dbName.length})"
    }
    return dbName
  }

  fun dropTables(dataSource: DataSource, vararg tableNames: String?) {
    val sql = "DROP TABLE IF EXISTS " + java.lang.String.join(",", *tableNames)
    dataSource.connection.use { con -> con.prepareStatement(sql).execute() }
  }

  fun resetAllConnections(dataSource: DataSource, db: String?) {
    val sql = "select pg_terminate_backend(pid) from pg_stat_activity where datname=?"
    try {
      dataSource.connection.use { con ->
        val ps = con.prepareStatement(sql)
        ps.setString(1, db)
        ps.execute()
      }
    } catch (ignored: Exception) {
      // The current connection will be dropped as well and the exception will be thrown
    }
  }

  fun dropAllTablesForce(dataSource: DataSource, databaseName: String?) {
    resetAllConnections(dataSource, databaseName)
    dropAllTables(dataSource)
  }

  fun dropAllTables(dataSource: DataSource) {
    val sql = "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
    dataSource.connection.use { con -> con.prepareStatement(sql).execute() }
  }

  fun createDataBase(
    dataSource: DataSource,
    databaseName: String,
  ) {
    dataSource.connection
      .use { con -> con.prepareStatement("CREATE DATABASE $databaseName;").execute() }
  }

  fun dropAndCreateDataBase(
    dataSource: DataSource,
    databaseName: String,
  ) {
    resetAllConnections(dataSource, databaseName)
    dataSource.connection
      .use { con ->
        con.prepareStatement(
          "DROP DATABASE IF EXISTS  $databaseName; CREATE DATABASE $databaseName;",
        ).execute()
      }
  }
}
