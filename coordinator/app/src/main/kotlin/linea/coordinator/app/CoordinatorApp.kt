package linea.coordinator.app

import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import io.vertx.micrometer.backends.NoopBackendRegistry
import io.vertx.sqlclient.SqlClient
import linea.coordinator.api.Api
import linea.coordinator.app.conflation.ConflationAppV1
import linea.coordinator.app.conflationbacktesting.ConflationBacktestingService
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.DatabaseConfig
import linea.coordinator.config.v2.isEnabled
import linea.coordinator.config.v2.logPretty
import linea.coordinator.extensions.CoordinatorContext
import linea.coordinator.extensions.CoordinatorExtensionFactory
import linea.persistence.DisabledForcedTransactionsDao
import linea.persistence.FeeHistoriesPostgresDao
import linea.persistence.conflation.AggregationsRepositoryImpl
import linea.persistence.conflation.BatchesPostgresDao
import linea.persistence.conflation.BlobsPostgresDao
import linea.persistence.conflation.BlobsRepositoryImpl
import linea.persistence.conflation.PostgresAggregationsDao
import linea.persistence.conflation.PostgresBatchesRepository
import linea.persistence.conflation.RetryingBatchesPostgresDao
import linea.persistence.conflation.RetryingBlobsPostgresDao
import linea.persistence.conflation.RetryingPostgresAggregationsDao
import linea.persistence.ftx.PostgresForcedTransactionsDao
import linea.persistence.ftx.RetryingPostgresForcedTransactionsDao
import lineth.DisabledService
import lineth.LongRunningService
import lineth.async.toSafeFuture
import lineth.contract.l1.Web3JLinethRollupSmartContractClientReadOnly
import lineth.domain.BlockParameter
import lineth.domain.RetryConfig
import lineth.ethapi.EthLogsSearcherImpl
import lineth.jsonrpc.client.LoadBalancingJsonRpcClient
import lineth.jsonrpc.client.VertxHttpJsonRpcClientFactory
import lineth.metrics.micrometer.MicrometerMetricsFacade
import lineth.persistence.db.Db
import lineth.persistence.db.PersistenceRetryer
import lineth.vertx.loadVertxConfig
import lineth.web3j.createWeb3jHttpClient
import lineth.web3j.ethapi.createEthApiClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds

