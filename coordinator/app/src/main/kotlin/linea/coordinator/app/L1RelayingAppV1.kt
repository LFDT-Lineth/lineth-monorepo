package linea.coordinator.app

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.contract.l1.LineaSmartContractClient
import linea.coordination.EventDispatcher
import linea.coordination.HighestULongTracker
import linea.coordination.LatestBlobSubmittedBlockNumberTracker
import linea.coordination.LatestFinalizationSubmittedBlockNumberTracker
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.L1SubmissionConfig
import linea.coordinator.config.v2.isDisabled
import linea.coordinator.config.v2.isEnabled
import linea.domain.BlobSubmittedEvent
import linea.domain.FinalizationSubmittedEvent
import linea.ethapi.EthApiClient
import linea.finalization.AggregationFinalizationCoordinator
import linea.finalization.AggregationSubmitterImpl
import linea.metrics.LineaMetricsCategory
import linea.persistence.AggregationsRepository
import linea.persistence.BlobsRepository
import linea.persistence.FeeHistoriesDao
import linea.submission.BlobSubmissionCoordinator
import linea.submission.L1ShnarfBasedAlreadySubmittedBlobsFilter
import linea.web3j.SmartContractErrors
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.WMAFeesCalculator
import net.consensys.linea.ethereum.gaspricing.dynamiccap.FeeHistoriesRepositoryImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.FeeHistoryCachingService
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapCalculatorImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapFeeHistoryFetcher
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapFeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderForDataSubmission
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderForFinalization
import net.consensys.linea.ethereum.gaspricing.dynamiccap.GasPriceCapProviderImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import java.util.function.Consumer
import kotlin.time.Clock

/**
 * Responsibilities:
 *   - poll the database for blobs and aggregations ready for submission to L1
 *   - submit blobs
 *   - submit aggregations/finalizations
 */
