package net.consensys.zkevm.coordinator.app.conflationbacktesting

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import io.vertx.core.Vertx
import linea.LongRunningService
import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.TracesConfig.ClientApiConfig
import linea.domain.BlockInterval
import linea.domain.BlockParameter
import linea.encoding.BlockRLPEncoder
import linea.ethapi.EthApiClient
import linea.kotlin.decodeHex
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.app.conflation.ConflationAppHelper
import net.consensys.zkevm.coordinator.app.conflation.TracesClientFactory
import net.consensys.zkevm.coordinator.blockcreation.BlockCreationMonitor
import net.consensys.zkevm.coordinator.blockcreation.LastProvenBlockNumberProviderAsync
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.prover.ProverClientFactory
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressionProofCoordinator
import net.consensys.zkevm.ethereum.coordination.blob.BlobShnarfMetaData
import net.consensys.zkevm.ethereum.coordination.blob.BlobZkStateProviderImpl
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobCompressor
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blob.ParentBlobDataProvider
import net.consensys.zkevm.ethereum.coordination.blob.RollingBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.BlockToBatchSubmissionCoordinator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByDataCompressed
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationService
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationServiceImpl
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlobAwareConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlockConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ProofGeneratingConflationHandlerImpl
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCoordinatorImpl
import net.consensys.zkevm.ethereum.coordination.proofcreation.ZkProofCreationCoordinatorImpl
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import kotlin.concurrent.atomics.AtomicBoolean
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@OptIn(ExperimentalAtomicApi::class)
class ConflationBacktestingApp(
  val vertx: Vertx,
  val conflationBacktestingAppConfig: ConflationBacktestingConfig,
  mainCoordinatorConfig: CoordinatorConfig,
  httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  val metricsFacade: MetricsFacade,
) : LongRunningService {

  init {
    require(mainCoordinatorConfig.conflation.backtestingDirectory != null) {
      "Backtesting requests parent directory must be set in conflation config"
    }
    mainCoordinatorConfig.traces.common?.endpoints?.contains(conflationBacktestingAppConfig.tracesApi.endpoint)
      ?.let { require(!it) { "Cannot use same traces endpoint for backtesting and main conflation" } }
    mainCoordinatorConfig.traces.counters?.endpoints?.contains(conflationBacktestingAppConfig.tracesApi.endpoint)
      ?.let { require(!it) { "Cannot use same traces endpoint for backtesting and main conflation" } }
    mainCoordinatorConfig.traces.conflation?.endpoints?.contains(conflationBacktestingAppConfig.tracesApi.endpoint)
      ?.let { require(!it) { "Cannot use same traces endpoint for backtesting and main conflation" } }
  }

  private val log = LogManager.getLogger("conflation_backtesting.job_${conflationBacktestingAppConfig.jobId()}")

  private val conflationBacktestingComplete = AtomicBoolean(false)

  fun isConflationBacktestingComplete(): Boolean = conflationBacktestingComplete.load()

  fun onConflationProgress(
    proofIndex: CompressionProofIndex,
  ): SafeFuture<*> {
    return if (proofIndex.endBlockNumber == conflationBacktestingAppConfig.endBlockNumber) {
      conflationBacktestingComplete.store(true)
      log.info("Conflation backtesting complete")
      this.stop()
    } else {
      log.info(
        "Conflation backtesting progress: processed till blockNumber={}, targetEndBlock={}",
        proofIndex.endBlockNumber,
        conflationBacktestingAppConfig.endBlockNumber,
      )
      SafeFuture.completedFuture(Unit)
    }
  }

  val backtestingCoordinatorConfig: CoordinatorConfig = mainCoordinatorConfig.copy(
    conflation = mainCoordinatorConfig.conflation.copy(
      l2FetchBlocksLimit = UInt.MAX_VALUE,
      blocksLimit = conflationBacktestingAppConfig.batchesFixedSize,
      forceStopConflationAtBlockInclusive = conflationBacktestingAppConfig.endBlockNumber,
      conflationDeadline = null,
      blobCompression = mainCoordinatorConfig.conflation.blobCompression.copy(
        blobCompressorVersion = conflationBacktestingAppConfig.blobCompressorVersion,
      ),
      proofAggregation = mainCoordinatorConfig.conflation.proofAggregation.copy(
        targetEndBlocks = (mainCoordinatorConfig.conflation.proofAggregation.targetEndBlocks ?: emptyList())
          .toMutableList().also { it.add(conflationBacktestingAppConfig.endBlockNumber) },
      ),
    ),
    proversConfig = mainCoordinatorConfig.proversConfig.copy(
      proverA = getUpdatedProverConfig(
        proverConfig = mainCoordinatorConfig.proversConfig.proverA,
        backtestingDirectory = mainCoordinatorConfig.conflation.backtestingDirectory!!,
        conflationBacktestingJobId = conflationBacktestingAppConfig.jobId(),
      ),
      proverB = mainCoordinatorConfig.proversConfig.proverB?.let { proverB ->
        getUpdatedProverConfig(
          proverConfig = proverB,
          backtestingDirectory = mainCoordinatorConfig.conflation.backtestingDirectory,
          conflationBacktestingJobId = conflationBacktestingAppConfig.jobId(),
        )
      },
    ),
    traces = mainCoordinatorConfig.traces.copy(
      expectedTracesApiVersion = conflationBacktestingAppConfig.tracesApi.version,
      common = ClientApiConfig(
        endpoints = listOf(conflationBacktestingAppConfig.tracesApi.endpoint),
        requestLimitPerEndpoint = conflationBacktestingAppConfig.tracesApi.requestLimitPerEndpoint,
      ),
      counters = mainCoordinatorConfig.traces.counters?.copy(
        endpoints = listOf(conflationBacktestingAppConfig.tracesApi.endpoint),
        requestLimitPerEndpoint = conflationBacktestingAppConfig.tracesApi.requestLimitPerEndpoint,
      ),
      conflation = mainCoordinatorConfig.traces.conflation?.copy(
        endpoints = listOf(conflationBacktestingAppConfig.tracesApi.endpoint),
        requestLimitPerEndpoint = conflationBacktestingAppConfig.tracesApi.requestLimitPerEndpoint,
      ),
    ),
    stateManager = mainCoordinatorConfig.stateManager.copy(
      endpoints = listOf(conflationBacktestingAppConfig.shomeiApi.endpoint),
      requestLimitPerEndpoint = conflationBacktestingAppConfig.shomeiApi.requestLimitPerEndpoint,
      version = conflationBacktestingAppConfig.shomeiApi.version,
    ),
  ).also {
    log.info("Conflation backtesting coordinatorConfig={}", it)
  }

  val l2EthClient: EthApiClient = createEthApiClient(
    rpcUrl = backtestingCoordinatorConfig.conflation.l2Endpoint.toString(),
    log = log,
    requestRetryConfig = backtestingCoordinatorConfig.conflation.l2RequestRetries,
    vertx = vertx,
  )

  private val lastProcessedBlockNumber = conflationBacktestingAppConfig.startBlockNumber - 1uL
  private val lastProcessedBlock = l2EthClient.ethGetBlockByNumberTxHashes(
    BlockParameter.fromNumber(lastProcessedBlockNumber),
  ).get()
  private val lastProcessedTimestamp = Instant.fromEpochSeconds(lastProcessedBlock.timestamp.toLong())

  private val conflationCalculator: TracesConflationCalculator = run {
    // To fail faster for JNA reasons
    val blobCompressor = GoBackedBlobCompressor.getInstance(
      compressorVersion = backtestingCoordinatorConfig.conflation.blobCompression.blobCompressorVersion,
      dataLimit = backtestingCoordinatorConfig.conflation.blobCompression.blobSizeLimit,
      metricsFacade = metricsFacade,
    )

    val compressedBlobCalculator = ConflationCalculatorByDataCompressed(
      blobCompressor = blobCompressor,
    )

    val globalCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastProcessedBlockNumber,
      syncCalculators = ConflationAppHelper.createCalculatorsForBlobsAndConflation(
        configs = backtestingCoordinatorConfig,
        compressedBlobCalculator = compressedBlobCalculator,
        lastProcessedTimestamp = lastProcessedTimestamp,
        logger = log,
        metricsFacade = metricsFacade,
      ),
      deferredTriggerConflationCalculators = emptyList(),
      emptyTracesCounters = backtestingCoordinatorConfig.conflation.tracesLimits.emptyTracesCounters,
      log = log,
    )

    val batchesLimit = backtestingCoordinatorConfig.conflation.blobCompression.batchesLimit
      ?: (backtestingCoordinatorConfig.conflation.proofAggregation.proofsLimit - 1U)

    GlobalBlobAwareConflationCalculator(
      conflationCalculator = globalCalculator,
      blobCalculator = compressedBlobCalculator,
      metricsFacade = metricsFacade,
      batchesLimit = batchesLimit,
      log = log,
    )
  }

  private val conflationService: ConflationService =
    ConflationServiceImpl(
      calculator = conflationCalculator,
      metricsFacade = metricsFacade,
      log = log,
    )

  private val proverClientFactory = ProverClientFactory(
    vertx = vertx,
    config = backtestingCoordinatorConfig.proversConfig,
    metricsFacade = metricsFacade,
  )

  private val tracesClients = TracesClientFactory.createTracesClients(
    vertx = vertx,
    rpcClientFactory = httpJsonRpcClientFactory,
    configs = backtestingCoordinatorConfig.traces,
    fallBackTracesCounters = backtestingCoordinatorConfig.conflation.tracesLimits.emptyTracesCounters,
    log = log,
  )

  private val zkStateClient: StateManagerClientV1 = StateManagerV1JsonRpcClient.create(
    rpcClientFactory = httpJsonRpcClientFactory,
    endpoints = backtestingCoordinatorConfig.stateManager.endpoints.map { it.toURI() },
    zkStateManagerVersion = backtestingCoordinatorConfig.stateManager.version,
    maxInflightRequestsPerClient = backtestingCoordinatorConfig.stateManager.requestLimitPerEndpoint,
    requestRetry = backtestingCoordinatorConfig.stateManager.requestRetries.toJsonRpcRetry(),
    requestTimeout = backtestingCoordinatorConfig.stateManager.requestTimeout?.inWholeMilliseconds,
    logger = log,
  )

  val proofGeneratingConflationHandlerImpl = run {
    val executionProverClient: ExecutionProverClientV2 = proverClientFactory.executionProverClient(
      // we cannot use configs.traces.expectedTracesApiVersion because it breaks prover expected version pattern
      tracesVersion = "2.1.0",
      stateManagerVersion = backtestingCoordinatorConfig.stateManager.version,
      log = log,
    )

    ProofGeneratingConflationHandlerImpl(
      tracesProductionCoordinator = TracesConflationCoordinatorImpl(
        tracesConflationClient = tracesClients.tracesConflationClient,
        zkStateClient = zkStateClient,
        log = log,
      ),
      zkProofProductionCoordinator = ZkProofCreationCoordinatorImpl(
        executionProverClient = executionProverClient,
        messageServiceAddress = backtestingCoordinatorConfig.protocol.l2.contractAddress,
        l2EthApiClient = createEthApiClient(
          rpcUrl = backtestingCoordinatorConfig.conflation.l2Endpoint.toString(),
          log = log,
          requestRetryConfig = backtestingCoordinatorConfig.conflation.l2RequestRetries,
          vertx = vertx,
        ),
        log = log,
      ),
      batchProofHandler = { _ -> SafeFuture.completedFuture(Unit) },
      vertx = vertx,
      config = ProofGeneratingConflationHandlerImpl.Config(
        conflationAndProofGenerationRetryBackoffDelay = 5.seconds,
        executionProofPollingInterval = 100.seconds,
      ),
      log = log,
      metricsFacade = metricsFacade,
    )
  }

  private val blobCompressionProofCoordinator = run {
    val parentBlobDataProvider = object : ParentBlobDataProvider {
      override fun getParentBlobShnarfMetaData(
        currentBlobRange: BlockInterval,
      ): SafeFuture<BlobShnarfMetaData> {
        return SafeFuture.completedFuture(
          BlobShnarfMetaData(
            startBlockNumber = conflationBacktestingAppConfig.startBlockNumber - 1uL,
            endBlockNumber = conflationBacktestingAppConfig.startBlockNumber - 1uL,
            blobHash = ByteArray(32),
            blobShnarf = conflationBacktestingAppConfig.parentBlobShnarf?.decodeHex()
              ?: backtestingCoordinatorConfig.protocol.genesis.genesisShnarf,
          ),
        )
      }
    }

    val blobCompressionProofCoordinator = BlobCompressionProofCoordinator(
      vertx = vertx,
      blobCompressionProverClient = proverClientFactory.blobCompressionProverClient(log = log),
      rollingBlobShnarfCalculator = RollingBlobShnarfCalculator(
        blobShnarfCalculator = GoBackedBlobShnarfCalculator(
          version = backtestingCoordinatorConfig.conflation.blobCompression.shnarfCalculatorVersion,
          metricsFacade = metricsFacade,
        ),
        parentBlobDataProvider = parentBlobDataProvider,
        genesisShnarf = backtestingCoordinatorConfig.protocol.genesis.genesisShnarf,
      ),
      blobZkStateProvider = BlobZkStateProviderImpl(
        zkStateClient = zkStateClient,
      ),
      config = BlobCompressionProofCoordinator.Config(
        pollingInterval = backtestingCoordinatorConfig.conflation.blobCompression.handlerPollingInterval,
      ),
      blobCompressionProofHandler = {
        SafeFuture.completedFuture(Unit)
      },
      blobCompressionProofRequestHandler = { proofIndex, _ ->
        onConflationProgress(proofIndex)
      },
      log = log,
      metricsFacade = metricsFacade,
    )
    blobCompressionProofCoordinator
  }

  init {
    conflationService.onConflatedBatch(proofGeneratingConflationHandlerImpl)
    conflationCalculator.onBlobCreation(blobCompressionProofCoordinator::handleBlob)
  }

  private val blockToBatchSubmissionCoordinator = BlockToBatchSubmissionCoordinator(
    conflationService = conflationService,
    tracesCountersClient = tracesClients.tracesCountersClient,
    vertx = vertx,
    encoder = BlockRLPEncoder,
    log = log,
  )

  private val blockCreationMonitor = BlockCreationMonitor(
    vertx = vertx,
    ethApi = l2EthClient,
    startingBlockNumberExclusive = conflationBacktestingAppConfig.startBlockNumber.toLong() - 1,
    blockCreationListener = blockToBatchSubmissionCoordinator,
    lastProvenBlockNumberProviderAsync = object : LastProvenBlockNumberProviderAsync {
      override fun getLastProvenBlockNumber(): SafeFuture<Long> {
        return SafeFuture.completedFuture(conflationBacktestingAppConfig.startBlockNumber.toLong() - 1)
      }
    },
    config = BlockCreationMonitor.Config(
      pollingInterval = backtestingCoordinatorConfig.conflation.blocksPollingInterval,
      blocksToFinalization = 0L,
      blocksFetchLimit = backtestingCoordinatorConfig.conflation.l2FetchBlocksLimit.toLong(),
      // We need to add 1 to forceStopConflationAtBlockInclusive because conflation calculator requires
      // block_number = forceStopConflationAtBlockInclusive + 1 to trigger conflation at
      // forceStopConflationAtBlockInclusive
      lastL2BlockNumberToProcessInclusive = conflationBacktestingAppConfig.endBlockNumber + 1uL,
    ),
    log = log,
  )

  override fun start(): SafeFuture<Unit> {
    return proofGeneratingConflationHandlerImpl.start()
      .thenCompose { blobCompressionProofCoordinator.start() }
      .thenCompose { blockCreationMonitor.start() }
      .thenPeek {
        log.info("Conflation backtesting started successfully")
      }
  }

  override fun stop(): SafeFuture<Unit> {
    return SafeFuture.allOf(
      proofGeneratingConflationHandlerImpl.stop(),
      blobCompressionProofCoordinator.stop(),
      blockCreationMonitor.stop(),
    ).thenApply {
      log.info("Conflation backtesting stopped successfully")
    }
  }

  companion object {
    fun getUpdatedProverConfig(
      proverConfig: ProverConfig,
      backtestingDirectory: Path,
      conflationBacktestingJobId: String,
    ): ProverConfig {
      val jobDirectory = backtestingDirectory.resolve(conflationBacktestingJobId)
      return proverConfig.copy(
        execution = proverConfig.execution.copy(
          requestsDirectory = jobDirectory
            .resolve("execution")
            .resolve("requests"),
          responsesDirectory = jobDirectory
            .resolve("execution")
            .resolve("responses"),
        ),
        blobCompression = proverConfig.blobCompression.copy(
          requestsDirectory = jobDirectory
            .resolve("compression")
            .resolve("requests"),
          responsesDirectory = jobDirectory
            .resolve("compression")
            .resolve("responses"),
        ),
        proofAggregation = proverConfig.proofAggregation.copy(
          requestsDirectory = jobDirectory
            .resolve("aggregation")
            .resolve("requests"),
          responsesDirectory = jobDirectory
            .resolve("aggregation")
            .resolve("responses"),
        ),
      )
    }
  }
}
