package net.consensys.zkevm.coordinator.app

import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import io.vertx.sqlclient.SqlClient
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.client.LoadBalancingJsonRpcClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.vertx.loadVertxConfig
import net.consensys.zkevm.coordinator.api.Api
import net.consensys.zkevm.coordinator.app.config.CoordinatorConfig
import net.consensys.zkevm.coordinator.app.config.DatabaseConfig
import net.consensys.zkevm.fileio.DirectoryCleaner
import net.consensys.zkevm.persistence.dao.aggregation.AggregationsRepositoryImpl
import net.consensys.zkevm.persistence.dao.aggregation.PostgresAggregationsDao
import net.consensys.zkevm.persistence.dao.aggregation.RetryingPostgresAggregationsDao
import net.consensys.zkevm.persistence.dao.batch.persistence.BatchesPostgresDao
import net.consensys.zkevm.persistence.dao.batch.persistence.PostgresBatchesRepository
import net.consensys.zkevm.persistence.dao.batch.persistence.RetryingBatchesPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsRepositoryImpl
import net.consensys.zkevm.persistence.dao.blob.RetryingBlobsPostgresDao
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesPostgresDao
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesRepositoryImpl
import net.consensys.zkevm.persistence.db.Db
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toKotlinDuration

class CoordinatorApp(private val configs: CoordinatorConfig) {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val vertx: Vertx = run {
    log.trace("System properties: {}", System.getProperties())
    val vertxConfig = loadVertxConfig()
    log.debug("Vertx full configs: {}", vertxConfig)
    log.info("App configs: {}", configs)

    Vertx.vertx(vertxConfig)
  }
  private val meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow()
  private val micrometerMetricsFacade = MicrometerMetricsFacade(meterRegistry, "linea")
  private val httpJsonRpcClientFactory = VertxHttpJsonRpcClientFactory(
    vertx = vertx,
    metricsFacade = MicrometerMetricsFacade(meterRegistry),
    requestResponseLogLevel = Level.TRACE,
    failuresLogLevel = Level.WARN
  )
  private val api = Api(
    Api.Config(
      configs.api.observabilityPort
    ),
    vertx
  )
  private val l2Web3jClient: Web3j = createWeb3jHttpClient(
    rpcUrl = configs.l2.rpcEndpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth-api.rpc-node"),
    pollingInterval = 1.seconds,
    requestResponseLogLevel = Level.TRACE,
    failuresLogLevel = Level.DEBUG
  )

  private val persistenceRetryer = PersistenceRetryer(
    vertx = vertx,
    config = PersistenceRetryer.Config(
      backoffDelay = configs.persistenceRetry.backoffDelay.toKotlinDuration(),
      maxRetries = configs.persistenceRetry.maxRetries,
      timeout = configs.persistenceRetry.timeout?.toKotlinDuration()
    )
  )

  private val sqlClient: SqlClient = initDb(configs.database)
  private val batchesRepository =
    PostgresBatchesRepository(
      batchesDao = RetryingBatchesPostgresDao(
        delegate = BatchesPostgresDao(
          connection = sqlClient
        ),
        persistenceRetryer = persistenceRetryer
      )
    )

  private val blobsRepository =
    BlobsRepositoryImpl(
      blobsDao = RetryingBlobsPostgresDao(
        delegate = BlobsPostgresDao(
          config = BlobsPostgresDao.Config(
            maxBlobsToReturn = configs.blobSubmission.maxBlobsToReturn.toUInt()
          ),
          connection = sqlClient
        ),
        persistenceRetryer = persistenceRetryer
      )
    )

  private val aggregationsRepository = AggregationsRepositoryImpl(
    aggregationsPostgresDao = RetryingPostgresAggregationsDao(
      delegate = PostgresAggregationsDao(
        connection = sqlClient
      ),
      persistenceRetryer = persistenceRetryer
    )
  )