class L1RelayingAppV1(
  private val configs: CoordinatorConfig,
  // Note/Todo: l1SubmissionConfig should be the only necessary one, however requires deeper refactor
  private val l1SubmissionConfig: L1SubmissionConfig = configs.l1Submission!!,
  private val vertx: Vertx,
  private val l1ChainId: ULong,
  private val lastFinalizedBlock: ULong,
  private val smartContractErrors: SmartContractErrors,
  private val metricsFacade: MetricsFacade,
  private val feeHistoriesDao: FeeHistoriesDao,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
  private val l1EthApiClient: EthApiClient = createEthApiClient(
    rpcUrl = l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.l1Endpoint.toString(),
    log = LogManager.getLogger("clients.l1.eth.fees-fetcher"),
    vertx = vertx,
    requestRetryConfig = l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.l1RequestRetries,
  ),
  private val feesFetcher: FeesFetcher = run {
    FeeHistoryFetcherImpl(
      ethApiClient = l1EthApiClient,
      config = FeeHistoryFetcherImpl.Config(
        feeHistoryBlockCount = l1SubmissionConfig.fallbackGasPrice.feeHistoryBlockCount,
        feeHistoryRewardPercentile = l1SubmissionConfig.fallbackGasPrice.feeHistoryRewardPercentile.toDouble(),
      ),
    )
  },
  private val l1MinPriorityFeeCalculator: FeesCalculator = WMAFeesCalculator(
    WMAFeesCalculator.Config(
      baseFeeCoefficient = 0.0,
      priorityFeeWmaCoefficient = 1.0,
    ),
  ),
  private val lineaSmartContractClientForDataSubmission: LineaSmartContractClient = run {
    val l1Web3jClient = createWeb3jHttpClient(
      rpcUrl = l1SubmissionConfig.blob.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.data-submission"),
    )
    createLineaContractClient(
      vertx = vertx,
      contractAddress = configs.protocol.l1.contractAddress,
      smartContractErrors = smartContractErrors,
      dataAvailabilityType = l1SubmissionConfig.dataAvailability,
      l1Web3jClient = l1Web3jClient,
      l1ChainId = l1ChainId,
      l1MinPriorityFeeCalculator = l1MinPriorityFeeCalculator,
      feesFetcher = feesFetcher,
      signerConfig = l1SubmissionConfig.blob.signer,
      gasConfig = l1SubmissionConfig.blob.gas,
      useEthEstimateGas = false,
    )
  },
  private val lineaSmartContractClientForFinalization: LineaSmartContractClient = run {
    val l1Web3jClient = createWeb3jHttpClient(
      rpcUrl = l1SubmissionConfig.aggregation.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.finalization"),
    )
    createLineaContractClient(
      vertx = vertx,
      contractAddress = configs.protocol.l1.contractAddress,
      smartContractErrors = smartContractErrors,
      dataAvailabilityType = l1SubmissionConfig.dataAvailability,
      l1Web3jClient = l1Web3jClient,
      l1ChainId = l1ChainId,
      l1MinPriorityFeeCalculator = l1MinPriorityFeeCalculator,
      feesFetcher = feesFetcher,
      signerConfig = l1SubmissionConfig.aggregation.signer,
      gasConfig = l1SubmissionConfig.aggregation.gas,
      useEthEstimateGas = true,
    )
  },
  private val log: Logger = LogManager.getLogger(L1RelayingAppV1::class.java),
) : LongRunningService {
  init {
    if (l1SubmissionConfig.blob.isDisabled()) {
      log.warn("L1 submission disabled for blobs")
    }
    if (l1SubmissionConfig.aggregation.isDisabled()) {
      log.warn("L1 submission disabled for aggregations")
    }
  }

  // Metrics Setup
  val highestAcceptedBlobTracker = HighestULongTracker(lastFinalizedBlock)
  val latestBlobSubmittedBlockNumberTracker = LatestBlobSubmittedBlockNumberTracker(lastFinalizedBlock)
  val latestFinalizationSubmittedBlockNumberTracker = LatestFinalizationSubmittedBlockNumberTracker(lastFinalizedBlock)
  val blobSubmissionDelayHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BLOB,
    name = "submission.delay",
    description = "Delay between blob submission and end block timestamps",
    baseUnit = "seconds",
  )
  val finalizationSubmissionDelayHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.AGGREGATION,
    name = "submission.delay",
    description = "Delay between finalization submission and end block timestamps",
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
  val submittedFinalizationConsumers: Map<Consumer<FinalizationSubmittedEvent>, String> = mapOf(
    Consumer<FinalizationSubmittedEvent> { finalizationSubmission ->
      latestFinalizationSubmittedBlockNumberTracker(finalizationSubmission)
    } to "Finalization Submission Consumer",
    Consumer<FinalizationSubmittedEvent> { finalizationSubmission ->
      finalizationSubmissionDelayHistogram.record(finalizationSubmission.getSubmissionDelay().toDouble())
    } to "Finalization Submission Delay Consumer",
  )

  open fun init() {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "highest.submitted.on.l1",
      description = "Highest submitted finalization end block number on l1",
      measurementSupplier = latestFinalizationSubmittedBlockNumberTracker::get,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "highest.accepted.block.number",
      description = "Highest accepted blob end block number",
      measurementSupplier = highestAcceptedBlobTracker::get,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "highest.submitted.on.l1",
      description = "Highest submitted blob end block number on l1",
      measurementSupplier = latestBlobSubmittedBlockNumberTracker::get,
    )
  }

  private val alreadySubmittedBlobsFilter =
    L1ShnarfBasedAlreadySubmittedBlobsFilter(
      lineaSmartContractClientReadOnly = lineaSmartContractClientForDataSubmission,
      acceptedBlobEndBlockNumberConsumer = { highestAcceptedBlobTracker(it) },
    )

  private val l1FeeHistoriesRepository =
    FeeHistoriesRepositoryImpl(
      FeeHistoriesRepositoryImpl.Config(
        rewardPercentiles = l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher
          .rewardPercentiles.map { it.toDouble() },
        minBaseFeePerBlobGasToCache = l1SubmissionConfig.dynamicGasPriceCap
          .gasPriceCapCalculation.historicBaseFeePerBlobGasLowerBound,
        fixedAverageRewardToCache = l1SubmissionConfig.dynamicGasPriceCap
          .gasPriceCapCalculation.historicAvgRewardConstant,
      ),
      feeHistoriesDao = this.feeHistoriesDao,
    )

  private val gasPriceCapProvider =
    if (l1SubmissionConfig.isEnabled() && l1SubmissionConfig.dynamicGasPriceCap.isEnabled()) {
      val feeHistoryPercentileWindowInBlocks =
        l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentileWindow
          .div(configs.protocol.l1.blockTime).toUInt()

      val feeHistoryPercentileWindowLeewayInBlocks =
        l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentileWindowLeeway
          .div(configs.protocol.l1.blockTime).toUInt()

      val l2EthApiClient: EthApiClient =
        createEthApiClient(
          rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
          log = LogManager.getLogger("clients.l2.eth.gascap-provider"),
          vertx = vertx,
          requestRetryConfig = configs.l1FinalizationMonitor.l2RequestRetries,
        )

      GasPriceCapProviderImpl(
        config = GasPriceCapProviderImpl.Config(
          enabled = l1SubmissionConfig.dynamicGasPriceCap.isEnabled(),
          gasFeePercentile =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentile.toDouble(),
          gasFeePercentileWindowInBlocks = feeHistoryPercentileWindowInBlocks,
          gasFeePercentileWindowLeewayInBlocks = feeHistoryPercentileWindowLeewayInBlocks,
          timeOfDayMultipliers =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.timeOfTheDayMultipliers,
          adjustmentConstant =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.adjustmentConstant,
          blobAdjustmentConstant =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.blobAdjustmentConstant,
          finalizationTargetMaxDelay =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.finalizationTargetMaxDelay,
          gasPriceCapsCoefficient =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.gasPriceCapsCheckCoefficient,
        ),
        l2EthApiBlockClient = l2EthApiClient,
        feeHistoriesRepository = l1FeeHistoriesRepository,
        gasPriceCapCalculator = GasPriceCapCalculatorImpl(),
      )
    } else {
      null
    }

  open val gasPriceCapProviderForDataSubmission =
    gasPriceCapProvider?.let { provider ->
      GasPriceCapProviderForDataSubmission(
        config = GasPriceCapProviderForDataSubmission.Config(
          maxPriorityFeePerGasCap = l1SubmissionConfig.blob.gas.maxPriorityFeePerGasCap,
          maxFeePerGasCap = l1SubmissionConfig.blob.gas.maxFeePerGasCap,
          maxFeePerBlobGasCap = l1SubmissionConfig.blob.gas.maxFeePerBlobGasCap,
        ),
        gasPriceCapProvider = provider,
        metricsFacade = metricsFacade,
      )
    }

  open val gasPriceCapProviderForFinalization =
    gasPriceCapProvider?.let { provider ->
      GasPriceCapProviderForFinalization(
        config = GasPriceCapProviderForFinalization.Config(
          maxPriorityFeePerGasCap = l1SubmissionConfig.aggregation.gas.maxPriorityFeePerGasCap,
          maxFeePerGasCap = l1SubmissionConfig.aggregation.gas.maxFeePerGasCap,
        ),
        gasPriceCapProvider = provider,
        metricsFacade = metricsFacade,
      )
    }

  open val blobSubmissionCoordinator = run {
    if (l1SubmissionConfig.isDisabled() || l1SubmissionConfig.blob.isDisabled()) {
      DisabledLongRunningService
    } else {
      BlobSubmissionCoordinator.create(
        config = BlobSubmissionCoordinator.Config(
          pollingInterval = l1SubmissionConfig.blob.submissionTickInterval,
          proofSubmissionDelay = l1SubmissionConfig.blob.submissionDelay,
          maxBlobsToSubmitPerTick =
          l1SubmissionConfig.blob.maxSubmissionTransactionsPerTick *
            l1SubmissionConfig.blob.targetBlobsPerTransaction,
          targetBlobsToSubmitPerTx = l1SubmissionConfig.blob.targetBlobsPerTransaction,
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

  open val aggregationFinalizationCoordinator = run {
    if (l1SubmissionConfig.isDisabled() || l1SubmissionConfig.aggregation.isDisabled()) {
      DisabledLongRunningService
    } else {
      AggregationFinalizationCoordinator.create(
        config = AggregationFinalizationCoordinator.Config(
          pollingInterval = l1SubmissionConfig.aggregation.submissionTickInterval,
          proofSubmissionDelay = l1SubmissionConfig.aggregation.submissionDelay,
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

  private val l1FeeHistoryCachingService: LongRunningService =
    if (l1SubmissionConfig.isEnabled() && l1SubmissionConfig.dynamicGasPriceCap.isEnabled()) {
      val feeHistoryPercentileWindowInBlocks =
        l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentileWindow
          .div(configs.protocol.l1.blockTime).toUInt()
      val feeHistoryStoragePeriodInBlocks =
        l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.storagePeriod
          .div(configs.protocol.l1.blockTime).toUInt()

      val l1FeeHistoryFetcher: GasPriceCapFeeHistoryFetcher = GasPriceCapFeeHistoryFetcherImpl(
        ethApiFeeClient = l1EthApiClient,
        config = GasPriceCapFeeHistoryFetcherImpl.Config(
          maxBlockCount = l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.maxBlockCount,
          rewardPercentiles = l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.rewardPercentiles
            .map { it.toDouble() },
        ),
      )

      FeeHistoryCachingService(
        config = FeeHistoryCachingService.Config(
          pollingInterval =
          l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.fetchInterval,
          feeHistoryMaxBlockCount =
          l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.maxBlockCount,
          gasFeePercentile =
          l1SubmissionConfig.dynamicGasPriceCap.gasPriceCapCalculation.baseFeePerGasPercentile.toDouble(),
          feeHistoryStoragePeriodInBlocks = feeHistoryStoragePeriodInBlocks,
          feeHistoryWindowInBlocks = feeHistoryPercentileWindowInBlocks,
          numOfBlocksBeforeLatest =
          l1SubmissionConfig.dynamicGasPriceCap.feeHistoryFetcher.numOfBlocksBeforeLatest,
        ),
        vertx = vertx,
        ethApiBlockClient = l1EthApiClient,
        feeHistoryFetcher = l1FeeHistoryFetcher,
        feeHistoriesRepository = l1FeeHistoriesRepository,
      )
    } else {
      DisabledLongRunningService
    }

  override fun start(): CompletableFuture<Unit> {
    init()
    return SafeFuture.completedFuture(Unit)
      .thenCompose { blobSubmissionCoordinator.start() }
      .thenCompose { aggregationFinalizationCoordinator.start() }
      .thenCompose { l1FeeHistoryCachingService.start() }
      .thenPeek {
        log.info("L1RelayingApp started")
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      blobSubmissionCoordinator.stop(),
      aggregationFinalizationCoordinator.stop(),
      l1FeeHistoryCachingService.stop(),
    )
      .thenApply { log.info("L1RelayingApp Stopped") }
  }
}
