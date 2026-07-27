package linea.coordinator.app

import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import io.vertx.micrometer.backends.NoopBackendRegistry
import io.vertx.sqlclient.SqlClient
import linea.coordinator.api.Api
import linea.coordinator.app.conflationbacktesting.ConflationBacktestingService
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.DatabaseConfig
import linea.coordinator.config.v2.logPretty
import linea.coordinator.extensions.CoordinatorContext
import linea.coordinator.extensions.CoordinatorExtensionFactory
import linea.fileio.DirectoryCleaner
import linea.persistence.DisabledForcedTransactionsDao
import linea.persistence.conflation.AggregationsRepositoryImpl
import linea.persistence.conflation.BatchesPostgresDao
import linea.persistence.conflation.BlobsPostgresDao
import linea.persistence.conflation.BlobsRepositoryImpl
import linea.persistence.conflation.PostgresAggregationsDao
import linea.persistence.conflation.PostgresBatchesRepository
import linea.persistence.conflation.RetryingBatchesPostgresDao
import linea.persistence.conflation.RetryingBlobsPostgresDao
import linea.persistence.conflation.RetryingPostgresAggregationsDao
import linea.persistence.db.Db
import linea.persistence.db.PersistenceRetryer
import linea.persistence.ftx.PostgresForcedTransactionsDao
import linea.persistence.ftx.RetryingPostgresForcedTransactionsDao
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.client.LoadBalancingJsonRpcClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.vertx.loadVertxConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock

class CoordinatorApp(
  private val configs: CoordinatorConfig,
  private val clock: Clock = Clock.System,
  // Single seam for downstream distributions: contributes extra services and JSON-RPC
  // handlers that share this app's Vertx, metrics and DB. Defaults to no-op so the OSS app
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

  private val l1App =
    L1DependentApp(
      configs = configs,
      vertx = vertx,
      httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      forcedTransactionsDao = forcedTransactionsDao,
      aggregationsRepository = aggregationsRepository,
      sqlClient = sqlClient,
      smartContractErrors = configs.smartContractErrors,
      metricsFacade = micrometerMetricsFacade,
      clock = this.clock,
      signerFactory = signerFactory,
    )

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
      conflationCheckpointResumeLatch = l1App::signalTargetCheckpointResumeFromApi,
      additionalRequestHandlers = extensionRpcHandlers,
    )

  private val requestFileCleanup =
    DirectoryCleaner(
      vertx = vertx,
      directories =
      listOfNotNull(
        configs.proversConfig.proverA.execution.requestsDirectory,
        configs.proversConfig.proverA.blobCompression.requestsDirectory,
        configs.proversConfig.proverA.proofAggregation.requestsDirectory,
        configs.proversConfig.proverB?.execution?.requestsDirectory,
        configs.proversConfig.proverB?.blobCompression?.requestsDirectory,
        configs.proversConfig.proverB?.proofAggregation?.requestsDirectory,
      ),
      fileFilters =
      DirectoryCleaner.getSuffixFileFilters(
        listOfNotNull(
          configs.proversConfig.proverA.execution.inprogressRequestWritingSuffix,
          configs.proversConfig.proverA.blobCompression.inprogressRequestWritingSuffix,
          configs.proversConfig.proverA.proofAggregation.inprogressRequestWritingSuffix,
          configs.proversConfig.proverB?.execution?.inprogressRequestWritingSuffix,
          configs.proversConfig.proverB?.blobCompression?.inprogressRequestWritingSuffix,
          configs.proversConfig.proverB?.proofAggregation?.inprogressRequestWritingSuffix,
        ),
      ) +
        if (configs.proversConfig.enableRequestFilesCleanup) {
          // Will delete prover request .json files from all the directories
          listOf(DirectoryCleaner.JSON_FILE_FILTER)
        } else {
          emptyList()
        },
    )

  init {
    log.info("Coordinator app instantiated")
  }

  fun start() {
    requestFileCleanup.cleanup()
      .thenCompose { l1App.start() }
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
      SafeFuture.allOf(
        SafeFuture.allOf(*extensionServices.map { it.stop().toSafeFuture() }.toTypedArray()),
        l1App.stop(),
        api.stop(),
        conflationBacktestingService.stop(),
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
