package net.consensys.zkevm.coordinator.app

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import build.linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.BlockNumberAndHash
import linea.encoding.BlockRLPEncoder
import linea.web3j.ExtendedWeb3JImpl
import linea.web3j.SmartContractErrors
import linea.web3j.Web3JLogsClient
import linea.web3j.Web3jBlobExtended
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.blob.ShnarfCalculatorVersion
import net.consensys.linea.contract.Web3JL2MessageService
import net.consensys.linea.contract.Web3JL2MessageServiceLogsClient
import net.consensys.linea.contract.l1.GenesisStateProvider
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.WMAFeesCalculator
import net.consensys.linea.ethereum.gaspricing.WMAGasProvider
import net.consensys.linea.ethereum.gaspricing.dynamiccap.FeeHistoriesRepositoryWithCache
import net.consensys.linea.ethereum.gaspricing.dynamiccap.FeeHistoryCachingService
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapCalculator
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapCalculatorImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapFeeHistoryFetcher
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapFeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderForDataSubmission
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderForFinalization
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.zkevm.LongRunningService
import net.consensys.zkevm.coordinator.app.config.CoordinatorConfig
import net.consensys.zkevm.coordinator.app.config.Type2StateProofProviderConfig
import net.consensys.zkevm.coordinator.blockcreation.BatchesRepoBasedLastProvenBlockNumberProvider
import net.consensys.zkevm.coordinator.blockcreation.BlockCreationMonitor
import net.consensys.zkevm.coordinator.blockcreation.GethCliqueSafeBlockProvider
import net.consensys.zkevm.coordinator.blockcreation.TracesConflationClientV2Adapter
import net.consensys.zkevm.coordinator.blockcreation.TracesCountersClientV2Adapter
import net.consensys.zkevm.coordinator.blockcreation.TracesCountersV1WatcherClient
import net.consensys.zkevm.coordinator.blockcreation.TracesFilesManager
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.ShomeiClient
import net.consensys.zkevm.coordinator.clients.TracesGeneratorJsonRpcClientV1
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
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import java.util.function.Consumer
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toKotlinDuration

