package net.consensys.zkevm.coordinator.app

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import io.vertx.core.Vertx
import io.vertx.sqlclient.SqlClient
import kotlinx.datetime.Clock
import linea.anchoring.MessageAnchoringApp
import linea.blob.ShnarfCalculatorVersion
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import linea.contract.l2.Web3JL2MessageServiceSmartContractClient
import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.isDisabled
import linea.coordinator.config.v2.isEnabled
import linea.domain.BlockNumberAndHash
import linea.domain.RetryConfig
import linea.encoding.BlockRLPEncoder
import linea.kotlin.toKWeiUInt
import linea.web3j.ExtendedWeb3JImpl
import linea.web3j.SmartContractErrors
import linea.web3j.Web3jBlobExtended
import linea.web3j.createWeb3jHttpClient
import linea.web3j.createWeb3jHttpService
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.contract.l1.GenesisStateProvider
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.WMAFeesCalculator
import net.consensys.linea.ethereum.gaspricing.WMAGasProvider
import net.consensys.linea.ethereum.gaspricing.dynamiccap.FeeHistoryCachingService
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapCalculatorImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapFeeHistoryFetcher
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapFeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderForDataSubmission
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderForFinalization
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.zkevm.LongRunningService
import net.consensys.zkevm.coordinator.blockcreation.BatchesRepoBasedLastProvenBlockNumberProvider
import net.consensys.zkevm.coordinator.blockcreation.BlockCreationMonitor
import net.consensys.zkevm.coordinator.blockcreation.GethCliqueSafeBlockProvider
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.ShomeiClient
import net.consensys.zkevm.coordinator.clients.TracesGeneratorJsonRpcClientV2
import net.consensys.zkevm.coordinator.clients.prover.ProverClientFactory
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.BlobSubmittedEvent
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.FinalizationSubmittedEvent
import net.consensys.zkevm.ethereum.coordination.EventDispatcher
import net.consensys.zkevm.ethereum.coordination.HighestConflationTracker
import net.consensys.zkevm.ethereum.coordination.HighestProvenBatchTracker
import net.consensys.zkevm.ethereum.coordination.HighestProvenBlobTracker
import net.consensys.zkevm.ethereum.coordination.HighestULongTracker
import net.consensys.zkevm.ethereum.coordination.HighestUnprovenBlobTracker
import net.consensys.zkevm.ethereum.coordination.LatestBlobSubmittedBlockNumberTracker
import net.consensys.zkevm.ethereum.coordination.LatestFinalizationSubmittedBlockNumberTracker
import net.consensys.zkevm.ethereum.coordination.SimpleCompositeSafeFutureHandler
import net.consensys.zkevm.ethereum.coordination.aggregation.ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTracker
import net.consensys.zkevm.ethereum.coordination.aggregation.ProofAggregationCoordinatorService
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressionProofCoordinator
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressionProofUpdate
import net.consensys.zkevm.ethereum.coordination.blob.BlobZkStateProviderImpl
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobCompressor
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blob.RollingBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blockcreation.ForkChoiceUpdaterImpl
import net.consensys.zkevm.ethereum.coordination.conflation.BlockToBatchSubmissionCoordinator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByBlockLimit
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByDataCompressed
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByExecutionTraces
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByTargetBlockNumbers
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByTimeDeadline
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationService
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationServiceImpl
import net.consensys.zkevm.ethereum.coordination.conflation.DeadlineConflationCalculatorRunner
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlobAwareConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlockConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ProofGeneratingConflationHandlerImpl
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCoordinatorImpl
import net.consensys.zkevm.ethereum.coordination.proofcreation.ZkProofCreationCoordinatorImpl
import net.consensys.zkevm.ethereum.finalization.AggregationFinalizationCoordinator
import net.consensys.zkevm.ethereum.finalization.AggregationSubmitterImpl
import net.consensys.zkevm.ethereum.finalization.FinalizationHandler
import net.consensys.zkevm.ethereum.finalization.FinalizationMonitor
import net.consensys.zkevm.ethereum.finalization.FinalizationMonitorImpl
import net.consensys.zkevm.ethereum.submission.BlobSubmissionCoordinator
import net.consensys.zkevm.ethereum.submission.L1ShnarfBasedAlreadySubmittedBlobsFilter
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import net.consensys.zkevm.persistence.dao.aggregation.RecordsCleanupFinalizationHandler
import net.consensys.zkevm.persistence.dao.batch.persistence.BatchProofHandlerImpl
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesPostgresDao
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesRepositoryImpl
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import java.util.function.Consumer
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class L1DependentApp(
  private val configs: CoordinatorConfig,
  private val vertx: Vertx,
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  private val batchesRepository: BatchesRepository,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
  private val sqlClient: SqlClient,
  private val smartContractErrors: SmartContractErrors,
  private val metricsFacade: MetricsFacade,
) : LongRunningService {
  private val log = LogManager.getLogger(this::class.java)

  init {
    if (configs.l1Submission.isDisabled()) {
      log.warn("L1 submission disabled for blobs and aggregations")
    } else {
      if (configs.l1Submission!!.blob.isDisabled()) {
        log.warn("L1 submission disabled for blobs")
      }
      if (configs.l1Submission.aggregation.isDisabled()) {
        log.warn("L1 submission disabled for aggregations")
      }
    }

    if (configs.messageAnchoring.isDisabled()) {
      log.warn("Message anchoring is disabled")
    }
    if (configs.l2NetworkGasPricing.isDisabled()) {
      log.warn("L2 Network dynamic gas pricing is disabled")
    }
  }

  private val l1ChainId = run {
    createEthApiClient(
      rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth"),
      vertx = vertx,
      requestRetryConfig = RetryConfig.endlessRetry(
        backoffDelay = 1.seconds,
        failuresWarningThreshold = 3u,
      ),
    ).getChainId().get()
  }

  private val proverClientFactory = ProverClientFactory(
    vertx = vertx,
    config = configs.proversConfig,
    metricsFacade = metricsFacade,
  )

  private val finalizationTransactionManager = createTransactionManager(
    vertx = vertx,
    signerConfig = configs.l1Submission!!.aggregation.signer,
    client = createWeb3jHttpClient(
      rpcUrl = configs.l1Submission.aggregation.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.finalization"),
    ),
  )

  private val l1MinPriorityFeeCalculator: FeesCalculator = WMAFeesCalculator(
    WMAFeesCalculator.Config(
      baseFeeCoefficient = 0.0,
      priorityFeeWmaCoefficient = 1.0,
    ),
  )

  private val l1DataSubmissionPriorityFeeCalculator: FeesCalculator = BoundableFeeCalculator(
    BoundableFeeCalculator.Config(
      feeUpperBound = configs.l1Submission!!.blob.gas.fallback.priorityFeePerGasUpperBound.toDouble(),
      feeLowerBound = configs.l1Submission.blob.gas.fallback.priorityFeePerGasLowerBound.toDouble(),
      feeMargin = 0.0,
    ),
    l1MinPriorityFeeCalculator,
  )

  private val l1FinalizationPriorityFeeCalculator: FeesCalculator = BoundableFeeCalculator(
    BoundableFeeCalculator.Config(
      feeUpperBound = configs.l1Submission!!.aggregation.gas.fallback.priorityFeePerGasUpperBound.toDouble(),
      feeLowerBound = configs.l1Submission.aggregation.gas.fallback.priorityFeePerGasUpperBound.toDouble(),
      feeMargin = 0.0,
    ),
    l1MinPriorityFeeCalculator,
  )

  private val feesFetcher: FeesFetcher = run {
    val httpService = createWeb3jHttpService(
      configs.l1Submission!!.dynamicGasPriceCap.feeHistoryFetcher.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.fees-fetcher"),
    )
    val l1Web3jClient = createWeb3jHttpClient(httpService)
    FeeHistoryFetcherImpl(
      web3jClient = l1Web3jClient,
      web3jService = Web3jBlobExtended(httpService),
      config = FeeHistoryFetcherImpl.Config(
        feeHistoryBlockCount = configs.l1Submission.fallbackGasPrice.feeHistoryBlockCount,
        feeHistoryRewardPercentile = configs.l1Submission.fallbackGasPrice.feeHistoryRewardPercentile.toDouble(),
      ),
    )
  }

  val lineaRollupClientForFinalizationMonitor: LineaRollupSmartContractClientReadOnly =
    Web3JLineaRollupSmartContractClientReadOnly(
      contractAddress = configs.protocol.l1.contractAddress,
      web3j = createWeb3jHttpClient(
        rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.finalization-monitor"),
      ),
    )

  private val l1FinalizationMonitor = run {
    FinalizationMonitorImpl(
      config =
      FinalizationMonitorImpl.Config(
        pollingInterval = configs.l1FinalizationMonitor.l1PollingInterval,
        l1QueryBlockTag = configs.l1FinalizationMonitor.l1QueryBlockTag,
      ),
      contract = lineaRollupClientForFinalizationMonitor,
      l2Client = createWeb3jHttpClient(
        rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
        log = LogManager.getLogger("clients.l2.eth.finalization-monitor"),
      ),
      vertx = vertx,
    )
  }

  private val l1FinalizationHandlerForShomeiRpc: LongRunningService = run {
    val l2Web3jClient: Web3j = createWeb3jHttpClient(
      rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
      log = LogManager.getLogger("clients.l2.eth.shomei-frontend"),
    )
    setupL1FinalizationMonitorForShomeiFrontend(
      type2StateProofProviderConfig = configs.type2StateProofProvider,
      httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      lineaRollupClient = lineaRollupClientForFinalizationMonitor,
      l2Web3jClient = l2Web3jClient,
      vertx = vertx,
    )
  }

  private val l1FeeHistoriesRepository =
    FeeHistoriesRepositoryImpl(
      FeeHistoriesRepositoryImpl.Config(
        rewardPercentiles = configs.l1Submission!!.dynamicGasPriceCap.feeHistoryFetcher
          .rewardPercentiles.map { it.toDouble() },
        minBaseFeePerBlobGasToCache = configs.l1Submission.dynamicGasPriceCap
          .gasPriceCapCalculation.historicBaseFeePerBlobGasLowerBound,
        fixedAverageRewardToCache = configs.l1Submission.dynamicGasPriceCap
          .gasPriceCapCalculation.historicAvgRewardConstant,
      ),
      FeeHistoriesPostgresDao(
        sqlClient,
      ),
    )

  private val gasPriceCapProvider =
    if (configs.l1Submission.isEnabled() && configs.l1Submission!!.dynamicGasPriceCap.isEnabled()) {
      val feeHistoryPercentileWindowInBlocks =
        configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentileWindow
          .div(configs.protocol.l1.blockTime).toUInt()

      val feeHistoryPercentileWindowLeewayInBlocks =
        configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentileWindowLeeway
          .div(configs.protocol.l1.blockTime).toUInt()

      val l2Web3jClient: Web3j =
        createWeb3jHttpClient(
          rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
          log = LogManager.getLogger("clients.l2.eth.gascap-provider"),
        )

      GasPriceCapProviderImpl(
        config = GasPriceCapProviderImpl.Config(
          enabled = configs.l1Submission.dynamicGasPriceCap.isEnabled(),
          gasFeePercentile =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentile.toDouble(),
          gasFeePercentileWindowInBlocks = feeHistoryPercentileWindowInBlocks,
          gasFeePercentileWindowLeewayInBlocks = feeHistoryPercentileWindowLeewayInBlocks,
          timeOfDayMultipliers =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.timeOfTheDayMultipliers,
          adjustmentConstant =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.adjustmentConstant,
          blobAdjustmentConstant =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.blobAdjustmentConstant,
          finalizationTargetMaxDelay =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.finalizationTargetMaxDelay,
          gasPriceCapsCoefficient =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.gasPriceCapsCheckCoefficient,
        ),
        l2ExtendedWeb3JClient = ExtendedWeb3JImpl(l2Web3jClient),
        feeHistoriesRepository = l1FeeHistoriesRepository,
        gasPriceCapCalculator = GasPriceCapCalculatorImpl(),
      )
    } else {
      null
    }

  private val gasPriceCapProviderForDataSubmission = if (configs.l1Submission!!.dynamicGasPriceCap.isEnabled()) {
    GasPriceCapProviderForDataSubmission(
      config = GasPriceCapProviderForDataSubmission.Config(
        maxPriorityFeePerGasCap = configs.l1Submission.blob.gas.maxPriorityFeePerGasCap,
        maxFeePerGasCap = configs.l1Submission.blob.gas.maxFeePerGasCap,
        maxFeePerBlobGasCap = configs.l1Submission.blob.gas.maxFeePerBlobGasCap!!,
      ),
      gasPriceCapProvider = gasPriceCapProvider!!,
      metricsFacade = metricsFacade,
    )
  } else {
    null
  }

  private val gasPriceCapProviderForFinalization = if (configs.l1Submission!!.dynamicGasPriceCap.isEnabled()) {
    GasPriceCapProviderForFinalization(
      config = GasPriceCapProviderForFinalization.Config(
        maxPriorityFeePerGasCap = configs.l1Submission.aggregation.gas.maxPriorityFeePerGasCap,
        maxFeePerGasCap = configs.l1Submission.aggregation.gas.maxFeePerGasCap,
      ),
      gasPriceCapProvider = gasPriceCapProvider!!,
      metricsFacade = metricsFacade,
    )
  } else {
    null
  }

  private val lastFinalizedBlock = lastFinalizedBlock().get()
  private val lastProcessedBlockNumber = resumeConflationFrom(
    aggregationsRepository,
    lastFinalizedBlock,
  ).get()
  private val lastConsecutiveAggregatedBlockNumber = resumeAggregationFrom(
    aggregationsRepository,
    lastFinalizedBlock,
  ).get()

  val l2Web3jClientForBlockCreation: Web3j = createWeb3jHttpClient(
    rpcUrl = configs.conflation.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.conflation"),
  )

  private fun createDeadlineConflationCalculatorRunner(): DeadlineConflationCalculatorRunner? {
    if (configs.conflation.isDisabled() || configs.conflation.conflationDeadline == null) {
      log.info("Conflation deadline calculator is disabled")
      return null
    }

    return DeadlineConflationCalculatorRunner(
      conflationDeadlineCheckInterval = configs.conflation.conflationDeadlineCheckInterval,
      delegate = ConflationCalculatorByTimeDeadline(
        config = ConflationCalculatorByTimeDeadline.Config(
          conflationDeadline = configs.conflation.conflationDeadline,
          conflationDeadlineLastBlockConfirmationDelay =
          configs.conflation.conflationDeadlineLastBlockConfirmationDelay,
        ),
        lastBlockNumber = lastProcessedBlockNumber,
        clock = Clock.System,
        latestBlockProvider = GethCliqueSafeBlockProvider(
          l2Web3jClientForBlockCreation,
          GethCliqueSafeBlockProvider.Config(blocksToFinalization = 0),
        ),
      ),
    )
  }

  private val deadlineConflationCalculatorRunner = createDeadlineConflationCalculatorRunner()

  private fun addBlocksLimitCalculatorIfDefined(calculators: MutableList<ConflationCalculator>) {
    if (configs.conflation.blocksLimit != null) {
      calculators.add(
        ConflationCalculatorByBlockLimit(
          blockLimit = configs.conflation.blocksLimit,
        ),
      )
    }
  }

  private fun addTargetEndBlockConflationCalculatorIfDefined(calculators: MutableList<ConflationCalculator>) {
    if (configs.conflation.proofAggregation.targetEndBlocks?.isNotEmpty() ?: false) {
      calculators.add(
        ConflationCalculatorByTargetBlockNumbers(
          targetEndBlockNumbers = configs.conflation.proofAggregation.targetEndBlocks!!.toSet(),
        ),
      )
    }
  }

  private fun createCalculatorsForBlobsAndConflation(
    logger: Logger,
    compressedBlobCalculator: ConflationCalculatorByDataCompressed,
  ): List<ConflationCalculator> {
    val calculators: MutableList<ConflationCalculator> =
      mutableListOf(
        ConflationCalculatorByExecutionTraces(
          tracesCountersLimit = configs.conflation.tracesLimitsV2,
          emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
          metricsFacade = metricsFacade,
          log = logger,
        ),
        compressedBlobCalculator,
      )
    addBlocksLimitCalculatorIfDefined(calculators)
    addTargetEndBlockConflationCalculatorIfDefined(calculators)
    return calculators
  }

  private val conflationCalculator: TracesConflationCalculator = run {
    val logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java)

    // To fail faster for JNA reasons
    val blobCompressor = GoBackedBlobCompressor.getInstance(
      compressorVersion = configs.conflation.blobCompression.blobCompressorVersion,
      dataLimit = configs.conflation.blobCompression.blobSizeLimit,
      metricsFacade = metricsFacade,
    )

    val compressedBlobCalculator = ConflationCalculatorByDataCompressed(
      blobCompressor = blobCompressor,
    )
    val globalCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastProcessedBlockNumber,
      syncCalculators = createCalculatorsForBlobsAndConflation(logger, compressedBlobCalculator),
      deferredTriggerConflationCalculators = listOfNotNull(deadlineConflationCalculatorRunner),
      emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      log = logger,
    )

    val batchesLimit = configs.conflation.blobCompression.batchesLimit
      ?: (configs.conflation.proofAggregation.proofsLimit - 1U)
    GlobalBlobAwareConflationCalculator(
      conflationCalculator = globalCalculator,
      blobCalculator = compressedBlobCalculator,
      metricsFacade = metricsFacade,
      batchesLimit = batchesLimit,
    )
  }
  private val conflationService: ConflationService =
    ConflationServiceImpl(calculator = conflationCalculator, metricsFacade = metricsFacade)

  private val zkStateClient: StateManagerClientV1 = StateManagerV1JsonRpcClient.create(
    rpcClientFactory = httpJsonRpcClientFactory,
    endpoints = configs.stateManager.endpoints.map { it.toURI() },
    maxInflightRequestsPerClient = configs.stateManager.requestLimitPerEndpoint,
    requestRetry = configs.stateManager.requestRetries.toJsonRpcRetry(),
    zkStateManagerVersion = configs.stateManager.version,
    logger = LogManager.getLogger("clients.StateManagerShomeiClient"),
  )

  private val lineaSmartContractClientForDataSubmission: LineaRollupSmartContractClient = run {
    // The below gas provider will act as the primary gas provider if L1
    // dynamic gas pricing is disabled and will act as a fallback gas provider
    // if L1 dynamic gas pricing is enabled
    val primaryOrFallbackGasProvider = WMAGasProvider(
      chainId = l1ChainId.toLong(),
      feesFetcher = feesFetcher,
      priorityFeeCalculator = l1DataSubmissionPriorityFeeCalculator,
      config = WMAGasProvider.Config(
        gasLimit = configs.l1Submission!!.blob.gas.gasLimit,
        maxFeePerGasCap = configs.l1Submission.blob.gas.maxFeePerGasCap,
        maxPriorityFeePerGasCap = configs.l1Submission.blob.gas.maxPriorityFeePerGasCap,
        maxFeePerBlobGasCap = configs.l1Submission.blob.gas.maxFeePerBlobGasCap!!,
      ),
    )
    val l1Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.l1Submission.blob.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.data-submission"),
    )
    createLineaRollupContractClient(
      contractAddress = configs.protocol.l1.contractAddress,
      transactionManager = createTransactionManager(
        vertx,
        signerConfig = configs.l1Submission.blob.signer,
        client = l1Web3jClient,
      ),
      contractGasProvider = primaryOrFallbackGasProvider,
      web3jClient = l1Web3jClient,
      smartContractErrors = smartContractErrors,
      // eth_estimateGas would fail because we submit multiple blob tx
      // and 2nd would fail with revert reason
      useEthEstimateGas = false,
    )
  }

  private val genesisStateProvider = GenesisStateProvider(
    stateRootHash = configs.protocol.genesis.genesisStateRootHash,
    shnarf = configs.protocol.genesis.genesisShnarf,
  )

  private val blobCompressionProofCoordinator = run {
    val maxProvenBlobCache = run {
      val highestProvenBlobTracker = HighestProvenBlobTracker(lastProcessedBlockNumber)
      metricsFacade.createGauge(
        category = LineaMetricsCategory.BLOB,
        name = "proven.highest.block.number",
        description = "Highest proven blob compression block number",
        measurementSupplier = highestProvenBlobTracker,
      )
      highestProvenBlobTracker
    }
    val blobCompressionProofHandler: (BlobCompressionProofUpdate) -> SafeFuture<*> = SimpleCompositeSafeFutureHandler(
      listOf(
        maxProvenBlobCache,
      ),
    )

    val blobCompressionProofCoordinator = BlobCompressionProofCoordinator(
      vertx = vertx,
      blobsRepository = blobsRepository,
      blobCompressionProverClient = proverClientFactory.blobCompressionProverClient(),
      rollingBlobShnarfCalculator = RollingBlobShnarfCalculator(
        blobShnarfCalculator = GoBackedBlobShnarfCalculator(
          version = ShnarfCalculatorVersion.V1_2,
          metricsFacade = metricsFacade,
        ),
        blobsRepository = blobsRepository,
        genesisShnarf = genesisStateProvider.shnarf,
      ),
      blobZkStateProvider = BlobZkStateProviderImpl(
        zkStateClient = zkStateClient,
      ),
      config = BlobCompressionProofCoordinator.Config(
        pollingInterval = configs.conflation.blobCompression.handlerPollingInterval,
      ),
      blobCompressionProofHandler = blobCompressionProofHandler,
      metricsFacade = metricsFacade,
    )
    val highestUnprovenBlobTracker = HighestUnprovenBlobTracker(lastProcessedBlockNumber)
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "unproven.highest.block.number",
      description = "Block number of highest unproven blob produced",
      measurementSupplier = highestUnprovenBlobTracker,
    )

    val compositeSafeFutureHandler = SimpleCompositeSafeFutureHandler(
      listOf(
        blobCompressionProofCoordinator::handleBlob,
        highestUnprovenBlobTracker,
      ),
    )
    conflationCalculator.onBlobCreation(compositeSafeFutureHandler)
    blobCompressionProofCoordinator
  }

  private val highestAcceptedBlobTracker = HighestULongTracker(lastProcessedBlockNumber).also {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "highest.accepted.block.number",
      description = "Highest accepted blob end block number",
      measurementSupplier = it,
    )
  }

  private val alreadySubmittedBlobsFilter =
    L1ShnarfBasedAlreadySubmittedBlobsFilter(
      lineaRollup = lineaSmartContractClientForDataSubmission,
      acceptedBlobEndBlockNumberConsumer = { highestAcceptedBlobTracker(it) },
    )

  private val latestBlobSubmittedBlockNumberTracker = LatestBlobSubmittedBlockNumberTracker(0UL)
  private val blobSubmissionCoordinator = run {
    if (configs.l1Submission.isDisabled() || configs.l1Submission!!.blob.isDisabled()) {
      DisabledLongRunningService
    } else {
      metricsFacade.createGauge(
        category = LineaMetricsCategory.BLOB,
        name = "highest.submitted.on.l1",
        description = "Highest submitted blob end block number on l1",
        measurementSupplier = { latestBlobSubmittedBlockNumberTracker.get() },
      )

      val blobSubmissionDelayHistogram = metricsFacade.createHistogram(
        category = LineaMetricsCategory.BLOB,
        name = "submission.delay",
        description = "Delay between blob submission and end block timestamps",
        baseUnit = "seconds",
      )

      val blobSubmittedEventConsumers: Map<Consumer<BlobSubmittedEvent>, String> = mapOf(
        Consumer<BlobSubmittedEvent> { blobSubmission ->
          latestBlobSubmittedBlockNumberTracker(blobSubmission)
        } to "Submitted Blob Tracker Consumer",
        Consumer<BlobSubmittedEvent> { blobSubmission ->
          blobSubmissionDelayHistogram.record(blobSubmission.getSubmissionDelay().toDouble())
        } to "Blob Submission Delay Consumer",
      )

      BlobSubmissionCoordinator.create(
        config = BlobSubmissionCoordinator.Config(
          pollingInterval = configs.l1Submission.blob.submissionTickInterval,
          proofSubmissionDelay = configs.l1Submission.blob.submissionDelay,
          maxBlobsToSubmitPerTick =
          configs.l1Submission.blob.maxSubmissionTransactionsPerTick *
            configs.l1Submission.blob.targetBlobsPerTransaction,
          targetBlobsToSubmitPerTx = configs.l1Submission.blob.targetBlobsPerTransaction,
        ),
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        lineaSmartContractClient = lineaSmartContractClientForDataSubmission,
        gasPriceCapProvider = gasPriceCapProviderForDataSubmission,
        alreadySubmittedBlobsFilter = alreadySubmittedBlobsFilter,
        blobSubmittedEventDispatcher = EventDispatcher(blobSubmittedEventConsumers),
        vertx = vertx,
        clock = Clock.System,
      )
    }
  }

  private val proofAggregationCoordinatorService: LongRunningService = run {
    val maxBlobEndBlockNumberTracker = ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTracker(
      aggregationsRepository,
      lastProcessedBlockNumber,
    )

    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "proven.highest.consecutive.block.number",
      description = "Highest consecutive proven blob compression block number",
      measurementSupplier = maxBlobEndBlockNumberTracker,
    )

    val highestAggregationTracker = HighestULongTracker(lastConsecutiveAggregatedBlockNumber)
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "proven.highest.block.number",
      description = "Highest proven aggregation block number",
      measurementSupplier = highestAggregationTracker,
    )

    val l2Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.conflation.l2Endpoint.toString(),
      log = LogManager.getLogger("clients.l2.eth.conflation"),
    )

    ProofAggregationCoordinatorService
      .create(
        vertx = vertx,
        aggregationCoordinatorPollingInterval = configs.conflation.proofAggregation.coordinatorPollingInterval,
        deadlineCheckInterval = configs.conflation.proofAggregation.deadlineCheckInterval,
        aggregationDeadline = configs.conflation.proofAggregation.deadline,
        latestBlockProvider = GethCliqueSafeBlockProvider(
          web3j = l2Web3jClient,
          config = GethCliqueSafeBlockProvider.Config(0),
        ),
        maxProofsPerAggregation = configs.conflation.proofAggregation.proofsLimit,
        startBlockNumberInclusive = lastConsecutiveAggregatedBlockNumber + 1u,
        aggregationsRepository = aggregationsRepository,
        consecutiveProvenBlobsProvider = maxBlobEndBlockNumberTracker,
        proofAggregationClient = proverClientFactory.proofAggregationProverClient(),
        l2EthApiClient = createEthApiClient(
          l2Web3jClient,
          requestRetryConfig = linea.domain.RetryConfig(
            backoffDelay = 1.seconds,
            failuresWarningThreshold = 3u,
          ),
          vertx = vertx,
        ),
        l2MessageService = Web3JL2MessageServiceSmartContractClient.createReadOnly(
          web3jClient = l2Web3jClient,
          contractAddress = configs.protocol.l2.contractAddress,
          smartContractErrors = smartContractErrors,
          smartContractDeploymentBlockNumber = configs.protocol.l2.contractDeploymentBlockNumber?.getNumber(),
        ),
        aggregationDeadlineDelay = configs.conflation.conflationDeadlineLastBlockConfirmationDelay,
        targetEndBlockNumbers = configs.conflation.proofAggregation.targetEndBlocks ?: emptyList(),
        metricsFacade = metricsFacade,
        provenAggregationEndBlockNumberConsumer = { aggEndBlockNumber -> highestAggregationTracker(aggEndBlockNumber) },
        aggregationSizeMultipleOf = configs.conflation.proofAggregation.aggregationSizeMultipleOf,
      )
  }

  private val aggregationFinalizationCoordinator = run {
    if (configs.l1Submission.isDisabled() || configs.l1Submission?.aggregation.isDisabled()) {
      DisabledLongRunningService
    } else {
      configs.l1Submission!!

      val l1Web3jClient = createWeb3jHttpClient(
        rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.finalization"),
      )
      // The below gas provider will act as the primary gas provider if L1
      // dynamic gas pricing is disabled and will act as a fallback gas provider
      // if L1 dynamic gas pricing is enabled
      val primaryOrFallbackGasProvider = WMAGasProvider(
        chainId = l1ChainId.toLong(),
        feesFetcher = feesFetcher,
        priorityFeeCalculator = l1FinalizationPriorityFeeCalculator,
        config = WMAGasProvider.Config(
          gasLimit = configs.l1Submission.aggregation.gas.gasLimit,
          maxFeePerGasCap = configs.l1Submission.aggregation.gas.maxFeePerGasCap,
          maxPriorityFeePerGasCap = configs.l1Submission.aggregation.gas.maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = 0UL, // we do not submit blobs in finalization tx
        ),
      )
      val lineaSmartContractClientForFinalization: LineaRollupSmartContractClient = createLineaRollupContractClient(
        contractAddress = configs.protocol.l1.contractAddress,
        transactionManager = finalizationTransactionManager,
        contractGasProvider = primaryOrFallbackGasProvider,
        web3jClient = l1Web3jClient,
        smartContractErrors = smartContractErrors,
        useEthEstimateGas = true,
      )

      val latestFinalizationSubmittedBlockNumberTracker = LatestFinalizationSubmittedBlockNumberTracker(0UL)
      metricsFacade.createGauge(
        category = LineaMetricsCategory.AGGREGATION,
        name = "highest.submitted.on.l1",
        description = "Highest submitted finalization end block number on l1",
        measurementSupplier = { latestFinalizationSubmittedBlockNumberTracker.get() },
      )

      val finalizationSubmissionDelayHistogram = metricsFacade.createHistogram(
        category = LineaMetricsCategory.AGGREGATION,
        name = "submission.delay",
        description = "Delay between finalization submission and end block timestamps",
        baseUnit = "seconds",
      )

      val submittedFinalizationConsumers: Map<Consumer<FinalizationSubmittedEvent>, String> = mapOf(
        Consumer<FinalizationSubmittedEvent> { finalizationSubmission ->
          latestFinalizationSubmittedBlockNumberTracker(finalizationSubmission)
        } to "Finalization Submission Consumer",
        Consumer<FinalizationSubmittedEvent> { finalizationSubmission ->
          finalizationSubmissionDelayHistogram.record(finalizationSubmission.getSubmissionDelay().toDouble())
        } to "Finalization Submission Delay Consumer",
      )

      AggregationFinalizationCoordinator.create(
        config = AggregationFinalizationCoordinator.Config(
          pollingInterval = configs.l1Submission.aggregation.submissionTickInterval,
          proofSubmissionDelay = configs.l1Submission.aggregation.submissionDelay,
        ),
        aggregationsRepository = aggregationsRepository,
        blobsRepository = blobsRepository,
        lineaRollup = lineaSmartContractClientForFinalization,
        alreadySubmittedBlobFilter = alreadySubmittedBlobsFilter,
        aggregationSubmitter = AggregationSubmitterImpl(
          lineaRollup = lineaSmartContractClientForFinalization,
          gasPriceCapProvider = gasPriceCapProviderForFinalization,
          aggregationSubmittedEventConsumer = EventDispatcher(submittedFinalizationConsumers),
        ),
        vertx = vertx,
        clock = Clock.System,
      )
    }
  }

  private val block2BatchCoordinator = run {
    val tracesCountersClient = run {
      val tracesCountersLog = LogManager.getLogger("clients.traces.counters")
      TracesGeneratorJsonRpcClientV2(
        vertx = vertx,
        rpcClient = httpJsonRpcClientFactory.createWithLoadBalancing(
          endpoints = configs.traces.counters.endpoints.toSet(),
          maxInflightRequestsPerClient = configs.traces.counters.requestLimitPerEndpoint,
          log = tracesCountersLog,
        ),
        config = TracesGeneratorJsonRpcClientV2.Config(
          expectedTracesApiVersion = configs.traces.expectedTracesApiVersion,
        ),
        retryConfig = configs.traces.counters.requestRetries.toJsonRpcRetry(),
        log = tracesCountersLog,
      )
    }

    val tracesConflationClient = run {
      val tracesConflationLog = LogManager.getLogger("clients.traces.conflation")
      TracesGeneratorJsonRpcClientV2(
        vertx = vertx,
        rpcClient = httpJsonRpcClientFactory.createWithLoadBalancing(
          endpoints = configs.traces.conflation.endpoints.toSet(),
          maxInflightRequestsPerClient = configs.traces.conflation.requestLimitPerEndpoint,
          log = tracesConflationLog,
        ),
        config = TracesGeneratorJsonRpcClientV2.Config(
          expectedTracesApiVersion = configs.traces.expectedTracesApiVersion,
        ),
        retryConfig = configs.traces.conflation.requestRetries.toJsonRpcRetry(),
        log = tracesConflationLog,
      )
    }

    val blobsConflationHandler: (BlocksConflation) -> SafeFuture<*> = run {
      val maxProvenBatchCache = run {
        val highestProvenBatchTracker = HighestProvenBatchTracker(lastProcessedBlockNumber)
        metricsFacade.createGauge(
          category = LineaMetricsCategory.BATCH,
          name = "proven.highest.block.number",
          description = "Highest proven batch execution block number",
          measurementSupplier = highestProvenBatchTracker,
        )
        highestProvenBatchTracker
      }

      val batchProofHandler = SimpleCompositeSafeFutureHandler(
        listOf(
          maxProvenBatchCache,
          BatchProofHandlerImpl(batchesRepository)::acceptNewBatch,
        ),
      )
      val executionProverClient: ExecutionProverClientV2 = proverClientFactory.executionProverClient(
        // we cannot use configs.traces.expectedTracesApiVersion because it breaks prover expected version pattern
        tracesVersion = "2.1.0",
        stateManagerVersion = configs.stateManager.version,
      )

      val proofGeneratingConflationHandlerImpl = ProofGeneratingConflationHandlerImpl(
        tracesProductionCoordinator = TracesConflationCoordinatorImpl(tracesConflationClient, zkStateClient),
        zkProofProductionCoordinator = ZkProofCreationCoordinatorImpl(
          executionProverClient = executionProverClient,
          l2EthApiClient = createEthApiClient(
            rpcUrl = configs.conflation.l2Endpoint.toString(),
            log = LogManager.getLogger("clients.l2.eth.conflation"),
            requestRetryConfig = configs.conflation.l2RequestRetries,
            vertx = vertx,
          ),
          messageServiceAddress = configs.protocol.l2.contractAddress,
        ),
        batchProofHandler = batchProofHandler,
        vertx = vertx,
        config = ProofGeneratingConflationHandlerImpl.Config(5.seconds),
      )

      val highestConflationTracker = HighestConflationTracker(lastProcessedBlockNumber)
      metricsFacade.createGauge(
        category = LineaMetricsCategory.CONFLATION,
        name = "last.block.number",
        description = "Last conflated block number",
        measurementSupplier = highestConflationTracker,
      )
      val conflationsCounter = metricsFacade.createCounter(
        category = LineaMetricsCategory.CONFLATION,
        name = "counter",
        description = "Counter of new conflations",
      )

      SimpleCompositeSafeFutureHandler(
        listOf(
          proofGeneratingConflationHandlerImpl::handleConflatedBatch,
          highestConflationTracker,
          {
            conflationsCounter.increment()
            SafeFuture.COMPLETE
          },
        ),
      )
    }

    conflationService.onConflatedBatch(blobsConflationHandler)

    BlockToBatchSubmissionCoordinator(
      conflationService = conflationService,
      tracesCountersClient = tracesCountersClient,
      vertx = vertx,
      encoder = BlockRLPEncoder,
    )
  }

  private val lastProvenBlockNumberProvider = run {
    val lastProvenConsecutiveBatchBlockNumberProvider = BatchesRepoBasedLastProvenBlockNumberProvider(
      lastProcessedBlockNumber.toLong(),
      batchesRepository,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "proven.highest.consecutive.block.number",
      description = "Highest proven consecutive execution batch block number",
      measurementSupplier = { lastProvenConsecutiveBatchBlockNumberProvider.getLastKnownProvenBlockNumber() },
    )
    lastProvenConsecutiveBatchBlockNumberProvider
  }

  private val blockCreationMonitor = run {
    log.info("Resuming conflation from block={} inclusive", lastProcessedBlockNumber + 1UL)
    val blockCreationMonitor = BlockCreationMonitor(
      vertx = vertx,
      web3j = ExtendedWeb3JImpl(l2Web3jClientForBlockCreation),
      startingBlockNumberExclusive = lastProcessedBlockNumber.toLong(),
      blockCreationListener = block2BatchCoordinator,
      lastProvenBlockNumberProviderAsync = lastProvenBlockNumberProvider,
      config = BlockCreationMonitor.Config(
        pollingInterval = configs.conflation.blocksPollingInterval,
        blocksToFinalization = 0L,
        blocksFetchLimit = configs.conflation.l2FetchBlocksLimit.toLong(),
        // We need to add 1 to forceStopConflationAtBlockInclusive because conflation calculator requires
        // block_number = forceStopConflationAtBlockInclusive + 1 to trigger conflation at
        // forceStopConflationAtBlockInclusive
        lastL2BlockNumberToProcessInclusive = configs.conflation.forceStopConflationAtBlockInclusive?.inc(),
      ),
    )
    blockCreationMonitor
  }

  private fun lastFinalizedBlock(): SafeFuture<ULong> {
    val l1BasedLastFinalizedBlockProvider = L1BasedLastFinalizedBlockProvider(
      vertx,
      lineaRollupSmartContractClient = lineaRollupClientForFinalizationMonitor,
      consistentNumberOfBlocksOnL1 = configs.conflation.consistentNumberOfBlocksOnL1ToWait,
    )
    return l1BasedLastFinalizedBlockProvider.getLastFinalizedBlock()
  }

  private val messageAnchoringApp: LongRunningService = if (configs.messageAnchoring.isEnabled()) {
    configs.messageAnchoring!!
    val l1Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.messageAnchoring.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.message-anchoring"),
    )
    val l2Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.messageAnchoring.l2Endpoint.toString(),
      log = LogManager.getLogger("clients.l2.eth.message-anchoring"),
    )
    val l2TransactionManager = createTransactionManager(
      vertx = vertx,
      signerConfig = configs.messageAnchoring.signer,
      client = l2Web3jClient,
    )
    MessageAnchoringApp(
      vertx = vertx,
      config = MessageAnchoringApp.Config(
        l1RequestRetryConfig = configs.messageAnchoring.l1RequestRetries,
        l1PollingInterval = configs.messageAnchoring.l1EventScrapping.pollingInterval,
        l1SuccessBackoffDelay = configs.messageAnchoring.l1EventScrapping.ethLogsSearchSuccessBackoffDelay,
        l1ContractAddress = configs.protocol.l1.contractAddress,
        l1EventPollingTimeout = configs.messageAnchoring.l1EventScrapping.pollingTimeout,
        l1EventSearchBlockChunk = configs.messageAnchoring.l1EventScrapping.ethLogsSearchBlockChunkSize,
        l1HighestBlockTag = configs.messageAnchoring.l1HighestBlockTag,
        l2HighestBlockTag = configs.messageAnchoring.l2HighestBlockTag,
        anchoringTickInterval = configs.messageAnchoring.anchoringTickInterval,
        messageQueueCapacity = configs.messageAnchoring.messageQueueCapacity,
        maxMessagesToAnchorPerL2Transaction = configs.messageAnchoring.maxMessagesToAnchorPerL2Transaction,
      ),
      l1EthApiClient = createEthApiClient(
        web3jClient = l1Web3jClient,
        requestRetryConfig = null,
        vertx = vertx,
      ),
      l2MessageService = Web3JL2MessageServiceSmartContractClient.create(
        web3jClient = l2Web3jClient,
        contractAddress = configs.protocol.l2.contractAddress,
        gasLimit = configs.messageAnchoring.gas.gasLimit,
        maxFeePerGasCap = configs.messageAnchoring.gas.maxFeePerGasCap,
        feeHistoryBlockCount = configs.messageAnchoring.gas.feeHistoryBlockCount,
        feeHistoryRewardPercentile = configs.messageAnchoring.gas.feeHistoryRewardPercentile.toDouble(),
        transactionManager = l2TransactionManager,
        smartContractErrors = smartContractErrors,
        smartContractDeploymentBlockNumber = configs.protocol.l2.contractDeploymentBlockNumber?.getNumber(),
      ),
    )
  } else {
    DisabledLongRunningService
  }

  private val l2NetworkGasPricingService: L2NetworkGasPricingService? =
    if (configs.l2NetworkGasPricing.isEnabled()) {
      configs.l2NetworkGasPricing!!

      val legacyConfig = L2NetworkGasPricingService.LegacyGasPricingCalculatorConfig(
        transactionCostCalculatorConfig = TransactionCostCalculator.Config(
          sampleTransactionCostMultiplier = configs.l2NetworkGasPricing.flatRateGasPricing.plainTransferCostMultiplier,
          fixedCostWei = configs.l2NetworkGasPricing.gasPriceFixedCost,
          compressedTxSize = configs.l2NetworkGasPricing.flatRateGasPricing.compressedTxSize.toInt(),
          expectedGas = configs.l2NetworkGasPricing.flatRateGasPricing.expectedGas.toInt(),
        ),
        naiveGasPricingCalculatorConfig = null,
        legacyGasPricingCalculatorBounds = BoundableFeeCalculator.Config(
          feeUpperBound = configs.l2NetworkGasPricing.flatRateGasPricing.gasPriceUpperBound.toDouble(),
          feeLowerBound = configs.l2NetworkGasPricing.flatRateGasPricing.gasPriceLowerBound.toDouble(),
          feeMargin = 0.0,
        ),
      )

      val config = L2NetworkGasPricingService.Config(
        feeHistoryFetcherConfig = FeeHistoryFetcherImpl.Config(
          feeHistoryBlockCount = configs.l2NetworkGasPricing.feeHistoryBlockCount,
          feeHistoryRewardPercentile = configs.l2NetworkGasPricing.feeHistoryRewardPercentile.toDouble(),
        ),
        legacy = legacyConfig,
        jsonRpcGasPriceUpdaterConfig = null,
        // we do not use miner_setGasPrice RPC method, so we set it to infinite
        jsonRpcPriceUpdateInterval = Duration.INFINITE,
        // there no other way to work now without setting extra data into sequencer node
        extraDataPricingPropagationEnabled = true,
        extraDataUpdateInterval = configs.l2NetworkGasPricing.priceUpdateInterval,
        variableFeesCalculatorConfig = VariableFeesCalculator.Config(
          blobSubmissionExpectedExecutionGas = configs.l2NetworkGasPricing.dynamicGasPricing
            .blobSubmissionExpectedExecutionGas.toUInt(),
          bytesPerDataSubmission = configs.l2NetworkGasPricing.dynamicGasPricing.l1BlobGas.toUInt(),
          expectedBlobGas = configs.l2NetworkGasPricing.dynamicGasPricing.l1BlobGas.toUInt(),
          margin = configs.l2NetworkGasPricing.dynamicGasPricing.margin,
        ),
        variableFeesCalculatorBounds = BoundableFeeCalculator.Config(
          feeUpperBound = configs.l2NetworkGasPricing.dynamicGasPricing.variableCostUpperBound.toDouble(),
          feeLowerBound = configs.l2NetworkGasPricing.dynamicGasPricing.variableCostLowerBound.toDouble(),
          feeMargin = 0.0,
        ),
        extraDataCalculatorConfig = MinerExtraDataV1CalculatorImpl.Config(
          fixedCostInKWei = configs.l2NetworkGasPricing.gasPriceFixedCost.toKWeiUInt(),
          ethGasPriceMultiplier = 1.0,
        ),
        extraDataUpdaterConfig = ExtraDataV1UpdaterImpl.Config(
          sequencerEndpoint = configs.l2NetworkGasPricing.extraDataUpdateEndpoint,
          retryConfig = configs.l2NetworkGasPricing.extraDataUpdateRequestRetries.toJsonRpcRetry(),
        ),
      )
      val l1Web3jClient = createWeb3jHttpClient(
        rpcUrl = configs.l2NetworkGasPricing.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.l2pricing"),
      )
      L2NetworkGasPricingService(
        vertx = vertx,
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        l1Web3jClient = l1Web3jClient,
        l1Web3jService = Web3jBlobExtended(
          createWeb3jHttpService(
            rpcUrl = configs.l2NetworkGasPricing.l1Endpoint.toString(),
            log = LogManager.getLogger("clients.l1.eth.l2pricing"),
          ),
        ),
        config = config,
      )
    } else {
      null
    }

  private val l1FeeHistoryCachingService: LongRunningService =
    if (configs.l1Submission.isEnabled() && configs.l1Submission!!.dynamicGasPriceCap.isEnabled()) {
      val feeHistoryPercentileWindowInBlocks =
        configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentileWindow
          .div(configs.protocol.l1.blockTime).toUInt()
      val feeHistoryStoragePeriodInBlocks =
        configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.storagePeriod
          .div(configs.protocol.l1.blockTime).toUInt()

      val l1FeeHistoryWeb3jBlobExtClient = Web3jBlobExtended(
        createWeb3jHttpService(
          rpcUrl = configs.l1Submission.aggregation.l1Endpoint.toString(),
          log = LogManager.getLogger("clients.l1.eth.feehistory-cache"),
        ),
      )

      val l1FeeHistoryFetcher: GasPriceCapFeeHistoryFetcher = GasPriceCapFeeHistoryFetcherImpl(
        web3jService = l1FeeHistoryWeb3jBlobExtClient,
        config = GasPriceCapFeeHistoryFetcherImpl.Config(
          maxBlockCount = configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.maxBlockCount,
          rewardPercentiles = configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.rewardPercentiles
            .map { it.toDouble() },
        ),
      )

      val l1Web3jClient = createWeb3jHttpClient(
        rpcUrl = configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.feehistory-cache"),
      )

      FeeHistoryCachingService(
        config = FeeHistoryCachingService.Config(
          pollingInterval =
          configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.fetchInterval,
          feeHistoryMaxBlockCount =
          configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.maxBlockCount,
          gasFeePercentile =
          configs.l1Submission.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentile.toDouble(),
          feeHistoryStoragePeriodInBlocks = feeHistoryStoragePeriodInBlocks,
          feeHistoryWindowInBlocks = feeHistoryPercentileWindowInBlocks,
          numOfBlocksBeforeLatest =
          configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.numOfBlocksBeforeLatest,
        ),
        vertx = vertx,
        web3jClient = l1Web3jClient,
        feeHistoryFetcher = l1FeeHistoryFetcher,
        feeHistoriesRepository = l1FeeHistoriesRepository,
      )
    } else {
      DisabledLongRunningService
    }

  val highestAcceptedFinalizationTracker = HighestULongTracker(lastProcessedBlockNumber).also {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "highest.accepted.block.number",
      description = "Highest finalized accepted end block number",
      measurementSupplier = it,
    )
  }

  init {
    mapOf(
      "last_proven_block_provider" to FinalizationHandler { update: FinalizationMonitor.FinalizationUpdate ->
        lastProvenBlockNumberProvider.updateLatestL1FinalizedBlock(update.blockNumber.toLong())
      },
      "finalized records cleanup" to RecordsCleanupFinalizationHandler(
        batchesRepository = batchesRepository,
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
      ),
      "highest_accepted_finalization_on_l1" to FinalizationHandler { update: FinalizationMonitor.FinalizationUpdate ->
        highestAcceptedFinalizationTracker(update.blockNumber)
      },
    )
      .forEach { (handlerName, handler) ->
        l1FinalizationMonitor.addFinalizationHandler(handlerName, handler)
      }
  }

  override fun start(): CompletableFuture<Unit> {
    return cleanupDbDataAfterBlockNumbers(
      lastProcessedBlockNumber = lastProcessedBlockNumber,
      lastConsecutiveAggregatedBlockNumber = lastConsecutiveAggregatedBlockNumber,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository,
    )
      .thenCompose { l1FinalizationMonitor.start() }
      .thenCompose { l1FinalizationHandlerForShomeiRpc.start() }
      .thenCompose { blobSubmissionCoordinator.start() }
      .thenCompose { aggregationFinalizationCoordinator.start() }
      .thenCompose { proofAggregationCoordinatorService.start() }
      .thenCompose { messageAnchoringApp.start() }
      .thenCompose { l2NetworkGasPricingService?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { l1FeeHistoryCachingService.start() }
      .thenCompose { deadlineConflationCalculatorRunner?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { blockCreationMonitor.start() }
      .thenCompose { blobCompressionProofCoordinator.start() }
      .thenPeek {
        log.info("L1App started")
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      l1FinalizationMonitor.stop(),
      l1FinalizationHandlerForShomeiRpc.stop(),
      blobSubmissionCoordinator.stop(),
      aggregationFinalizationCoordinator.stop(),
      proofAggregationCoordinatorService.stop(),
      messageAnchoringApp.stop(),
      l2NetworkGasPricingService?.stop() ?: SafeFuture.completedFuture(Unit),
      l1FeeHistoryCachingService.stop(),
      blockCreationMonitor.stop(),
      deadlineConflationCalculatorRunner?.stop() ?: SafeFuture.completedFuture(Unit),
      blobCompressionProofCoordinator.stop(),
    )
      .thenApply { log.info("L1App Stopped") }
  }

  companion object {

    fun cleanupDbDataAfterBlockNumbers(
      lastProcessedBlockNumber: ULong,
      lastConsecutiveAggregatedBlockNumber: ULong,
      batchesRepository: BatchesRepository,
      blobsRepository: BlobsRepository,
      aggregationsRepository: AggregationsRepository,
    ): SafeFuture<*> {
      val blockNumberInclusiveToDeleteFrom = lastProcessedBlockNumber + 1u
      val cleanupBatches = batchesRepository.deleteBatchesAfterBlockNumber(blockNumberInclusiveToDeleteFrom.toLong())
      val cleanupBlobs = blobsRepository.deleteBlobsAfterBlockNumber(blockNumberInclusiveToDeleteFrom)
      val cleanupAggregations = aggregationsRepository
        .deleteAggregationsAfterBlockNumber((lastConsecutiveAggregatedBlockNumber + 1u).toLong())

      return SafeFuture.allOf(cleanupBatches, cleanupBlobs, cleanupAggregations)
    }

    /**
     * Returns the last block number inclusive upto which we have consecutive proven blobs or the last finalized block
     * number inclusive
     */
    fun resumeConflationFrom(
      aggregationsRepository: AggregationsRepository,
      lastFinalizedBlock: ULong,
    ): SafeFuture<ULong> {
      return aggregationsRepository
        .findConsecutiveProvenBlobs(lastFinalizedBlock.toLong() + 1)
        .thenApply { blobAndBatchCounters ->
          if (blobAndBatchCounters.isNotEmpty()) {
            blobAndBatchCounters.last().blobCounters.endBlockNumber
          } else {
            lastFinalizedBlock
          }
        }
    }

    fun resumeAggregationFrom(
      aggregationsRepository: AggregationsRepository,
      lastFinalizedBlock: ULong,
    ): SafeFuture<ULong> {
      return aggregationsRepository
        .findHighestConsecutiveEndBlockNumber(lastFinalizedBlock.toLong() + 1)
        .thenApply { highestEndBlockNumber ->
          highestEndBlockNumber?.toULong() ?: lastFinalizedBlock
        }
    }

    fun setupL1FinalizationMonitorForShomeiFrontend(
      type2StateProofProviderConfig: linea.coordinator.config.v2.Type2StateProofManagerConfig,
      httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
      lineaRollupClient: LineaRollupSmartContractClientReadOnly,
      l2Web3jClient: Web3j,
      vertx: Vertx,
    ): LongRunningService {
      if (type2StateProofProviderConfig.isDisabled()) {
        return DisabledLongRunningService
      }

      val finalizedBlockNotifier = run {
        val log = LogManager.getLogger("clients.ForkChoiceUpdaterShomeiClient")
        val type2StateProofProviderClients = type2StateProofProviderConfig.endpoints.map {
          ShomeiClient(
            vertx = vertx,
            rpcClient = httpJsonRpcClientFactory.create(it, log = log),
            retryConfig = type2StateProofProviderConfig.requestRetries.toJsonRpcRetry(),
            log = log,
          )
        }

        ForkChoiceUpdaterImpl(type2StateProofProviderClients)
      }

      val l1FinalizationMonitor =
        FinalizationMonitorImpl(
          config =
          FinalizationMonitorImpl.Config(
            pollingInterval = type2StateProofProviderConfig.l1PollingInterval,
            l1QueryBlockTag = type2StateProofProviderConfig.l1QueryBlockTag,
          ),
          contract = lineaRollupClient,
          l2Client = l2Web3jClient,
          vertx = vertx,
        )

      l1FinalizationMonitor.addFinalizationHandler("type 2 state proof provider finalization updates", {
        finalizedBlockNotifier.updateFinalizedBlock(
          BlockNumberAndHash(it.blockNumber, it.blockHash.toArray()),
        )
      })

      return l1FinalizationMonitor
    }
  }
}
