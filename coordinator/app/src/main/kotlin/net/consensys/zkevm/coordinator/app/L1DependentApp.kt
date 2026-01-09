package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import io.vertx.sqlclient.SqlClient
import kotlinx.datetime.Clock
import linea.LongRunningService
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.Type2StateProofManagerConfig
import linea.coordinator.config.v2.isDisabled
import linea.coordinator.config.v2.isEnabled
import linea.domain.BlockNumberAndHash
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import linea.kotlin.toKWeiUInt
import linea.web3j.SmartContractErrors
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
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
import net.consensys.linea.ethereum.gaspricing.staticcap.L2CalldataBasedVariableFeesCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.L2CalldataSizeAccumulatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.app.conflation.ConflationApp
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper.resumeConflationFrom
import net.consensys.zkevm.coordinator.clients.ShomeiClient
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaSmartContractClient
import net.consensys.zkevm.domain.BlobSubmittedEvent
import net.consensys.zkevm.domain.FinalizationSubmittedEvent
import net.consensys.zkevm.ethereum.coordination.EventDispatcher
import net.consensys.zkevm.ethereum.coordination.HighestULongTracker
import net.consensys.zkevm.ethereum.coordination.LatestBlobSubmittedBlockNumberTracker
import net.consensys.zkevm.ethereum.coordination.LatestFinalizationSubmittedBlockNumberTracker
import net.consensys.zkevm.ethereum.coordination.blockcreation.ForkChoiceUpdaterImpl
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
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesPostgresDao
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesRepositoryImpl
import org.apache.logging.log4j.LogManager
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
    ).ethChainId().get()
  }

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
    val l1EthApiClient = createEthApiClient(
      rpcUrl = configs.l1Submission!!.dynamicGasPriceCap.feeHistoryFetcher.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.fees-fetcher"),
    )

    FeeHistoryFetcherImpl(
      ethApiClient = l1EthApiClient,
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
      l2EthApiClient = createEthApiClient(
        rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
        log = LogManager.getLogger("clients.l2.eth.finalization-monitor"),
      ),
      vertx = vertx,
    )
  }

  private val l1FinalizationHandlerForShomeiRpc: LongRunningService = run {
    val l2EthApiClient: EthApiClient = createEthApiClient(
      rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
      log = LogManager.getLogger("clients.l2.eth.shomei-frontend"),
    )
    setupL1FinalizationMonitorForShomeiFrontend(
      type2StateProofProviderConfig = configs.type2StateProofProvider,
      httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      lineaRollupClient = lineaRollupClientForFinalizationMonitor,
      l2EthApiClient = l2EthApiClient,
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

      val l2EthApiClient: EthApiClient =
        createEthApiClient(
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
        l2EthApiBlockClient = l2EthApiClient,
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
        maxFeePerBlobGasCap = configs.l1Submission.blob.gas.maxFeePerBlobGasCap,
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

  private val lineaSmartContractClientForDataSubmission: LineaSmartContractClient = run {
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
        maxFeePerBlobGasCap = configs.l1Submission.blob.gas.maxFeePerBlobGasCap,
      ),
    )
    val l1Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.l1Submission.blob.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.data-submission"),
    )
    val transactionManager = createTransactionManager(
      vertx,
      signerConfig = configs.l1Submission.blob.signer,
      client = l1Web3jClient,
    )
    createLineaContractClient(
      dataAvailabilityType = configs.l1Submission.dataAvailability,
      contractAddress = configs.protocol.l1.contractAddress,
      transactionManager = transactionManager,
      contractGasProvider = primaryOrFallbackGasProvider,
      web3jClient = l1Web3jClient,
      smartContractErrors = smartContractErrors,
      // eth_estimateGas would fail because we submit multiple blob tx
      // and 2nd would fail with revert reason
      useEthEstimateGas = false,
    )
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
      lineaSmartContractClientReadOnly = lineaSmartContractClientForDataSubmission,
      acceptedBlobEndBlockNumberConsumer = { highestAcceptedBlobTracker(it) },
    )

  private val blobSubmissionCoordinator = run {
    if (configs.l1Submission.isDisabled() || configs.l1Submission!!.blob.isDisabled()) {
      DisabledLongRunningService
    } else {
      val latestBlobSubmittedBlockNumberTracker = LatestBlobSubmittedBlockNumberTracker(0UL)
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

  private val aggregationFinalizationCoordinator = run {
    if (configs.l1Submission.isDisabled() || configs.l1Submission?.aggregation.isDisabled()) {
      DisabledLongRunningService
    } else {
      configs.l1Submission!!

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
      val lineaSmartContractClientForFinalization = createLineaContractClient(
        dataAvailabilityType = configs.l1Submission.dataAvailability,
        contractAddress = configs.protocol.l1.contractAddress,
        transactionManager = finalizationTransactionManager,
        contractGasProvider = primaryOrFallbackGasProvider,
        web3jClient = createWeb3jHttpClient(
          rpcUrl = configs.l1FinalizationMonitor.l1Endpoint.toString(),
          log = LogManager.getLogger("clients.l1.eth.finalization"),
        ),
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
        lineaSmartContractClient = lineaSmartContractClientForFinalization,
        alreadySubmittedBlobFilter = alreadySubmittedBlobsFilter,
        aggregationSubmitter = AggregationSubmitterImpl(
          lineaSmartContractClient = lineaSmartContractClientForFinalization,
          gasPriceCapProvider = gasPriceCapProviderForFinalization,
          aggregationSubmittedEventConsumer = EventDispatcher(submittedFinalizationConsumers),
        ),
        vertx = vertx,
        clock = Clock.System,
      )
    }
  }

  private fun lastFinalizedBlock(): SafeFuture<ULong> {
    val l1BasedLastFinalizedBlockProvider = L1BasedLastFinalizedBlockProvider(
      vertx,
      lineaRollupSmartContractClient = lineaRollupClientForFinalizationMonitor,
      consistentNumberOfBlocksOnL1 = configs.conflation.consistentNumberOfBlocksOnL1ToWait,
    )
    return l1BasedLastFinalizedBlockProvider.getLastFinalizedBlock()
  }

  private val messageAnchoringApp: LongRunningService = MessageAnchoringAppConfigurator.create(
    vertx = vertx,
    configs = configs,
  )

  private val conflationApp = ConflationApp(
    vertx = vertx,
    configs = configs,
    batchesRepository = batchesRepository,
    blobsRepository = blobsRepository,
    aggregationsRepository = aggregationsRepository,
    lastFinalizedBlock = lastFinalizedBlock,
    metricsFacade = metricsFacade,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
  )

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
        l2CalldataPricingCalculatorConfig = configs.l2NetworkGasPricing.dynamicGasPricing.calldataBasedPricing?.let {
          if (configs.l2NetworkGasPricing.dynamicGasPricing.calldataBasedPricing.calldataSumSizeBlockCount > 0U) {
            L2NetworkGasPricingService.L2CalldataPricingConfig(
              l2CalldataSizeAccumulatorConfig = L2CalldataSizeAccumulatorImpl.Config(
                blockSizeNonCalldataOverhead = it.blockSizeNonCalldataOverhead,
                calldataSizeBlockCount = it.calldataSumSizeBlockCount,
              ),
              l2CalldataBasedVariableFeesCalculatorConfig = L2CalldataBasedVariableFeesCalculator.Config(
                feeChangeDenominator = it.feeChangeDenominator,
                calldataSizeBlockCount = it.calldataSumSizeBlockCount,
                maxBlockCalldataSize = it.calldataSumSizeTarget.toUInt(),
              ),
            )
          } else {
            log.debug("Calldata-based variable fee is disabled as calldataSizeBlockCount is set as 0")
            null
          }
        },
      )
      val l2EthApiClient = createEthApiClient(
        rpcUrl = configs.l2NetworkGasPricing.l2Endpoint.toString(),
        log = LogManager.getLogger("clients.l2.eth.l2pricing"),
      )
      val l1EthApiClient = createEthApiClient(
        rpcUrl = configs.l2NetworkGasPricing.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.l1pricing"),
      )
      L2NetworkGasPricingService(
        vertx = vertx,
        metricsFacade = metricsFacade,
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        l1EthApiClient = l1EthApiClient,
        l2EthApiBlockClient = l2EthApiClient,
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

      val l1EthApiClient = createEthApiClient(
        rpcUrl = configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.feehistory-cache"),
      )

      val l1FeeHistoryFetcher: GasPriceCapFeeHistoryFetcher = GasPriceCapFeeHistoryFetcherImpl(
        ethApiFeeClient = l1EthApiClient,
        config = GasPriceCapFeeHistoryFetcherImpl.Config(
          maxBlockCount = configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.maxBlockCount,
          rewardPercentiles = configs.l1Submission.dynamicGasPriceCap.feeHistoryFetcher.rewardPercentiles
            .map { it.toDouble() },
        ),
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
        ethApiBlockClient = l1EthApiClient,
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
        conflationApp.updateLatestL1FinalizedBlock(update.blockNumber.toLong())
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
    return l1FinalizationMonitor.start()
      .thenCompose { l1FinalizationHandlerForShomeiRpc.start() }
      .thenCompose { blobSubmissionCoordinator.start() }
      .thenCompose { aggregationFinalizationCoordinator.start() }
      .thenCompose { messageAnchoringApp.start() }
      .thenCompose { conflationApp.start() }
      .thenCompose { l2NetworkGasPricingService?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { l1FeeHistoryCachingService.start() }
      .thenPeek {
        log.info("L1App started")
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      conflationApp.stop(),
      l1FinalizationMonitor.stop(),
      l1FinalizationHandlerForShomeiRpc.stop(),
      blobSubmissionCoordinator.stop(),
      aggregationFinalizationCoordinator.stop(),
      messageAnchoringApp.stop(),
      l2NetworkGasPricingService?.stop() ?: SafeFuture.completedFuture(Unit),
      l1FeeHistoryCachingService.stop(),
    )
      .thenApply { log.info("L1App Stopped") }
  }

  companion object {
    fun setupL1FinalizationMonitorForShomeiFrontend(
      type2StateProofProviderConfig: Type2StateProofManagerConfig,
      httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
      lineaRollupClient: LineaRollupSmartContractClientReadOnly,
      l2EthApiClient: EthApiClient,
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
          l2EthApiClient = l2EthApiClient,
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