class CoordinatorApp(
  private val configs: CoordinatorConfig,
  private val clock: Clock = Clock.System,
  // Single seam for downstream distributions: contributes extra services and JSON-RPC
  // handlers that share this app's Vertx, metrics and DB. Defaults to no-op so the app
  // behaves identically when no extension is supplied.
  extensionsFactory: CoordinatorExtensionFactory = CoordinatorExtensionFactory.NOOP,
  signerFactory: SignerFactory = DefaultSignerFactory,
) {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val vertx: Vertx =
    run {
      log.trace("System properties: {}", System.getProperties())
      val vertxConfig = loadVertxConfig()
      log.debug("Vertx full configs: {}", vertxConfig)
      // Raw single-line dump kept for existing tooling that parses this line.
      log.info("App configs: {}", configs)
      // Human-readable form: one fully-qualified `path: value` per INFO event so each config is a
      // separate line in log aggregators (Grafana/Loki) instead of a collapsed multi-line blob.
      configs.logPretty(log)
      log.trace(
        "Full smartContractErrors ({} entries): {}",
        configs.smartContractErrors.size,
        configs.smartContractErrors,
      )
      configs.l1Submission?.dynamicGasPriceCap?.let { dgc ->
        log.trace("dynamicGasPriceCap.timeOfDayMultipliers: {}", dgc.timeOfDayMultipliers)
        log.trace(
          "dynamicGasPriceCap.gasPriceCapCalculation.timeOfTheDayMultipliers: {}",
          dgc.gasPriceCapCalculation.timeOfTheDayMultipliers,
        )
      }

      Vertx.vertx(vertxConfig)
    }
  private val meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow()
  private val micrometerMetricsFacade = MicrometerMetricsFacade(meterRegistry, "linea")
  private val httpJsonRpcClientFactory =
    VertxHttpJsonRpcClientFactory(
      vertx = vertx,
      metricsFacade = micrometerMetricsFacade,
      requestResponseLogLevel = Level.TRACE,
      failuresLogLevel = Level.WARN,
    )

  private val conflationBacktestingService = ConflationBacktestingService(
    vertx = vertx,
    configs = configs,
    metricsFacade = MicrometerMetricsFacade(NoopBackendRegistry.INSTANCE.meterRegistry, "conflationbacktesting"),
  )

  private val persistenceRetryer =
    PersistenceRetryer(
      vertx = vertx,
      config =
      PersistenceRetryer.Config(
        backoffDelay = configs.database.persistenceRetries.backoffDelay,
        maxRetries = configs.database.persistenceRetries.maxRetries?.toInt(),
        timeout = configs.database.persistenceRetries.timeout,
        ignoreFirstExceptionsUntilTimeElapsed =
        configs.database.persistenceRetries.ignoreFirstExceptionsUntilTimeElapsed,
      ),
    )

  private val sqlClient: SqlClient = initDb(configs.database)
  private val batchesRepository =
    PostgresBatchesRepository(
      batchesDao =
      RetryingBatchesPostgresDao(
        delegate =
        BatchesPostgresDao(
          connection = sqlClient,
        ),
        persistenceRetryer = persistenceRetryer,
      ),
    )
  private val blobsRepository =
    BlobsRepositoryImpl(
      blobsDao =
      RetryingBlobsPostgresDao(
        delegate =
        BlobsPostgresDao(
          config =
          BlobsPostgresDao.Config(
            maxBlobsToReturn = configs.l1Submission?.blob?.dbMaxBlobsToReturn ?: 50u,
          ),
          connection = sqlClient,
        ),
        persistenceRetryer = persistenceRetryer,
      ),
    )
  private val aggregationsRepository =
    AggregationsRepositoryImpl(
      aggregationsPostgresDao =
      RetryingPostgresAggregationsDao(
        delegate =
        PostgresAggregationsDao(
          connection = sqlClient,
        ),
        persistenceRetryer = persistenceRetryer,
      ),
    )
  private val forcedTransactionsDao = run {
    if (configs.forcedTransactions?.disabled ?: true) {
      DisabledForcedTransactionsDao()
    } else {
      RetryingPostgresForcedTransactionsDao(
        delegate =
        PostgresForcedTransactionsDao(
          connection = sqlClient,
        ),
        persistenceRetryer = persistenceRetryer,
      )
    }
  }
  private val feeHistoriesDao = FeeHistoriesPostgresDao(sqlClient)

  private val l1ChainId: ULong = createEthApiClient(
    rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
    log = LogManager.getLogger("clients.l1.eth"),
    vertx = vertx,
    requestRetryConfig = RetryConfig.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  ).ethChainId().get()

  private val linethRollupClientForFinalizationMonitor = run {
    val web3j = createWeb3jHttpClient(
      rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.finalization-monitor"),
    )
    Web3JLinethRollupSmartContractClientReadOnly(
      contractAddress = configs.protocol.l1.contractAddress,
      web3j = web3j,
      ethLogsSearcher = EthLogsSearcherImpl(
        vertx = vertx,
        ethApiClient = createEthApiClient(
          web3jClient = web3j,
          requestRetryConfig = configs.l1FinalizationMonitor.l1RequestRetries,
          vertx = vertx,
        ),
      ),
      finalizedStateSearchInitialBlockParameter = configs.protocol.l1.contractDeploymentBlockNumber
        ?: BlockParameter.Tag.EARLIEST,
    )
  }

  private val lastFinalizedBlock: ULong = L1BasedLastFinalizedBlockProvider(
    vertx,
    linethRollupSmartContractClient = linethRollupClientForFinalizationMonitor,
    consistentNumberOfBlocksOnL1 = configs.conflation.consistentNumberOfBlocksOnL1ToWait,
  ).getLastFinalizedBlock().get()

  private val conflationApp: ConflationAppV1 = ConflationAppV1(
    vertx = vertx,
    clock = clock,
    batchesRepository = batchesRepository,
    blobsRepository = blobsRepository,
    aggregationsRepository = aggregationsRepository,
    forcedTransactionsDao = forcedTransactionsDao,
    lastFinalizedBlock = lastFinalizedBlock,
    configs = configs,
    metricsFacade = micrometerMetricsFacade,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
  )

  private val l1FinalizationMonitorApp = L1FinalizationMonitorApp(
    configs = configs,
    vertx = vertx,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    finalizedStateDataProvider = linethRollupClientForFinalizationMonitor,
    lastFinalizedBlock = lastFinalizedBlock,
    batchesRepository = batchesRepository,
    blobsRepository = blobsRepository,
    aggregationsRepository = aggregationsRepository,
    forcedTransactionsDao = forcedTransactionsDao,
    metricsFacade = micrometerMetricsFacade,
    l1FinalizationUpdateHandler = conflationApp::updateLatestL1FinalizedBlock,
  )

  private val messageAnchoringApp: LongRunningService = MessageAnchoringAppConfigurator.create(
    vertx = vertx,
    configs = configs,
    signerFactory = signerFactory,
  )

  private val l1RelayingAppV1 = run {
    if (configs.l1Submission.isEnabled()) {
      L1RelayingAppV1(
        configs = configs,
        l1SubmissionConfig = configs.l1Submission!!,
        vertx = vertx,
        l1ChainId = l1ChainId,
        lastFinalizedBlock = lastFinalizedBlock,
        smartContractErrors = configs.smartContractErrors,
        metricsFacade = micrometerMetricsFacade,
        feeHistoriesDao = feeHistoriesDao,
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        signerFactory = signerFactory,
      )
    } else {
      log.warn("L1 submission disabled for blobs and aggregations")
      DisabledService("L1RelayingApp")
    }
  }

  private val l2PricingApp: LongRunningService =
    if (configs.l2NetworkGasPricing.isEnabled()) {
      L2PricingApp(
        l2NetworkGasPricingConfig = configs.l2NetworkGasPricing!!,
        vertx = vertx,
        metricsFacade = micrometerMetricsFacade,
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      )
    } else {
      log.warn("L2 Network dynamic gas pricing is disabled")
      DisabledService("L2PricingApp")
    }

  // Resolve extensions once, against the infrastructure already built above. Done before any
  // service is started so their services join the lifecycle and their handlers join the router.
  private val extensions =
    extensionsFactory.create(
      object : CoordinatorContext {
        override val vertx: Vertx = this@CoordinatorApp.vertx
        override val metricsFacade = micrometerMetricsFacade
        override val sqlClient: SqlClient = this@CoordinatorApp.sqlClient
      },
    )
  private val extensionServices = extensions.flatMap { it.services() }
  private val extensionRpcHandlers = extensions.flatMap { it.jsonRpcHandlers().entries }
    .associate { it.key to it.value }
    .also { handlers ->
      if (handlers.isNotEmpty()) {
        log.info("Registered {} extension JSON-RPC handler(s): {}", handlers.size, handlers.keys)
      }
    }

  private val api =
    Api(
      configs = Api.Config(
        observabilityPort = configs.api.observabilityPort,
        jsonRpcPort = configs.api.jsonRpcPort,
        jsonRpcPath = configs.api.jsonRpcPath,
        jsonRpcServerVerticles = configs.api.jsonRpcServerVerticles,
      ),
      vertx = vertx,
      conflationBacktestingService = conflationBacktestingService,
      metricsFacade = micrometerMetricsFacade,
      conflationCheckpointResumeLatch = conflationApp::signalTargetCheckpointResumeFromApi,
      additionalRequestHandlers = extensionRpcHandlers,
    )

  init {
    log.info("Coordinator app instantiated")
  }

  fun start() {
    SafeFuture.completedFuture(Unit)
      .thenCompose { l1FinalizationMonitorApp.start() }
      .thenCompose { conflationApp.start() }
      .thenCompose { l1RelayingAppV1.start() }
      .thenCompose { messageAnchoringApp.start() }
      .thenCompose { l2PricingApp.start() }
      .thenCompose { conflationBacktestingService.start() }
      .thenCompose {
        SafeFuture.allOf(*extensionServices.map { it.start().toSafeFuture() }.toTypedArray())
      }
      .thenCompose { api.start() }
      .get()

    log.info("Started :)")
  }

  fun stop(): Int {
    return try {
      l1FinalizationMonitorApp.stop()
        .thenCompose { conflationApp.stop() }
        .thenCompose {
          SafeFuture.allOf(
            SafeFuture.allOf(*extensionServices.map { it.stop().toSafeFuture() }.toTypedArray()),
            l2PricingApp.stop(),
            messageAnchoringApp.stop(),
            l1RelayingAppV1.stop(),
            api.stop(),
            conflationBacktestingService.stop(),
          )
        }.thenApply {
          LoadBalancingJsonRpcClient.stop()
        }.thenCompose {
          vertx.close().toSafeFuture().thenApply { log.info("vertx Stopped") }
        }.thenApply {
          log.info("CoordinatorApp Stopped")
        }.get()
      0
    } catch (e: Exception) {
      log.error("CoordinatorApp Stopped with error: errorMessage={}", e.message, e)
      1
    }
  }

  private fun initDb(dbConfig: DatabaseConfig): SqlClient {
    Db.applyDbMigrations(
      host = dbConfig.host,
      port = dbConfig.port,
      database = dbConfig.schema,
      target = dbConfig.schemaVersion.toString(),
      username = dbConfig.username,
      password = dbConfig.password.value,
    )
    return Db.vertxSqlClient(
      vertx = vertx,
      host = dbConfig.host,
      port = dbConfig.port,
      database = dbConfig.schema,
      username = dbConfig.username,
      password = dbConfig.password.value,
      maxPoolSize = dbConfig.transactionalPoolSize,
      pipeliningLimit = dbConfig.readPipeliningLimit,
    )
  }
}