class L1DependentApp(
  private val configs: CoordinatorConfig,
  private val vertx: Vertx,
  private val l2Web3jClient: Web3j,
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  private val batchesRepository: BatchesRepository,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
  l1FeeHistoriesRepository: FeeHistoriesRepositoryWithCache,
  private val smartContractErrors: SmartContractErrors,
  private val metricsFacade: MetricsFacade
) : LongRunningService {
  private val log = LogManager.getLogger(this::class.java)

  init {
    if (configs.messageAnchoringService.disabled) {
      log.warn("Message anchoring service is disabled")
    }
    if (configs.l2NetworkGasPricingService == null) {
      log.warn("Dynamic gas price service is disabled")
    }
  }

  private val l2TransactionManager = createTransactionManager(
    vertx,
    configs.l2Signer,
    l2Web3jClient
  )

  private val l2MessageService = instantiateL2MessageServiceContractClient(
    configs.l2,
    l2TransactionManager,
    l2Web3jClient,
    smartContractErrors
  )
  private val l1Web3jClient = createWeb3jHttpClient(
    rpcUrl = configs.l1.rpcEndpoint.toString(),
    log = LogManager.getLogger("clients.l1.eth-api"),
    pollingInterval = 1.seconds
  )
  private val l1Web3jService = Web3jBlobExtended(HttpService(configs.l1.ethFeeHistoryEndpoint.toString()))

  private val l1ChainId = l1Web3jClient.ethChainId().send().chainId.toLong()

  private val l2MessageServiceLogsClient = Web3JL2MessageServiceLogsClient(
    logsClient = Web3JLogsClient(vertx, l2Web3jClient),
    l2MessageServiceAddress = configs.l2.messageServiceAddress
  )

  private val proverClientFactory = ProverClientFactory(
    vertx = vertx,
    config = configs.proversConfig,
    metricsFacade = metricsFacade
  )

  private val l2ExtendedWeb3j = ExtendedWeb3JImpl(l2Web3jClient)

  private val finalizationTransactionManager = createTransactionManager(
    vertx,
    configs.finalizationSigner,
    l1Web3jClient
  )

  private val l1MinPriorityFeeCalculator: FeesCalculator = WMAFeesCalculator(
    WMAFeesCalculator.Config(
      baseFeeCoefficient = 0.0,
      priorityFeeWmaCoefficient = 1.0
    )
  )

  private val l1DataSubmissionPriorityFeeCalculator: FeesCalculator = BoundableFeeCalculator(
    BoundableFeeCalculator.Config(
      feeUpperBound = configs.blobSubmission.priorityFeePerGasUpperBound.toDouble(),
      feeLowerBound = configs.blobSubmission.priorityFeePerGasLowerBound.toDouble(),
      feeMargin = 0.0
    ),
    l1MinPriorityFeeCalculator
  )

  private val l1FinalizationPriorityFeeCalculator: FeesCalculator = BoundableFeeCalculator(
    BoundableFeeCalculator.Config(
      feeUpperBound = configs.l1.maxPriorityFeePerGasCap.toDouble() * configs.l1.gasPriceCapMultiplierForFinalization,
      feeLowerBound = 0.0,
      feeMargin = 0.0
    ),
    l1MinPriorityFeeCalculator
  )

  private val feesFetcher: FeesFetcher = FeeHistoryFetcherImpl(
    l1Web3jClient,
    l1Web3jService,
    FeeHistoryFetcherImpl.Config(
      configs.l1.feeHistoryBlockCount.toUInt(),
      configs.l1.feeHistoryRewardPercentile
    )
  )

  private val lineaRollupClient: LineaRollupSmartContractClientReadOnly = Web3JLineaRollupSmartContractClientReadOnly(
    contractAddress = configs.l1.zkEvmContractAddress,
    web3j = l1Web3jClient
  )

  private val l1FinalizationMonitor = run {
    FinalizationMonitorImpl(
      config =
      FinalizationMonitorImpl.Config(
        pollingInterval = configs.l1.finalizationPollingInterval.toKotlinDuration(),
        l1QueryBlockTag = configs.l1.l1QueryBlockTag
      ),
      contract = lineaRollupClient,
      l2Client = l2Web3jClient,
      vertx = vertx
    )
  }

  private val l1FinalizationHandlerForShomeiRpc: LongRunningService = setupL1FinalizationMonitorForShomeiFrontend(
    type2StateProofProviderConfig = configs.type2StateProofProvider,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    lineaRollupClient = lineaRollupClient,
    l2Web3jClient = l2Web3jClient,
    vertx = vertx
  )

  private val gasPriceCapProvider =
    if (configs.l1DynamicGasPriceCapService.enabled) {
      val feeHistoryPercentileWindowInBlocks =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.gasFeePercentileWindow
          .toKotlinDuration().inWholeSeconds.div(configs.l1.blockTime.seconds).toUInt()

      val feeHistoryPercentileWindowLeewayInBlocks =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.gasFeePercentileWindowLeeway
          .toKotlinDuration().inWholeSeconds.div(configs.l1.blockTime.seconds).toUInt()

      val l1GasPriceCapCalculator: GasPriceCapCalculator = GasPriceCapCalculatorImpl()

      GasPriceCapProviderImpl(
        config = GasPriceCapProviderImpl.Config(
          enabled = configs.l1DynamicGasPriceCapService.enabled,
          gasFeePercentile =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.gasFeePercentile,
          gasFeePercentileWindowInBlocks = feeHistoryPercentileWindowInBlocks,
          gasFeePercentileWindowLeewayInBlocks = feeHistoryPercentileWindowLeewayInBlocks,
          timeOfDayMultipliers =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.timeOfDayMultipliers!!,
          adjustmentConstant =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.adjustmentConstant,
          blobAdjustmentConstant =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.blobAdjustmentConstant,
          finalizationTargetMaxDelay =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.finalizationTargetMaxDelay.toKotlinDuration(),
          gasPriceCapsCoefficient =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.gasPriceCapsCheckCoefficient
        ),
        l2ExtendedWeb3JClient = l2ExtendedWeb3j,
        feeHistoriesRepository = l1FeeHistoriesRepository,
        gasPriceCapCalculator = l1GasPriceCapCalculator
      )
    } else {
      null
    }

  private val gasPriceCapProviderForDataSubmission = if (configs.l1DynamicGasPriceCapService.enabled) {
    GasPriceCapProviderForDataSubmission(
      config = GasPriceCapProviderForDataSubmission.Config(
        maxPriorityFeePerGasCap = configs.l1.maxPriorityFeePerGasCap,
        maxFeePerGasCap = configs.l1.maxFeePerGasCap,
        maxFeePerBlobGasCap = configs.l1.maxFeePerBlobGasCap
      ),
      gasPriceCapProvider = gasPriceCapProvider!!,
      metricsFacade = metricsFacade
    )
  } else {
    null
  }

  private val gasPriceCapProviderForFinalization = if (configs.l1DynamicGasPriceCapService.enabled) {
    GasPriceCapProviderForFinalization(
      config = GasPriceCapProviderForFinalization.Config(
        maxPriorityFeePerGasCap = configs.l1.maxPriorityFeePerGasCap,
        maxFeePerGasCap = configs.l1.maxFeePerGasCap,
        gasPriceCapMultiplier = configs.l1.gasPriceCapMultiplierForFinalization
      ),
      gasPriceCapProvider = gasPriceCapProvider!!,
      metricsFacade = metricsFacade
    )
  } else {
    null
  }

  private val lastFinalizedBlock = lastFinalizedBlock().get()
  private val lastProcessedBlockNumber = resumeConflationFrom(
    aggregationsRepository,
    lastFinalizedBlock
  ).get()
  private val lastConsecutiveAggregatedBlockNumber = resumeAggregationFrom(
    aggregationsRepository,
    lastFinalizedBlock
  ).get()

  private fun createDeadlineConflationCalculatorRunner(): DeadlineConflationCalculatorRunner {
    return DeadlineConflationCalculatorRunner(
      conflationDeadlineCheckInterval = configs.conflation.conflationDeadlineCheckInterval.toKotlinDuration(),
      delegate = ConflationCalculatorByTimeDeadline(
        config = ConflationCalculatorByTimeDeadline.Config(
          conflationDeadline = configs.conflation.conflationDeadline.toKotlinDuration(),
          conflationDeadlineLastBlockConfirmationDelay =
          configs.conflation.conflationDeadlineLastBlockConfirmationDelay.toKotlinDuration()
        ),
        lastBlockNumber = lastProcessedBlockNumber,
        clock = Clock.System,
        latestBlockProvider = GethCliqueSafeBlockProvider(
          l2ExtendedWeb3j.web3jClient,
          GethCliqueSafeBlockProvider.Config(configs.l2.blocksToFinalization.toLong())
        )
      )
    )
  }

  private val deadlineConflationCalculatorRunnerOld = createDeadlineConflationCalculatorRunner()
  private val deadlineConflationCalculatorRunnerNew = createDeadlineConflationCalculatorRunner()

  private fun addBlocksLimitCalculatorIfDefined(calculators: MutableList<ConflationCalculator>) {
    if (configs.conflation.blocksLimit != null) {
      calculators.add(
        ConflationCalculatorByBlockLimit(
          blockLimit = configs.conflation.blocksLimit.toUInt()
        )
      )
    }
  }

  private fun addTargetEndBlockConflationCalculatorIfDefined(calculators: MutableList<ConflationCalculator>) {
    if (configs.conflation.conflationTargetEndBlockNumbers.isNotEmpty()) {
      calculators.add(
        ConflationCalculatorByTargetBlockNumbers(
          targetEndBlockNumbers = configs.conflation.conflationTargetEndBlockNumbers
        )
      )
    }
  }

  private fun createCalculatorsForBlobsAndConflation(
    logger: Logger,
    compressedBlobCalculator: ConflationCalculatorByDataCompressed
  ): List<ConflationCalculator> {
    val calculators: MutableList<ConflationCalculator> =
      mutableListOf(
        ConflationCalculatorByExecutionTraces(
          tracesCountersLimit = when (configs.traces.switchToLineaBesu) {
            true -> configs.conflation.tracesLimitsV2
            false -> configs.conflation.tracesLimitsV1
          },
          emptyTracesCounters = getEmptyTracesCounters(configs.traces.switchToLineaBesu),
          metricsFacade = metricsFacade,
          log = logger
        ),
        compressedBlobCalculator
      )
    addBlocksLimitCalculatorIfDefined(calculators)
    addTargetEndBlockConflationCalculatorIfDefined(calculators)
    return calculators
  }

  private val conflationCalculator: TracesConflationCalculator = run {
    val logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java)

    // To fail faster for JNA reasons
    val compressorVersion = configs.traces.blobCompressorVersion
    val blobCompressor = GoBackedBlobCompressor.getInstance(
      compressorVersion = compressorVersion,
      dataLimit = configs.blobCompression.blobSizeLimit.toUInt(),
      metricsFacade = metricsFacade
    )

    val compressedBlobCalculator = ConflationCalculatorByDataCompressed(
      blobCompressor = blobCompressor
    )
    val globalCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastProcessedBlockNumber,
      syncCalculators = createCalculatorsForBlobsAndConflation(logger, compressedBlobCalculator),
      deferredTriggerConflationCalculators = listOf(deadlineConflationCalculatorRunnerNew),
      emptyTracesCounters = getEmptyTracesCounters(configs.traces.switchToLineaBesu),
      log = logger
    )

    val batchesLimit =
      configs.blobCompression.batchesLimit ?: (configs.proofAggregation.aggregationProofsLimit.toUInt() - 1U)

    GlobalBlobAwareConflationCalculator(
      conflationCalculator = globalCalculator,
      blobCalculator = compressedBlobCalculator,
      metricsFacade = metricsFacade,
      batchesLimit = batchesLimit
    )
  }
  private val conflationService: ConflationService =
    ConflationServiceImpl(calculator = conflationCalculator, metricsFacade = metricsFacade)

  private val zkStateClient: StateManagerClientV1 = StateManagerV1JsonRpcClient.create(
    rpcClientFactory = httpJsonRpcClientFactory,
    endpoints = configs.stateManager.endpoints.map { it.toURI() },
    maxInflightRequestsPerClient = configs.stateManager.requestLimitPerEndpoint,
    requestRetry = configs.stateManager.requestRetryConfig,
    zkStateManagerVersion = configs.stateManager.version,
    logger = LogManager.getLogger("clients.StateManagerShomeiClient")
  )

  private val lineaSmartContractClientForDataSubmission: LineaRollupSmartContractClient = run {
    // The below gas provider will act as the primary gas provider if L1
    // dynamic gas pricing is disabled and will act as a fallback gas provider
    // if L1 dynamic gas pricing is enabled
    val primaryOrFallbackGasProvider = WMAGasProvider(
      chainId = l1ChainId,
      feesFetcher = feesFetcher,
      priorityFeeCalculator = l1DataSubmissionPriorityFeeCalculator,
      config = WMAGasProvider.Config(
        gasLimit = configs.l1.gasLimit,
        maxFeePerGasCap = configs.l1.maxFeePerGasCap,
        maxPriorityFeePerGasCap = configs.l1.maxPriorityFeePerGasCap,
        maxFeePerBlobGasCap = configs.l1.maxFeePerBlobGasCap
      )
    )
    createLineaRollupContractClient(
      l1Config = configs.l1,
      transactionManager = createTransactionManager(
        vertx,
        configs.dataSubmissionSigner,
        l1Web3jClient
      ),
      contractGasProvider = primaryOrFallbackGasProvider,
      web3jClient = l1Web3jClient,
      smartContractErrors = smartContractErrors,
      useEthEstimateGas = configs.blobSubmission.useEthEstimateGas
    )
  }

  private val genesisStateProvider = GenesisStateProvider(
    configs.l1.genesisStateRootHash,
    configs.l1.genesisShnarfV6
  )

  private val blobCompressionProofCoordinator = run {
    val maxProvenBlobCache = run {
      val highestProvenBlobTracker = HighestProvenBlobTracker(lastProcessedBlockNumber)
      metricsFacade.createGauge(
        category = LineaMetricsCategory.BLOB,
        name = "proven.highest.block.number",
        description = "Highest proven blob compression block number",
        measurementSupplier = highestProvenBlobTracker
      )
      highestProvenBlobTracker
    }
    val blobCompressionProofHandler: (BlobCompressionProofUpdate) -> SafeFuture<*> = SimpleCompositeSafeFutureHandler(
      listOf(
        maxProvenBlobCache
      )
    )
    val blobShnarfCalculatorVersion = if (configs.traces.switchToLineaBesu) {
      ShnarfCalculatorVersion.V1_0_1
    } else {
      ShnarfCalculatorVersion.V0_1_0
    }

    val blobCompressionProofCoordinator = BlobCompressionProofCoordinator(
      vertx = vertx,
      blobsRepository = blobsRepository,
      blobCompressionProverClient = proverClientFactory.blobCompressionProverClient(),
      rollingBlobShnarfCalculator = RollingBlobShnarfCalculator(
        blobShnarfCalculator = GoBackedBlobShnarfCalculator(
          version = blobShnarfCalculatorVersion,
          metricsFacade = metricsFacade
        ),
        blobsRepository = blobsRepository,
        genesisShnarf = genesisStateProvider.shnarf
      ),
      blobZkStateProvider = BlobZkStateProviderImpl(
        zkStateClient = zkStateClient
      ),
      config = BlobCompressionProofCoordinator.Config(
        pollingInterval = configs.blobCompression.handlerPollingInterval.toKotlinDuration()
      ),
      blobCompressionProofHandler = blobCompressionProofHandler,
      metricsFacade = metricsFacade
    )
    val highestUnprovenBlobTracker = HighestUnprovenBlobTracker(lastProcessedBlockNumber)
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "unproven.highest.block.number",
      description = "Block number of highest unproven blob produced",
      measurementSupplier = highestUnprovenBlobTracker
    )

    val compositeSafeFutureHandler = SimpleCompositeSafeFutureHandler(
      listOf(
        blobCompressionProofCoordinator::handleBlob,
        highestUnprovenBlobTracker
      )
    )
    conflationCalculator.onBlobCreation(compositeSafeFutureHandler)
    blobCompressionProofCoordinator
  }

  private val highestAcceptedBlobTracker = HighestULongTracker(lastProcessedBlockNumber).also {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "highest.accepted.block.number",
      description = "Highest accepted blob end block number",
      measurementSupplier = it
    )
  }

  private val alreadySubmittedBlobsFilter =
    L1ShnarfBasedAlreadySubmittedBlobsFilter(
      lineaRollup = lineaSmartContractClientForDataSubmission,
      acceptedBlobEndBlockNumberConsumer = { highestAcceptedBlobTracker(it) }
    )

  private val latestBlobSubmittedBlockNumberTracker = LatestBlobSubmittedBlockNumberTracker(0UL)
  private val blobSubmissionCoordinator = run {
    if (!configs.blobSubmission.enabled) {
      DisabledLongRunningService
    } else {
      metricsFacade.createGauge(
        category = LineaMetricsCategory.BLOB,
        name = "highest.submitted.on.l1",
        description = "Highest submitted blob end block number on l1",
        measurementSupplier = { latestBlobSubmittedBlockNumberTracker.get() }
      )

      val blobSubmissionDelayHistogram = metricsFacade.createHistogram(
        category = LineaMetricsCategory.BLOB,
        name = "submission.delay",
        description = "Delay between blob submission and end block timestamps",
        baseUnit = "seconds"
      )

      val blobSubmittedEventConsumers: Map<Consumer<BlobSubmittedEvent>, String> = mapOf(
        Consumer<BlobSubmittedEvent> { blobSubmission ->
          latestBlobSubmittedBlockNumberTracker(blobSubmission)
        } to "Submitted Blob Tracker Consumer",
        Consumer<BlobSubmittedEvent> { blobSubmission ->
          blobSubmissionDelayHistogram.record(blobSubmission.getSubmissionDelay().toDouble())
        } to "Blob Submission Delay Consumer"
      )

      BlobSubmissionCoordinator.create(
        config = BlobSubmissionCoordinator.Config(
          configs.blobSubmission.dbPollingInterval.toKotlinDuration(),
          configs.blobSubmission.proofSubmissionDelay.toKotlinDuration(),
          configs.blobSubmission.maxBlobsToSubmitPerTick.toUInt()
        ),
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        lineaSmartContractClient = lineaSmartContractClientForDataSubmission,
        gasPriceCapProvider = gasPriceCapProviderForDataSubmission,
        alreadySubmittedBlobsFilter = alreadySubmittedBlobsFilter,
        blobSubmittedEventDispatcher = EventDispatcher(blobSubmittedEventConsumers),
        vertx = vertx,
        clock = Clock.System
      )
    }
  }

  private val proofAggregationCoordinatorService: LongRunningService = run {
    // it needs it's own client because internally set the blockNumber when making queries.
    // it does not make any transaction
    val messageService = instantiateL2MessageServiceContractClient(
      configs.l2,
      l2TransactionManager,
      l2Web3jClient,
      smartContractErrors
    )
    val l2MessageServiceClient = Web3JL2MessageService(
      vertx = vertx,
      l2MessageServiceLogsClient = l2MessageServiceLogsClient,
      web3jL2MessageService = messageService
    )

    val maxBlobEndBlockNumberTracker = ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTracker(
      aggregationsRepository,
      lastProcessedBlockNumber
    )

    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "proven.highest.consecutive.block.number",
      description = "Highest consecutive proven blob compression block number",
      measurementSupplier = maxBlobEndBlockNumberTracker
    )

    val highestAggregationTracker = HighestULongTracker(lastProcessedBlockNumber)
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "proven.highest.block.number",
      description = "Highest proven aggregation block number",
      measurementSupplier = highestAggregationTracker
    )

    ProofAggregationCoordinatorService
      .create(
        vertx = vertx,
        aggregationCoordinatorPollingInterval =
        configs.proofAggregation.aggregationCoordinatorPollingInterval.toKotlinDuration(),
        deadlineCheckInterval = configs.proofAggregation.deadlineCheckInterval.toKotlinDuration(),
        aggregationDeadline = configs.proofAggregation.aggregationDeadline.toKotlinDuration(),
        latestBlockProvider = GethCliqueSafeBlockProvider(
          l2ExtendedWeb3j.web3jClient,
          GethCliqueSafeBlockProvider.Config(configs.l2.blocksToFinalization.toLong())
        ),
        maxProofsPerAggregation = configs.proofAggregation.aggregationProofsLimit.toUInt(),
        startBlockNumberInclusive = lastConsecutiveAggregatedBlockNumber + 1u,
        aggregationsRepository = aggregationsRepository,
        consecutiveProvenBlobsProvider = maxBlobEndBlockNumberTracker,
        proofAggregationClient = proverClientFactory.proofAggregationProverClient(),
        l2web3jClient = l2Web3jClient,
        l2MessageServiceClient = l2MessageServiceClient,
        aggregationDeadlineDelay = configs.conflation.conflationDeadlineLastBlockConfirmationDelay.toKotlinDuration(),
        targetEndBlockNumbers = configs.proofAggregation.targetEndBlocks,
        metricsFacade = metricsFacade,
        provenAggregationEndBlockNumberConsumer = { highestAggregationTracker(it) },
        aggregationSizeMultipleOf = configs.proofAggregation.aggregationSizeMultipleOf.toUInt()
      )
  }

  private val aggregationFinalizationCoordinator = run {
    if (!configs.aggregationFinalization.enabled) {
      DisabledLongRunningService
    } else {
      // The below gas provider will act as the primary gas provider if L1
      // dynamic gas pricing is disabled and will act as a fallback gas provider
      // if L1 dynamic gas pricing is enabled
      val primaryOrFallbackGasProvider = WMAGasProvider(
        chainId = l1ChainId,
        feesFetcher = feesFetcher,
        priorityFeeCalculator = l1FinalizationPriorityFeeCalculator,
        config = WMAGasProvider.Config(
          gasLimit = configs.l1.gasLimit,
          maxFeePerGasCap = (
            configs.l1.maxFeePerGasCap.toDouble() *
              configs.l1.gasPriceCapMultiplierForFinalization
            ).toULong(),
          maxPriorityFeePerGasCap = (
            configs.l1.maxPriorityFeePerGasCap.toDouble() *
              configs.l1.gasPriceCapMultiplierForFinalization
            ).toULong(),
          maxFeePerBlobGasCap = configs.l1.maxFeePerBlobGasCap
        )
      )
      val lineaSmartContractClientForFinalization: LineaRollupSmartContractClient = createLineaRollupContractClient(
        l1Config = configs.l1,
        transactionManager = finalizationTransactionManager,
        contractGasProvider = primaryOrFallbackGasProvider,
        web3jClient = l1Web3jClient,
        smartContractErrors = smartContractErrors,
        useEthEstimateGas = configs.aggregationFinalization.useEthEstimateGas
      )

      val latestFinalizationSubmittedBlockNumberTracker = LatestFinalizationSubmittedBlockNumberTracker(0UL)
      metricsFacade.createGauge(
        category = LineaMetricsCategory.AGGREGATION,
        name = "highest.submitted.on.l1",
        description = "Highest submitted finalization end block number on l1",
        measurementSupplier = { latestFinalizationSubmittedBlockNumberTracker.get() }
      )

      val finalizationSubmissionDelayHistogram = metricsFacade.createHistogram(
        category = LineaMetricsCategory.AGGREGATION,
        name = "submission.delay",
        description = "Delay between finalization submission and end block timestamps",
        baseUnit = "seconds"
      )

      val submittedFinalizationConsumers: Map<Consumer<FinalizationSubmittedEvent>, String> = mapOf(
        Consumer<FinalizationSubmittedEvent> { finalizationSubmission ->
          latestFinalizationSubmittedBlockNumberTracker(finalizationSubmission)
        } to "Finalization Submission Consumer",
        Consumer<FinalizationSubmittedEvent> { finalizationSubmission ->
          finalizationSubmissionDelayHistogram.record(finalizationSubmission.getSubmissionDelay().toDouble())
        } to "Finalization Submission Delay Consumer"
      )

      AggregationFinalizationCoordinator.create(
        config = AggregationFinalizationCoordinator.Config(
          configs.aggregationFinalization.dbPollingInterval.toKotlinDuration(),
          configs.aggregationFinalization.proofSubmissionDelay.toKotlinDuration()
        ),
        aggregationsRepository = aggregationsRepository,
        blobsRepository = blobsRepository,
        lineaRollup = lineaSmartContractClientForFinalization,
        alreadySubmittedBlobFilter = alreadySubmittedBlobsFilter,
        aggregationSubmitter = AggregationSubmitterImpl(
          lineaRollup = lineaSmartContractClientForFinalization,
          gasPriceCapProvider = gasPriceCapProviderForFinalization,
          aggregationSubmittedEventConsumer = EventDispatcher(submittedFinalizationConsumers)
        ),
        vertx = vertx,
        clock = Clock.System
      )
    }
  }

  private val block2BatchCoordinator = run {
    val tracesCountersLog = LogManager.getLogger("clients.TracesCounters")
    val tracesCountersClient = when (configs.traces.switchToLineaBesu) {
      true -> {
        val tracesCounterV2Config = configs.traces.countersV2!!
        val expectedTracesApiVersionV2 = configs.traces.expectedTracesApiVersionV2!!
        val tracesCountersClientV2 = TracesGeneratorJsonRpcClientV2(
          vertx = vertx,
          rpcClient = httpJsonRpcClientFactory.createWithLoadBalancing(
            endpoints = tracesCounterV2Config.endpoints.toSet(),
            maxInflightRequestsPerClient = tracesCounterV2Config.requestLimitPerEndpoint,
            log = tracesCountersLog
          ),
          config = TracesGeneratorJsonRpcClientV2.Config(
            expectedTracesApiVersion = expectedTracesApiVersionV2
          ),
          retryConfig = tracesCounterV2Config.requestRetryConfig,
          log = tracesCountersLog
        )

        TracesCountersClientV2Adapter(tracesCountersClientV2 = tracesCountersClientV2)
      }

      false -> {
        val tracesFilesManager = TracesFilesManager(
          vertx,
          TracesFilesManager.Config(
            configs.traces.fileManager.rawTracesDirectory,
            configs.traces.fileManager.nonCanonicalRawTracesDirectory,
            configs.traces.fileManager.pollingInterval.toKotlinDuration(),
            configs.traces.fileManager.tracesFileCreationWaitTimeout.toKotlinDuration(),
            configs.traces.rawExecutionTracesVersion,
            configs.traces.fileManager.tracesFileExtension,
            configs.traces.fileManager.createNonCanonicalDirectory
          )
        )
        val tracesCountersClientV1 = TracesGeneratorJsonRpcClientV1(
          vertx = vertx,
          rpcClient = httpJsonRpcClientFactory.createWithLoadBalancing(
            endpoints = configs.traces.counters.endpoints.toSet(),
            maxInflightRequestsPerClient = configs.traces.counters.requestLimitPerEndpoint,
            log = tracesCountersLog
          ),
          config = TracesGeneratorJsonRpcClientV1.Config(
            rawExecutionTracesVersion = configs.traces.rawExecutionTracesVersion,
            expectedTracesApiVersion = configs.traces.expectedTracesApiVersion
          ),
          retryConfig = configs.traces.counters.requestRetryConfig,
          log = tracesCountersLog
        )

        TracesCountersV1WatcherClient(
          tracesFilesManager = tracesFilesManager,
          tracesCountersClientV1 = tracesCountersClientV1
        )
      }
    }

    val tracesConflationLog = LogManager.getLogger("clients.TracesConflation")
    val tracesConflationClient = when (configs.traces.switchToLineaBesu) {
      true -> {
        val tracesConflationConfigV2 = configs.traces.conflationV2!!
        val expectedTracesApiVersionV2 = configs.traces.expectedTracesApiVersionV2!!
        val tracesConflationClientV2 = TracesGeneratorJsonRpcClientV2(
          vertx = vertx,
          rpcClient = httpJsonRpcClientFactory.createWithLoadBalancing(
            endpoints = tracesConflationConfigV2.endpoints.toSet(),
            maxInflightRequestsPerClient = tracesConflationConfigV2.requestLimitPerEndpoint,
            log = tracesConflationLog
          ),
          config = TracesGeneratorJsonRpcClientV2.Config(
            expectedTracesApiVersion = expectedTracesApiVersionV2
          ),
          retryConfig = configs.traces.conflation.requestRetryConfig,
          log = tracesConflationLog
        )

        TracesConflationClientV2Adapter(
          tracesConflationClientV2 = tracesConflationClientV2
        )
      }

      false -> {
        TracesGeneratorJsonRpcClientV1(
          vertx = vertx,
          rpcClient = httpJsonRpcClientFactory.createWithLoadBalancing(
            endpoints = configs.traces.conflation.endpoints.toSet(),
            maxInflightRequestsPerClient = configs.traces.conflation.requestLimitPerEndpoint,
            log = tracesConflationLog
          ),
          config = TracesGeneratorJsonRpcClientV1.Config(
            rawExecutionTracesVersion = configs.traces.rawExecutionTracesVersion,
            expectedTracesApiVersion = configs.traces.expectedTracesApiVersion
          ),
          retryConfig = configs.traces.conflation.requestRetryConfig,
          log = tracesConflationLog
        )
      }
    }

    val blobsConflationHandler: (BlocksConflation) -> SafeFuture<*> = run {
      val maxProvenBatchCache = run {
        val highestProvenBatchTracker = HighestProvenBatchTracker(lastProcessedBlockNumber)
        metricsFacade.createGauge(
          category = LineaMetricsCategory.BATCH,
          name = "proven.highest.block.number",
          description = "Highest proven batch execution block number",
          measurementSupplier = highestProvenBatchTracker
        )
        highestProvenBatchTracker
      }

      val batchProofHandler = SimpleCompositeSafeFutureHandler(
        listOf(
          maxProvenBatchCache,
          BatchProofHandlerImpl(batchesRepository)::acceptNewBatch
        )
      )
      val executionProverClient: ExecutionProverClientV2 = proverClientFactory.executionProverClient(
        tracesVersion = configs.traces.rawExecutionTracesVersion,
        stateManagerVersion = configs.stateManager.version
      )

      val proofGeneratingConflationHandlerImpl = ProofGeneratingConflationHandlerImpl(
        tracesProductionCoordinator = TracesConflationCoordinatorImpl(tracesConflationClient, zkStateClient),
        zkProofProductionCoordinator = ZkProofCreationCoordinatorImpl(
          executionProverClient = executionProverClient,
          l2MessageServiceLogsClient = l2MessageServiceLogsClient,
          l2Web3jClient = l2Web3jClient
        ),
        batchProofHandler = batchProofHandler,
        vertx = vertx,
        config = ProofGeneratingConflationHandlerImpl.Config(5.seconds)
      )

      val highestConflationTracker = HighestConflationTracker(lastProcessedBlockNumber)
      metricsFacade.createGauge(
        category = LineaMetricsCategory.CONFLATION,
        name = "last.block.number",
        description = "Last conflated block number",
        measurementSupplier = highestConflationTracker
      )
      val conflationsCounter = metricsFacade.createCounter(
        category = LineaMetricsCategory.CONFLATION,
        name = "counter",
        description = "Counter of new conflations"
      )

      SimpleCompositeSafeFutureHandler(
        listOf(
          proofGeneratingConflationHandlerImpl::handleConflatedBatch,
          highestConflationTracker,
          {
            conflationsCounter.increment()
            SafeFuture.COMPLETE
          }
        )
      )
    }

    conflationService.onConflatedBatch(blobsConflationHandler)

    BlockToBatchSubmissionCoordinator(
      conflationService = conflationService,
      tracesCountersClient = tracesCountersClient,
      vertx = vertx,
      encoder = BlockRLPEncoder
    )
  }

  private val lastProvenBlockNumberProvider = run {
    val lastProvenConsecutiveBatchBlockNumberProvider = BatchesRepoBasedLastProvenBlockNumberProvider(
      lastProcessedBlockNumber.toLong(),
      batchesRepository
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "proven.highest.consecutive.block.number",
      description = "Highest proven consecutive execution batch block number",
      measurementSupplier = { lastProvenConsecutiveBatchBlockNumberProvider.getLastKnownProvenBlockNumber() }
    )
    lastProvenConsecutiveBatchBlockNumberProvider
  }

  private val blockCreationMonitor = run {
    log.info("Resuming conflation from block={} inclusive", lastProcessedBlockNumber + 1UL)
    val blockCreationMonitor = BlockCreationMonitor(
      vertx = vertx,
      web3j = l2ExtendedWeb3j,
      startingBlockNumberExclusive = lastProcessedBlockNumber.toLong(),
      blockCreationListener = block2BatchCoordinator,
      lastProvenBlockNumberProviderAsync = lastProvenBlockNumberProvider,
      config = BlockCreationMonitor.Config(
        pollingInterval = configs.l2.newBlockPollingInterval.toKotlinDuration(),
        blocksToFinalization = configs.l2.blocksToFinalization.toLong(),
        blocksFetchLimit = configs.conflation.fetchBlocksLimit.toLong(),
        // We need to add 1 to l2InclusiveBlockNumberToStopAndFlushAggregation because conflation calculator requires
        // block_number = l2InclusiveBlockNumberToStopAndFlushAggregation + 1 to trigger conflation at
        // l2InclusiveBlockNumberToStopAndFlushAggregation
        lastL2BlockNumberToProcessInclusive = configs.l2InclusiveBlockNumberToStopAndFlushAggregation?.let { it + 1uL }
      )
    )
    blockCreationMonitor
  }

  private fun lastFinalizedBlock(): SafeFuture<ULong> {
    val l1BasedLastFinalizedBlockProvider = L1BasedLastFinalizedBlockProvider(
      vertx,
      lineaRollupClient,
      configs.conflation.consistentNumberOfBlocksOnL1ToWait.toUInt()
    )
    return l1BasedLastFinalizedBlockProvider.getLastFinalizedBlock()
  }

  private val messageAnchoringApp: L1toL2MessageAnchoringApp? =
    if (configs.messageAnchoringService.enabled) {
      L1toL2MessageAnchoringApp(
        vertx,
        L1toL2MessageAnchoringApp.Config(
          configs.l1,
          configs.l2,
          configs.finalizationSigner,
          configs.l2Signer,
          configs.messageAnchoringService
        ),
        l1Web3jClient,
        l2Web3jClient,
        smartContractErrors,
        l2MessageService,
        l2TransactionManager
      )
    } else {
      null
    }

  private val l2NetworkGasPricingService: L2NetworkGasPricingService? =
    if (configs.l2NetworkGasPricingService != null) {
      L2NetworkGasPricingService(
        vertx = vertx,
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        l1Web3jClient = l1Web3jClient,
        l1Web3jService = l1Web3jService,
        config = configs.l2NetworkGasPricingService
      )
    } else {
      null
    }

  private val l1FeeHistoryCachingService: LongRunningService =
    if (configs.l1DynamicGasPriceCapService.enabled) {
      val feeHistoryPercentileWindowInBlocks =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.gasFeePercentileWindow
          .toKotlinDuration().inWholeSeconds.div(configs.l1.blockTime.seconds).toUInt()

      val feeHistoryStoragePeriodInBlocks =
        configs.l1DynamicGasPriceCapService.feeHistoryStorage.storagePeriod
          .toKotlinDuration().inWholeSeconds.div(configs.l1.blockTime.seconds).toUInt()

      val l1FeeHistoryWeb3jBlobExtClient = Web3jBlobExtended(
        HttpService(
          configs.l1DynamicGasPriceCapService.feeHistoryFetcher.endpoint?.toString()
            ?: configs.l1.ethFeeHistoryEndpoint.toString()
        )
      )

      val l1FeeHistoryFetcher: GasPriceCapFeeHistoryFetcher = GasPriceCapFeeHistoryFetcherImpl(
        web3jService = l1FeeHistoryWeb3jBlobExtClient,
        config = GasPriceCapFeeHistoryFetcherImpl.Config(
          maxBlockCount = configs.l1DynamicGasPriceCapService.feeHistoryFetcher.maxBlockCount,
          rewardPercentiles = configs.l1DynamicGasPriceCapService.feeHistoryFetcher.rewardPercentiles
        )
      )

      FeeHistoryCachingService(
        config = FeeHistoryCachingService.Config(
          pollingInterval =
          configs.l1DynamicGasPriceCapService.feeHistoryFetcher.fetchInterval.toKotlinDuration(),
          feeHistoryMaxBlockCount =
          configs.l1DynamicGasPriceCapService.feeHistoryFetcher.maxBlockCount,
          gasFeePercentile =
          configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.gasFeePercentile,
          feeHistoryStoragePeriodInBlocks = feeHistoryStoragePeriodInBlocks,
          feeHistoryWindowInBlocks = feeHistoryPercentileWindowInBlocks,
          numOfBlocksBeforeLatest =
          configs.l1DynamicGasPriceCapService.feeHistoryFetcher.numOfBlocksBeforeLatest
        ),
        vertx = vertx,
        web3jClient = l1Web3jClient,
        feeHistoryFetcher = l1FeeHistoryFetcher,
        feeHistoriesRepository = l1FeeHistoriesRepository
      )
    } else {
      DisabledLongRunningService
    }

  val highestAcceptedFinalizationTracker = HighestULongTracker(lastProcessedBlockNumber).also {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "highest.accepted.block.number",
      description = "Highest finalized accepted end block number",
      measurementSupplier = it
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
        aggregationsRepository = aggregationsRepository
      ),
      "highest_accepted_finalization_on_l1" to FinalizationHandler { update: FinalizationMonitor.FinalizationUpdate ->
        highestAcceptedFinalizationTracker(update.blockNumber)
      }
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
      aggregationsRepository = aggregationsRepository
    )
      .thenCompose { l1FinalizationMonitor.start() }
      .thenCompose { l1FinalizationHandlerForShomeiRpc.start() }
      .thenCompose { blobSubmissionCoordinator.start() }
      .thenCompose { aggregationFinalizationCoordinator.start() }
      .thenCompose { proofAggregationCoordinatorService.start() }
      .thenCompose { messageAnchoringApp?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { l2NetworkGasPricingService?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { l1FeeHistoryCachingService.start() }
      .thenCompose { deadlineConflationCalculatorRunnerOld.start() }
      .thenCompose { deadlineConflationCalculatorRunnerNew.start() }
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
      messageAnchoringApp?.stop() ?: SafeFuture.completedFuture(Unit),
      l2NetworkGasPricingService?.stop() ?: SafeFuture.completedFuture(Unit),
      l1FeeHistoryCachingService.stop(),
      blockCreationMonitor.stop(),
      deadlineConflationCalculatorRunnerOld.stop(),
      deadlineConflationCalculatorRunnerNew.stop(),
      blobCompressionProofCoordinator.stop()
    )
      .thenCompose { SafeFuture.fromRunnable { l1Web3jClient.shutdown() } }
      .thenApply { log.info("L1App Stopped") }
  }

  companion object {

    fun cleanupDbDataAfterBlockNumbers(
      lastProcessedBlockNumber: ULong,
      lastConsecutiveAggregatedBlockNumber: ULong,
      batchesRepository: BatchesRepository,
      blobsRepository: BlobsRepository,
      aggregationsRepository: AggregationsRepository
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
      lastFinalizedBlock: ULong
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
      lastFinalizedBlock: ULong
    ): SafeFuture<ULong> {
      return aggregationsRepository
        .findHighestConsecutiveEndBlockNumber(lastFinalizedBlock.toLong() + 1)
        .thenApply { highestEndBlockNumber ->
          highestEndBlockNumber?.toULong() ?: lastFinalizedBlock
        }
    }

    fun getEmptyTracesCounters(switchToLineaBesu: Boolean): TracesCounters {
      return when (switchToLineaBesu) {
        true -> TracesCountersV2.EMPTY_TRACES_COUNT
        false -> TracesCountersV1.EMPTY_TRACES_COUNT
      }
    }

    fun setupL1FinalizationMonitorForShomeiFrontend(
      type2StateProofProviderConfig: Type2StateProofProviderConfig?,
      httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
      lineaRollupClient: LineaRollupSmartContractClientReadOnly,
      l2Web3jClient: Web3j,
      vertx: Vertx
    ): LongRunningService {
      if (type2StateProofProviderConfig == null || type2StateProofProviderConfig.endpoints.isEmpty()) {
        return DisabledLongRunningService
      }

      val finalizedBlockNotifier = run {
        val log = LogManager.getLogger("clients.ForkChoiceUpdaterShomeiClient")
        val type2StateProofProviderClients = type2StateProofProviderConfig.endpoints.map {
          ShomeiClient(
            vertx = vertx,
            rpcClient = httpJsonRpcClientFactory.create(it, log = log),
            retryConfig = type2StateProofProviderConfig.requestRetryConfig,
            log = log
          )
        }

        ForkChoiceUpdaterImpl(type2StateProofProviderClients)
      }

      val l1FinalizationMonitor =
        FinalizationMonitorImpl(
          config =
          FinalizationMonitorImpl.Config(
            pollingInterval = type2StateProofProviderConfig.l1PollingInterval.toKotlinDuration(),
            l1QueryBlockTag = type2StateProofProviderConfig.l1QueryBlockTag
          ),
          contract = lineaRollupClient,
          l2Client = l2Web3jClient,
          vertx = vertx
        )

      l1FinalizationMonitor.addFinalizationHandler("type 2 state proof provider finalization updates", {
        finalizedBlockNotifier.updateFinalizedBlock(
          BlockNumberAndHash(it.blockNumber, it.blockHash.toArray())
        )
      })

      return l1FinalizationMonitor
    }
  }
}

private object DisabledLongRunningService : LongRunningService {
  override fun start(): CompletableFuture<Unit> {
    return SafeFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.completedFuture(Unit)
  }
}
