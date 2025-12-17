package net.consensys.zkevm.persistence.db

import io.vertx.core.Vertx
import io.vertx.pgclient.PgBuilder
import io.vertx.pgclient.PgConnectOptions
import io.vertx.sqlclient.Pool
import io.vertx.sqlclient.PoolOptions
import io.vertx.sqlclient.SqlClient
import org.apache.logging.log4j.LogManager
import org.flywaydb.core.Flyway
import org.postgresql.ds.PGSimpleDataSource
import java.time.Instant
import javax.sql.DataSource

object Db {
  private val LOG = LogManager.getLogger()
  private const val DEFAULT_VERTX_CONNECTION_POOL_MAX_SIZE = 10

  /**
   * Applies Flyway DB migration scripts on the specified database
   *
   * @param dataSource datasource
   */
  fun applyDbMigrations(
    host: String,
    port: Int,
    database: String,
    target: String,
    username: String,
    password: String,
    migrationLocations: String = "classpath:db/",
  ) {
    val dataSource =
      PGSimpleDataSource().apply {
        this.serverNames = arrayOf(host)
        this.portNumbers = intArrayOf(port)
        this.user = username
        this.password = password
        this.databaseName = database
      }
    applyDbMigrations(dataSource, target, migrationLocations)
  }

  /**
   * Applies Flyway DB migration scripts on the specified datasource
   *
   * @param dataSource datasource
   */
  fun applyDbMigrations(dataSource: DataSource, target: String, migrationLocations: String = "classpath:db/") {
    LOG.info("Migrating coordinator database")
    Flyway.configure()
      .dataSource(dataSource)
      .locations(migrationLocations)
      .baselineDescription("Migration baseline from legacy database generated ${Instant.now()}")
      .table("schema_version")
      .target(target)
      .load()
      .migrate()
    LOG.info("Coordinator database migration completed")
  }

  /**
   * Creates a new connection pool to be used in a Vertx context
   *
   * @param vertx vertx context
   * @param host database hostname
   * @param port database port
   * @param database database name/schema
   * @param username database username
   * @param password database password
   * @param maxPoolSize maximum pool size
   * @return connection pool
   */
  fun vertxConnectionPool(
    vertx: Vertx,
    host: String?,
    port: Int,
    database: String,
    username: String,
    password: String,
    maxPoolSize: Int = DEFAULT_VERTX_CONNECTION_POOL_MAX_SIZE,
  ): Pool {
    val connectOptions =
      PgConnectOptions()
        .setPort(port)
        .setHost(host)
        .setDatabase(database)
        .setUser(username)
        .setPassword(password)
        .setCachePreparedStatements(true)
        .setPreparedStatementCacheMaxSize(512)
        .setPreparedStatementCacheSqlLimit(20488 * 2)
    val poolOptions = PoolOptions().setMaxSize(maxPoolSize)
    return PgBuilder.pool().with(poolOptions).connectingTo(connectOptions).using(vertx).build()
  }

  /**
   * Creates a new connection pool to be used in a Vertx context
   *
   * @param vertx vertx context
   * @param host database hostname
   * @param port database port
   * @param database database name/schema
   * @param username database username
   * @param password database password
   * @param maxPoolSize maximum pool size
   * @param pipeliningLimit pipelining limit per connection
   * @return connection pool
   */
  fun vertxSqlClient(
    vertx: Vertx,
    host: String,
    port: Int,
    database: String,
    username: String,
    password: String,
    maxPoolSize: Int = DEFAULT_VERTX_CONNECTION_POOL_MAX_SIZE,
    pipeliningLimit: Int = PgConnectOptions.DEFAULT_PIPELINING_LIMIT,
  ): SqlClient {
    val connectOptions =
      PgConnectOptions()
        .setPort(port)
        .setHost(host)
        .setDatabase(database)
        .setUser(username)
        .setPipeliningLimit(pipeliningLimit)
        .setPassword(password)
        .setCachePreparedStatements(true)
        .setPreparedStatementCacheMaxSize(512)
        .setPreparedStatementCacheSqlLimit(20488 * 2)
    val poolOptions = PoolOptions().setMaxSize(maxPoolSize)
    return PgBuilder.client().with(poolOptions).connectingTo(connectOptions).using(vertx).build()
  }
}
