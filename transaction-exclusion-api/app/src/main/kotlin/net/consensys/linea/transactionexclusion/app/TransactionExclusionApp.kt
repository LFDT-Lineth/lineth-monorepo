package net.consensys.linea.transactionexclusion.app

import com.sksamuel.hoplite.Masked
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import io.vertx.sqlclient.SqlClient
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.async.toVertxFuture
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.transactionexclusion.TransactionExclusionServiceV1
import net.consensys.linea.transactionexclusion.app.api.Api
import net.consensys.linea.transactionexclusion.app.api.ApiConfig
import net.consensys.linea.transactionexclusion.service.RejectedTransactionCleanupService
import net.consensys.linea.transactionexclusion.service.TransactionExclusionServiceV1Impl
import net.consensys.linea.vertx.loadVertxConfig
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsDao
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsPostgresDao
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RetryingRejectedTransactionsPostgresDao
import net.consensys.zkevm.persistence.db.Db
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import org.apache.logging.log4j.LogManager
import java.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration
import kotlin.time.toKotlinDuration

data class DbConnectionConfig(
  val host: String,
  val port: Int,
  val username: String,
  val password: Masked
)

data class DbCleanupConfig(
  val pollingInterval: Duration,
  val storagePeriod: Duration
)

data class DatabaseConfig(
  val read: DbConnectionConfig,
  val write: DbConnectionConfig,
  val cleanup: DbCleanupConfig,
  val persistenceRetry: PersistenceRetryConfig,
  val schema: String = "linea_transaction_exclusion",
  val readPoolSize: Int = 10,
  val readPipeliningLimit: Int = 10,
  val transactionalPoolSize: Int = 10
)

data class AppConfig(
  val api: ApiConfig,
  val database: DatabaseConfig,
  val dataQueryableWindowSinceRejectedTimestamp: Duration
)

data class PersistenceRetryConfig(
  val maxRetries: Int? = null,
  val backoffDelay: Duration = 1.seconds.toJavaDuration(),
  val timeout: Duration? = 20.seconds.toJavaDuration()
)

class TransactionExclusionApp(config: AppConfig) {
  private val log = LogManager.getLogger(TransactionExclusionApp::class.java)
  private val meterRegistry: MeterRegistry
  private val vertx: Vertx
  private val api: Api
  private val sqlReadClient: SqlClient
  private val sqlWriteClient: SqlClient
  private val rejectedTransactionsRepository: RejectedTransactionsDao
  private val transactionExclusionService: TransactionExclusionServiceV1
  private val rejectedTransactionCleanupService: RejectedTransactionCleanupService
  private val micrometerMetricsFacade: MicrometerMetricsFacade
  val apiBindedPort: Int
    get() = api.bindedPort

  init {
    log.debug("System properties: {}", System.getProperties())
    val vertxConfig = loadVertxConfig()
    log.debug("Vertx full configs: {}", vertxConfig)
    log.info("App configs: {}", config)
    this.vertx = Vertx.vertx(vertxConfig)
    this.meterRegistry = BackendRegistries.getDefaultNow()
    this.micrometerMetricsFacade = MicrometerMetricsFacade(meterRegistry, "linea")
    this.sqlReadClient = initDb(
      connectionConfig = config.database.read,
      schema = config.database.schema,
      transactionalPoolSize = config.database.transactionalPoolSize,
      readPipeliningLimit = config.database.readPipeliningLimit,
      skipMigration = true
    )
    this.sqlWriteClient = initDb(
      connectionConfig = config.database.write,
      schema = config.database.schema,
      transactionalPoolSize = config.database.transactionalPoolSize,
      readPipeliningLimit = config.database.readPipeliningLimit
    )
    this.rejectedTransactionsRepository = RetryingRejectedTransactionsPostgresDao(
      delegate = RejectedTransactionsPostgresDao(
        readConnection = this.sqlReadClient,
        writeConnection = this.sqlWriteClient
      ),
      persistenceRetryer = PersistenceRetryer(
        vertx = vertx,
        config = PersistenceRetryer.Config(
          backoffDelay = config.database.persistenceRetry.backoffDelay.toKotlinDuration(),
          maxRetries = config.database.persistenceRetry.maxRetries,
          timeout = config.database.persistenceRetry.timeout?.toKotlinDuration()
        )
      )
    )
    this.transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = TransactionExclusionServiceV1Impl.Config(
        config.dataQueryableWindowSinceRejectedTimestamp.toKotlinDuration()
      ),
      repository = this.rejectedTransactionsRepository,
      metricsFacade = this.micrometerMetricsFacade
    )
    this.rejectedTransactionCleanupService = RejectedTransactionCleanupService(
      config = RejectedTransactionCleanupService.Config(
        pollingInterval = config.database.cleanup.pollingInterval.toKotlinDuration(),
        storagePeriod = config.database.cleanup.storagePeriod.toKotlinDuration()
      ),
      repository = this.rejectedTransactionsRepository,
      vertx = this.vertx
    )
    this.api =
      Api(
        configs = config.api,
        vertx = vertx,
        meterRegistry = meterRegistry,
        transactionExclusionService = transactionExclusionService
      )
  }

  fun start(): Future<*> {
    log.info("Starting up app..")
    return api.start().toSafeFuture()
      .thenCompose { rejectedTransactionCleanupService.start() }
      .thenPeek {
        log.info("App successfully started")
      }.toVertxFuture()
  }

  fun stop(): Future<*> {
    log.info("Shooting down app..")
    return api.stop().toSafeFuture()
      .thenCompose { rejectedTransactionCleanupService.stop() }
      .thenPeek {
        log.info("App successfully stopped")
      }.toVertxFuture()
  }

  private fun initDb(
    connectionConfig: DbConnectionConfig,
    schema: String,
    transactionalPoolSize: Int,
    readPipeliningLimit: Int,
    skipMigration: Boolean = false
  ): SqlClient {
    val dbVersion = "3"
    if (!skipMigration) {
      Db.applyDbMigrations(
        host = connectionConfig.host,
        port = connectionConfig.port,
        database = schema,
        target = dbVersion,
        username = connectionConfig.username,
        password = connectionConfig.password.value
      )
    }
    return Db.vertxSqlClient(
      vertx = vertx,
      host = connectionConfig.host,
      port = connectionConfig.port,
      database = schema,
      username = connectionConfig.username,
      password = connectionConfig.password.value,
      maxPoolSize = transactionalPoolSize,
      pipeliningLimit = readPipeliningLimit
    )
  }
}
