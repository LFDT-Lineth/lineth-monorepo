package net.consensys.zkevm.coordinator.app.conflation

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import io.vertx.core.Vertx
import kotlinx.datetime.Instant
import linea.LongRunningService
import linea.blob.ShnarfCalculatorVersion
import linea.contract.l2.Web3JL2MessageServiceSmartContractClient
import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.CoordinatorConfig
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.encoding.BlockRLPEncoder
import linea.ethapi.EthApiClient
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper.cleanupDbDataAfterBlockNumbers
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper.createCalculatorsForBlobsAndConflation
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper.createDeadlineConflationCalculatorRunner
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper.resumeAggregationFrom
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper.resumeConflationFrom
import net.consensys.zkevm.coordinator.app.conflation.TracesClientFactory.createTracesClients
import net.consensys.zkevm.coordinator.blockcreation.BatchesRepoBasedLastProvenBlockNumberProvider
import net.consensys.zkevm.coordinator.blockcreation.BlockCreationMonitor
import net.consensys.zkevm.coordinator.blockcreation.GethCliqueSafeBlockProvider
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.prover.ProverClientFactory
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.ethereum.coordination.HighestConflationTracker
import net.consensys.zkevm.ethereum.coordination.HighestProvenBatchTracker
import net.consensys.zkevm.ethereum.coordination.HighestProvenBlobTracker
import net.consensys.zkevm.ethereum.coordination.HighestULongTracker
import net.consensys.zkevm.ethereum.coordination.HighestUnprovenBlobTracker
import net.consensys.zkevm.ethereum.coordination.SimpleCompositeSafeFutureHandler
import net.consensys.zkevm.ethereum.coordination.aggregation.ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTracker
import net.consensys.zkevm.ethereum.coordination.aggregation.ProofAggregationCoordinatorService
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressionProofCoordinator
import net.consensys.zkevm.ethereum.coordination.blob.BlobZkStateProviderImpl
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobCompressor
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blob.ParentBlobDataProviderImpl
import net.consensys.zkevm.ethereum.coordination.blob.RollingBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.BlockToBatchSubmissionCoordinator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByDataCompressed
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationService
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationServiceImpl
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlobAwareConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlockConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ProofGeneratingConflationHandlerImpl
import net.consensys.zkevm.ethereum.coordination.conflation.TimestampHardForkConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCoordinatorImpl
import net.consensys.zkevm.ethereum.coordination.proofcreation.ZkProofCreationCoordinatorImpl
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import net.consensys.zkevm.persistence.dao.batch.persistence.BatchProofHandlerImpl
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class ConflationApp(
  val vertx: Vertx,
  val batchesRepository: BatchesRepository,
  val blobsRepository: BlobsRepository,
  val aggregationsRepository: AggregationsRepository,
  val lastFinalizedBlock: ULong,
  val configs: CoordinatorConfig,
  val metricsFacade: MetricsFacade,
  val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
) : LongRunningService {
  private val log = LogManager.getLogger("conflation.app")

  private val lastProcessedBlockNumber = resumeConflationFrom(
    aggregationsRepository,
    lastFinalizedBlock,
  ).get()
  private val lastConsecutiveAggregatedBlockNumber = resumeAggregationFrom(
    aggregationsRepository,
    lastFinalizedBlock,
  ).get()

  val l2EthClient: EthApiClient = createEthApiClient(
    rpcUrl = configs.conflation.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.conflation"),
    requestRetryConfig = configs.conflation.l2RequestRetries,
    vertx = vertx,
  )

  private val lastProcessedBlock = l2EthClient.ethGetBlockByNumberTxHashes(
    lastProcessedBlockNumber.toBlockParameter(),
  ).get()

  init {
    require(lastProcessedBlock != null) {
      "lastProcessedBlock=$lastProcessedBlock is null! Unable to instantiate conflation calculators!"
    }
  }

  private val lastProcessedTimestamp = Instant.fromEpochSeconds(lastProcessedBlock!!.timestamp.toLong())

  private val deadlineConflationCalculatorRunner = createDeadlineConflationCalculatorRunner(
    configs = configs,
    lastProcessedBlockNumber = lastProcessedBlockNumber,
    l2EthClient = l2EthClient,
  ).also {
    if (it == null) {
      log.info("Conflation deadline calculator is disabled")
    }
  }

  private val conflationCalculator: TracesConflationCalculator = run {
    val logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java)

    // To fail faster for JNA reasons
    val blobCompressor = GoBackedBlobCompressor.Companion.getInstance(
      compressorVersion = configs.conflation.blobCompression.blobCompressorVersion,
      dataLimit = configs.conflation.blobCompression.blobSizeLimit,
      metricsFacade = metricsFacade,
    )

    val compressedBlobCalculator = ConflationCalculatorByDataCompressed(
      blobCompressor = blobCompressor,
    )
    val syncCalculators = createCalculatorsForBlobsAndConflation(
      configs = configs,
      compressedBlobCalculator = compressedBlobCalculator,
      lastProcessedTimestamp = lastProcessedTimestamp,
      logger = logger,
      metricsFacade = metricsFacade,
    ).also {
      it.filterIsInstance<TimestampHardForkConflationCalculator>().forEach { calculator ->
        log.info(
          "Added timestamp-based hard fork calculator={} ",
          calculator,
        )
      }
    }

    val globalCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastProcessedBlockNumber,
      syncCalculators = syncCalculators,
      deferredTriggerConflationCalculators = listOfNotNull(deadlineConflationCalculatorRunner),
      emptyTracesCounters = configs.conflation.tracesLimits.emptyTracesCounters,
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

  private val zkStateClient: StateManagerClientV1 = StateManagerV1JsonRpcClient.Companion.create(
    rpcClientFactory = httpJsonRpcClientFactory,
    endpoints = configs.stateManager.endpoints.map { it.toURI() },
    maxInflightRequestsPerClient = configs.stateManager.requestLimitPerEndpoint,
    requestRetry = configs.stateManager.requestRetries.toJsonRpcRetry(),
    requestTimeout = configs.stateManager.requestTimeout?.inWholeMilliseconds,
    zkStateManagerVersion = configs.stateManager.version,
    logger = LogManager.getLogger("clients.StateManagerShomeiClient"),
  )

  private val proverClientFactory = ProverClientFactory(
    vertx = vertx,
    config = configs.proversConfig,
    metricsFacade = metricsFacade,
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
    val blobCompressionProofHandler: (BlobRecord) -> SafeFuture<*> = SimpleCompositeSafeFutureHandler(
      listOf(
        blobsRepository::saveNewBlob,
        maxProvenBlobCache,
      ),
    )

    val blobCompressionProofCoordinator = BlobCompressionProofCoordinator(
      vertx = vertx,
      blobCompressionProverClient = proverClientFactory.blobCompressionProverClient(),
      rollingBlobShnarfCalculator = RollingBlobShnarfCalculator(
        blobShnarfCalculator = GoBackedBlobShnarfCalculator(
          version = ShnarfCalculatorVersion.V1_2,
          metricsFacade = metricsFacade,
        ),
        parentBlobDataProvider = ParentBlobDataProviderImpl(blobsRepository),
        genesisShnarf = configs.protocol.genesis.genesisShnarf,
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

    val highestConsecutiveAggregationTracker = HighestULongTracker(lastConsecutiveAggregatedBlockNumber)
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "proven.highest.consecutive.block.number",
      description = "Highest consecutive proven aggregation block number",
      measurementSupplier = highestConsecutiveAggregationTracker,
    )
    ProofAggregationCoordinatorService.Companion
      .create(
        vertx = vertx,
        aggregationCoordinatorPollingInterval = configs.conflation.proofAggregation.coordinatorPollingInterval,
        deadlineCheckInterval = configs.conflation.proofAggregation.deadlineCheckInterval,
        aggregationDeadline = configs.conflation.proofAggregation.deadline,
        latestBlockProvider = GethCliqueSafeBlockProvider(
          ethApiBlockClient = l2EthClient,
          config = GethCliqueSafeBlockProvider.Config(0),
        ),
        maxProofsPerAggregation = configs.conflation.proofAggregation.proofsLimit,
        maxBlobsPerAggregation = configs.conflation.proofAggregation.blobsLimit,
        startBlockNumberInclusive = lastConsecutiveAggregatedBlockNumber + 1u,
        aggregationsRepository = aggregationsRepository,
        consecutiveProvenBlobsProvider = maxBlobEndBlockNumberTracker,
        proofAggregationClient = proverClientFactory.proofAggregationProverClient(),
        l2EthApiClient = l2EthClient,
        l2MessageService = Web3JL2MessageServiceSmartContractClient.createReadOnly(
          web3jClient = createWeb3jHttpClient(
            rpcUrl = configs.conflation.l2Endpoint.toString(),
            log = LogManager.getLogger("clients.l2.eth.conflation"),
          ),
          ethApiClient = l2EthClient,
          contractAddress = configs.protocol.l2.contractAddress,
          smartContractErrors = configs.smartContractErrors,
          smartContractDeploymentBlockNumber = configs.protocol.l2.contractDeploymentBlockNumber?.getNumber(),
        ),
        noL2ActivityTimeout = configs.conflation.conflationDeadlineLastBlockConfirmationDelay,
        waitForNoL2ActivityToTriggerAggregation =
        configs.conflation.proofAggregation.waitForNoL2ActivityToTriggerAggregation,
        targetEndBlockNumbers = configs.conflation.proofAggregation.targetEndBlocks ?: emptyList(),
        metricsFacade = metricsFacade,
        provenAggregationEndBlockNumberConsumer = { aggEndBlockNumber -> highestAggregationTracker(aggEndBlockNumber) },
        provenConsecutiveAggregationEndBlockNumberConsumer =
        { aggEndBlockNumber -> highestConsecutiveAggregationTracker(aggEndBlockNumber) },
        lastFinalizedBlockNumberSupplier = { lastProvenBlockNumberProvider.getLatestL1FinalizedBlock().toULong() },
        aggregationSizeMultipleOf = configs.conflation.proofAggregation.aggregationSizeMultipleOf,
        hardForkTimestamps = configs.conflation.proofAggregation.timestampBasedHardForks,
        initialTimestamp = lastProcessedTimestamp,
      )
  }
  val tracesClients =
    createTracesClients(
      vertx = vertx,
      rpcClientFactory = httpJsonRpcClientFactory,
      configs = configs.traces,
      fallBackTracesCounters = configs.conflation.tracesLimits.emptyTracesCounters,
    )
  val proofGeneratingConflationHandlerImpl = run {
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
    ProofGeneratingConflationHandlerImpl(
      tracesProductionCoordinator = TracesConflationCoordinatorImpl(
        tracesClients.tracesConflationClient,
        zkStateClient,
      ),
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
      config = ProofGeneratingConflationHandlerImpl.Config(
        conflationAndProofGenerationRetryBackoffDelay = 5.seconds,
        executionProofPollingInterval = 100.milliseconds,
      ),
      metricsFacade = metricsFacade,
    )
  }

  private val block2BatchCoordinator = run {
    val blobsConflationHandler: (BlocksConflation) -> SafeFuture<*> = run {
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
      tracesCountersClient = tracesClients.tracesCountersClient,
      vertx = vertx,
      encoder = BlockRLPEncoder,
    )
  }

  private val lastProvenBlockNumberProvider = run {
    val lastProvenConsecutiveBatchBlockNumberProvider = BatchesRepoBasedLastProvenBlockNumberProvider(
      lastProcessedBlockNumber.toLong(),
      lastFinalizedBlock.toLong(),
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
      ethApi = l2EthClient,
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
        lastL2BlockTimestampToProcessInclusive = configs.conflation.forceStopConflationAtBlockTimestampInclusive,
      ),
    )
    blockCreationMonitor
  }

  override fun start(): CompletableFuture<Unit> {
    return cleanupDbDataAfterBlockNumbers(
      lastProcessedBlockNumber = lastProcessedBlockNumber,
      lastConsecutiveAggregatedBlockNumber = lastConsecutiveAggregatedBlockNumber,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository,
    )
      .thenCompose { proofGeneratingConflationHandlerImpl.start() }
      .thenCompose { proofAggregationCoordinatorService.start() }
      .thenCompose { deadlineConflationCalculatorRunner?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { blockCreationMonitor.start() }
      .thenCompose { blobCompressionProofCoordinator.start() }
      .thenPeek {
        log.info("Conflation started")
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      proofGeneratingConflationHandlerImpl.stop(),
      proofAggregationCoordinatorService.stop(),
      blockCreationMonitor.stop(),
      deadlineConflationCalculatorRunner?.stop() ?: SafeFuture.completedFuture(Unit),
      blobCompressionProofCoordinator.stop(),
    )
      .thenApply { log.info("Conflation Stopped") }
  }

  fun updateLatestL1FinalizedBlock(blockNumber: Long): SafeFuture<Unit> {
    return lastProvenBlockNumberProvider.updateLatestL1FinalizedBlock(blockNumber)
  }
}
