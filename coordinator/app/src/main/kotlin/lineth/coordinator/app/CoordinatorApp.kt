package lineth.coordinator.app

import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import io.vertx.micrometer.backends.NoopBackendRegistry
import io.vertx.sqlclient.SqlClient
import linea.DisabledService
import linea.LongRunningService
import linea.clients.StateManagerV1JsonRpcClient
import linea.contract.l1.FinalizedStateDataClientReadOnly
import linea.contract.l1.Web3JLineaValidiumSmartContractClientReadOnly
import linea.contract.l1.Web3JLinethRollupSmartContractClientReadOnly
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.ethapi.EthLogsSearcherImpl
import linea.ftx.ForcedTransactionsApp
import linea.persistence.db.Db
import linea.persistence.db.PersistenceRetryer
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import lineth.coordinator.api.Api
import lineth.coordinator.app.conflation.ConflationAppHelper
import lineth.coordinator.app.conflation.ConflationAppV1
import lineth.coordinator.app.conflation.ConflationAppV2
import lineth.coordinator.app.conflation.TracesClientFactory.createTracesClients
import lineth.coordinator.app.conflation.TracesClients
import lineth.coordinator.app.conflationbacktesting.ConflationBacktestingService
import lineth.coordinator.clients.ForcedTransactionsJsonRpcClient
import lineth.coordinator.clients.prover.ProverClientFactory
import lineth.coordinator.config.toJsonRpcRetry
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.coordinator.config.v2.DatabaseConfig
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.coordinator.config.v2.isEnabled
import lineth.coordinator.config.v2.logPretty
import lineth.coordinator.extensions.CoordinatorContext
import lineth.coordinator.extensions.CoordinatorExtensionFactory
import lineth.ftx.conflation.ForcedTransactionsInvalidityProofService
import lineth.ftx.conflation.InvalidityProofAssembler
import lineth.persistence.DisabledForcedTransactionsDao
import lineth.persistence.FeeHistoriesPostgresDao
import lineth.persistence.conflation.AggregationsRepositoryImpl
import lineth.persistence.conflation.BatchesPostgresDao
import lineth.persistence.conflation.BlobsPostgresDao
import lineth.persistence.conflation.BlobsRepositoryImpl
import lineth.persistence.conflation.PostgresAggregationsDao
import lineth.persistence.conflation.PostgresBatchesRepository
import lineth.persistence.conflation.RetryingBatchesPostgresDao
import lineth.persistence.conflation.RetryingBlobsPostgresDao
import lineth.persistence.conflation.RetryingPostgresAggregationsDao
import lineth.persistence.ftx.PostgresForcedTransactionsDao
import lineth.persistence.ftx.RetryingPostgresForcedTransactionsDao
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
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

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
      val dgc = configs.l1Submission.dynamicGasPriceCap
      log.trace("dynamicGasPriceCap.timeOfDayMultipliers: {}", dgc.timeOfDayMultipliers)
      log.trace(
        "dynamicGasPriceCap.gasPriceCapCalculation.timeOfTheDayMultipliers: {}",
        dgc.gasPriceCapCalculation.timeOfTheDayMultipliers,
      )

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
            maxBlobsToReturn = configs.l1Submission.blob.dbMaxBlobsToReturn,
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

  // The data-availability mode decides which L1 contract client the read path uses.
  private val dataAvailability: L1SubmissionConfig.DataAvailability = configs.l1Submission.dataAvailability

  // Must agree with the forcedTransactionsApp wiring below; see ConflationAppHelper.forcedTransactionsEnabled.
  private val forcedTransactionsDao = run {
    if (!ConflationAppHelper.forcedTransactionsEnabled(configs.forcedTransactions, dataAvailability)) {
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

  // The finalization monitor reads the L1 contract, so its client must match the deployed contract
  // flavour: a rollup client pointed at a validium contract fails version detection and the
  // coordinator cannot start.
  private val finalizationMonitorClient: FinalizedStateDataClientReadOnly = run {
    val web3j = createWeb3jHttpClient(
      rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.finalization-monitor"),
    )
    when (dataAvailability) {
      L1SubmissionConfig.DataAvailability.ROLLUP ->
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

      L1SubmissionConfig.DataAvailability.VALIDIUM ->
        // No logs searcher / l1FinalizationMonitor.l1RequestRetries here: the validium client has no
        // event search yet. When getFinalizedStateData's V2 branch adds the FinalizedStateUpdated
        // search, the searcher + those retries must be wired in here too.
        Web3JLineaValidiumSmartContractClientReadOnly(
          contractAddress = configs.protocol.l1.contractAddress,
          web3j = web3j,
        )
    }
  }

  private val lastFinalizedBlock: ULong = L1BasedLastFinalizedBlockProvider(
    vertx,
    lineaSmartContractClient = finalizationMonitorClient,
    consistentNumberOfBlocksOnL1 = configs.conflation.consistentNumberOfBlocksOnL1ToWait,
  ).getLastFinalizedBlock().get()

  private val proverClientFactory: ProverClientFactory = ProverClientFactory(
    vertx = vertx,
    config = configs.proversConfig,
    metricsFacade = micrometerMetricsFacade,
  )

  private val l2EthClientForConflation = createEthApiClient(
    rpcUrl = configs.conflation.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.conflation"),
    requestRetryConfig = configs.conflation.l2RequestRetries,
    vertx = vertx,
  )

  private val zkStateClient: StateManagerV1JsonRpcClient = StateManagerV1JsonRpcClient.create(
    rpcClientFactory = httpJsonRpcClientFactory,
    endpoints = configs.stateManager.endpoints.map { it.toURI() },
    maxInflightRequestsPerClient = configs.stateManager.requestLimitPerEndpoint,
    requestRetry = configs.stateManager.requestRetries.toJsonRpcRetry(),
    requestTimeout = configs.stateManager.requestTimeout?.inWholeMilliseconds,
    logger = LogManager.getLogger("clients.StateManagerShomeiClient"),
  )

  private val tracesClients: TracesClients = createTracesClients(
    vertx = vertx,
    rpcClientFactory = httpJsonRpcClientFactory,
    configs = configs.traces,
    fallBackTracesCounters = configs.conflation.tracesLimits.emptyTracesCounters,
  )

  private val forcedTransactionsApp: ForcedTransactionsApp = run {
    // Forced transactions are rollup-only for now; see ConflationAppHelper.forcedTransactionsEnabled.
    val ftxConfig = configs.forcedTransactions
    if (ftxConfig == null || !ConflationAppHelper.forcedTransactionsEnabled(ftxConfig, dataAvailability)) {
      // If we get here with forced transactions enabled in config, the chain must be running in validium mode.
      if (ftxConfig?.disabled == false) {
        log.warn(
          "Forced transactions are enabled in config but the coordinator does not support them on validium " +
            "chains yet; disabling. Storing a forced transaction on a validium V2 contract halts finalization.",
        )
      }
      ForcedTransactionsApp.createDisabled()
    } else {
      val l1EthClient = createEthApiClient(
        rpcUrl = ftxConfig.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.ftx"),
        vertx = vertx,
        requestRetryConfig = ftxConfig.l1RequestRetries,
      )
      val ftxAppConfig = ForcedTransactionsApp.Config(
        l1PollingInterval = ftxConfig.l1EventScraping.pollingInterval,
        l1ContractAddress = configs.protocol.l1.contractAddress,
        l1HighestBlockTag = ftxConfig.l1HighestBlockTag,
        l1EventSearchBlockChunk = ftxConfig.l1EventScraping.ethLogsSearchBlockChunkSize,
        l1EventSearchMaxBlockRange = ftxConfig.l1EventScraping.ethLogsSearchMaxBlockRange,
        ftxSequencerSendingInterval = ftxConfig.processingTickInterval,
        maxFtxToSendToSequencer = ftxConfig.processingBatchSize,
        ftxProcessingDelay = ftxConfig.processingDelay,
        invalidityProofProcessingInterval = ftxConfig.invalidityProofCheckInterval,
      )
      val ftxClient = ForcedTransactionsJsonRpcClient(
        vertx = vertx,
        rpcClient = httpJsonRpcClientFactory.create(
          endpoint = ftxConfig.sequencerEndpoint,
          log = LogManager.getLogger("clients.l2.ftx.sequencer"),
        ),
        retryConfig = ftxConfig.sequencerRequestRetries.toJsonRpcRetry(),
        log = LogManager.getLogger("clients.l2.ftx.sequencer"),
      )
      val l1Web3jClient = createWeb3jHttpClient(
        rpcUrl = ftxConfig.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.ftx"),
      )
      val contractClient = Web3JLinethRollupSmartContractClientReadOnly(
        contractAddress = configs.protocol.l1.contractAddress,
        web3j = l1Web3jClient,
        ethLogsSearcher = EthLogsSearcherImpl(
          vertx = vertx,
          ethApiClient = createEthApiClient(
            web3jClient = l1Web3jClient,
            requestRetryConfig = ftxConfig.l1RequestRetries,
            vertx = vertx,
          ),
        ),
        finalizedStateSearchInitialBlockParameter = configs.protocol.l1.contractDeploymentBlockNumber
          ?: BlockParameter.Tag.EARLIEST,
      )
      val riscvCutoverTimestamp = configs.conflation.riscvStartingBlockTimestampInclusive
      val lastFinalizedBlockTimestamp: Instant = if (lastFinalizedBlock == 0UL) {
        Instant.fromEpochSeconds(0)
      } else {
        l2EthClientForConflation
          .ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(lastFinalizedBlock.toLong()))
          .thenApply { block -> Instant.fromEpochSeconds(block.timestamp.toLong()) }
          .get()
      }
      val ftxInvalidityProofService: LongRunningService = if (
        riscvCutoverTimestamp != null && lastFinalizedBlockTimestamp >= riscvCutoverTimestamp
      ) {
        log.info(
          "FTX invalidity proof service disabled: already past RISC-V cutover. " +
            "lastFinalizedBlockTimestamp={}, cutover={}",
          lastFinalizedBlockTimestamp,
          riscvCutoverTimestamp,
        )
        DisabledService("forced-transactions-invalidity-proof")
      } else {
        check(configs.proversConfig.proverA.invalidity != null) {
          "prover.invalidity config is required for forced transactions feature to work"
        }
        val l1EthLogsSearcherForFtx = EthLogsSearcherImpl(vertx = vertx, ethApiClient = l1EthClient)
        ForcedTransactionsInvalidityProofService(
          ftxDao = forcedTransactionsDao,
          invalidityProofAssembler = InvalidityProofAssembler(
            invalidityProofClient = proverClientFactory.createInvalidityProofClient(),
            stateManagerClient = zkStateClient,
            accountProofClient = zkStateClient,
            ethApiLogsSearcher = l1EthLogsSearcherForFtx,
            ftxDao = forcedTransactionsDao,
            tracesClient = tracesClients.tracesConflationClient,
            contractAddress = configs.protocol.l1.contractAddress,
            l1EventSearchMaxBlockRange = ftxConfig.l1EventScraping.ethLogsSearchMaxBlockRange,
          ),
          vertx = vertx,
          pollingInterval = ftxConfig.invalidityProofCheckInterval,
          riscvCutoverTimestamp = riscvCutoverTimestamp,
        )
      }
      ForcedTransactionsApp.create(
        config = ftxAppConfig,
        vertx = vertx,
        ftxDao = forcedTransactionsDao,
        l1EthApiClient = l1EthClient,
        l2EthApiClient = l2EthClientForConflation,
        ftxClient = ftxClient,
        finalizedStateProvider = contractClient,
        contractVersionProvider = contractClient,
        clock = clock,
        metricsFacade = micrometerMetricsFacade,
        ftxInvalidityProofService = ftxInvalidityProofService,
      )
    }
  }

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
    proverClientFactory = proverClientFactory,
    l2EthClient = l2EthClientForConflation,
    zkStateClient = zkStateClient,
    tracesClients = tracesClients,
    forcedTransactionsApp = forcedTransactionsApp,
  )

  private val conflationAppV2: ConflationAppV2? =
    if (configs.conflation.riscvStartingBlockTimestampInclusive != null) {
      ConflationAppV2(
        vertx = vertx,
        lastFinalizedBlock = lastFinalizedBlock,
        batchesRepository = batchesRepository,
        configs = configs,
        forcedTransactionsApp = forcedTransactionsApp,
      )
    } else {
      null
    }

  private val l1FinalizationMonitorApp = L1FinalizationMonitorApp(
    configs = configs,
    vertx = vertx,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    finalizedStateDataProvider = finalizationMonitorClient,
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
        l1SubmissionConfig = configs.l1Submission,
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
      .thenCompose { forcedTransactionsApp.start() }
      .thenCompose { conflationAppV2?.start() ?: SafeFuture.completedFuture(Unit) }
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
        .thenCompose { conflationAppV2?.stop() ?: SafeFuture.completedFuture(Unit) }
        .thenCompose {
          SafeFuture.allOf(
            SafeFuture.allOf(*extensionServices.map { it.stop().toSafeFuture() }.toTypedArray()),
            l2PricingApp.stop(),
            messageAnchoringApp.stop(),
            l1RelayingAppV1.stop(),
            api.stop(),
            conflationBacktestingService.stop(),
            forcedTransactionsApp.stop(),
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