  private val l1FeeHistoriesRepository =
    FeeHistoriesRepositoryImpl(
      FeeHistoriesRepositoryImpl.Config(
        rewardPercentiles = configs.l1DynamicGasPriceCapService.feeHistoryFetcher.rewardPercentiles,
        minBaseFeePerBlobGasToCache =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.historicBaseFeePerBlobGasLowerBound,
        fixedAverageRewardToCache =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.historicAvgRewardConstant
      ),
      FeeHistoriesPostgresDao(
        sqlClient
      )
    )

  private val l1App = L1DependentApp(
    configs = configs,
    vertx = vertx,
    l2Web3jClient = l2Web3jClient,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    batchesRepository = batchesRepository,
    blobsRepository = blobsRepository,
    aggregationsRepository = aggregationsRepository,
    l1FeeHistoriesRepository = l1FeeHistoriesRepository,
    smartContractErrors = configs.conflation.smartContractErrors,
    metricsFacade = micrometerMetricsFacade
  )

  private val requestFileCleanup = DirectoryCleaner(
    vertx = vertx,
    directories = listOfNotNull(
      configs.proversConfig.proverA.execution.requestsDirectory,
      configs.proversConfig.proverA.blobCompression.requestsDirectory,
      configs.proversConfig.proverA.proofAggregation.requestsDirectory,
      configs.proversConfig.proverB?.execution?.requestsDirectory,
      configs.proversConfig.proverB?.blobCompression?.requestsDirectory,
      configs.proversConfig.proverB?.proofAggregation?.requestsDirectory
    ),
    fileFilters = DirectoryCleaner.getSuffixFileFilters(
      listOfNotNull(
        configs.proversConfig.proverA.execution.inprogressRequestWritingSuffix,
        configs.proversConfig.proverA.blobCompression.inprogressRequestWritingSuffix,
        configs.proversConfig.proverA.proofAggregation.inprogressRequestWritingSuffix,
        configs.proversConfig.proverB?.execution?.inprogressRequestWritingSuffix,
        configs.proversConfig.proverB?.blobCompression?.inprogressRequestWritingSuffix,
        configs.proversConfig.proverB?.proofAggregation?.inprogressRequestWritingSuffix
      )
    ) + DirectoryCleaner.JSON_FILE_FILTER
  )

  init {
    log.info("Coordinator app instantiated")
  }

  fun start() {
    requestFileCleanup.cleanup()
      .thenCompose { l1App.start() }
      .thenCompose { api.start().toSafeFuture() }
      .get()

    log.info("Started :)")
  }

  fun stop(): Int {
    return kotlin.runCatching {
      SafeFuture.allOf(
        l1App.stop(),
        SafeFuture.fromRunnable { l2Web3jClient.shutdown() },
        api.stop().toSafeFuture()
      ).thenApply {
        LoadBalancingJsonRpcClient.stop()
      }.thenCompose {
        requestFileCleanup.cleanup()
      }.thenCompose {
        vertx.close().toSafeFuture().thenApply { log.info("vertx Stopped") }
      }.thenApply {
        log.info("CoordinatorApp Stopped")
      }.get()
      0
    }.recover { e ->
      log.error("CoordinatorApp Stopped with error: errorMessage={}", e.message, e)
      1
    }.getOrThrow()
  }

  private fun initDb(dbConfig: DatabaseConfig): SqlClient {
    val dbVersion = "4"
    Db.applyDbMigrations(
      host = dbConfig.host,
      port = dbConfig.port,
      database = dbConfig.schema,
      target = dbVersion,
      username = dbConfig.username,
      password = dbConfig.password.value
    )
    return Db.vertxSqlClient(
      vertx = vertx,
      host = dbConfig.host,
      port = dbConfig.port,
      database = dbConfig.schema,
      username = dbConfig.username,
      password = dbConfig.password.value,
      maxPoolSize = dbConfig.transactionalPoolSize,
      pipeliningLimit = dbConfig.readPipeliningLimit
    )
  }
}
