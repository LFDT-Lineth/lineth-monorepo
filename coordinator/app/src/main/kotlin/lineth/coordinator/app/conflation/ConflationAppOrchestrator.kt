package lineth.coordinator.app.conflation

import io.vertx.core.Vertx
import linea.DisabledService
import linea.LongRunningService
import linea.clients.StateManagerV1JsonRpcClient
import linea.contract.l1.Web3JLinethRollupSmartContractClientReadOnly
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.ethapi.EthLogsSearcherImpl
import linea.ftx.ForcedTransactionsApp
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import lineth.coordinator.blockcreation.BatchesRepoBasedLastProvenBlockNumberProvider
import lineth.coordinator.blockcreation.ConflationTargetCheckpointPauseController
import lineth.coordinator.clients.ForcedTransactionsJsonRpcClient
import lineth.coordinator.clients.prover.ProverClientFactory
import lineth.coordinator.config.toJsonRpcRetry
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.ftx.conflation.ForcedTransactionsInvalidityProofService
import lineth.ftx.conflation.InvalidityProofAssembler
import lineth.metrics.LineaMetricsCategory
import lineth.persistence.AggregationsRepository
import lineth.persistence.BatchesRepository
import lineth.persistence.BlobsRepository
import lineth.persistence.ForcedTransactionsDao
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Owns the lifecycle of ConflationAppV1, ConflationAppV2, and ForcedTransactionsApp.
 * Decides which conflation apps to instantiate based on configuration and coordinates
 * their start/stop sequencing.
 *
 * Future responsibility: resume block number resolution and DB cleanup on V1→V2 cutover.
 */
class ConflationAppOrchestrator(
  private val vertx: Vertx,
  private val clock: Clock,
  private val batchesRepository: BatchesRepository,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
  private val forcedTransactionsDao: ForcedTransactionsDao,
  private val lastFinalizedBlock: ULong,
  private val configs: CoordinatorConfig,
  private val metricsFacade: MetricsFacade,
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  private val proverClientFactory: ProverClientFactory,
  private val l2EthClient: EthApiClient,
  private val zkStateClient: StateManagerV1JsonRpcClient,
  private val tracesClients: TracesClients,
) : LongRunningService {

  private val log = LogManager.getLogger(ConflationAppOrchestrator::class.java)

  private val riscvCutoverTimestamp = configs.conflation.riscvStartingBlockTimestampInclusive
  private val lastFinalizedBlockTimestamp: Instant = if (lastFinalizedBlock == 0UL) {
    Instant.fromEpochSeconds(0)
  } else {
    l2EthClient
      .ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(lastFinalizedBlock.toLong()))
      .thenApply { block -> Instant.fromEpochSeconds(block.timestamp.toLong()) }
      .get()
  }

  private val forcedTransactionsApp: ForcedTransactionsApp = run {
    // Forced transactions are rollup-only for now; see ConflationAppHelper.forcedTransactionsEnabled.
    val ftxConfig = configs.forcedTransactions
    if (!ConflationAppHelper.forcedTransactionsEnabled(ftxConfig, configs.l1Submission.dataAvailability)) {
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
        rpcUrl = ftxConfig!!.l1Endpoint.toString(),
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

      val ftxInvalidityProofService: LongRunningService = if (riscVCutoverCrossed()) {
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
        l2EthApiClient = l2EthClient,
        ftxClient = ftxClient,
        finalizedStateProvider = contractClient,
        contractVersionProvider = contractClient,
        clock = clock,
        metricsFacade = metricsFacade,
        ftxInvalidityProofService = ftxInvalidityProofService,
      )
    }
  }

  private val lastProcessedBlocks = if (riscVCutoverCrossed()) {
    ConflationAppHelper.getLastRiscVProcessedBlocks(lastFinalizedBlock, l2EthClient).get()
  } else {
    ConflationAppHelper.getLastConflatedAndAggregatedBlocks(
      lastFinalizedBlock = lastFinalizedBlock,
      aggregationsRepository = aggregationsRepository,
      l2EthClient = l2EthClient,
    ).get()
  }

  private val lastProvenBlockNumberProvider = run {
    val lastProvenConsecutiveBatchBlockNumberProvider = BatchesRepoBasedLastProvenBlockNumberProvider(
      lastProcessedBlocks.lastConflatedBlock.number.toLong(),
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

  // This object acts as an independent periodic polling service which is responsible
  // for monitoring the highest consecutive proven block number in the batch db
  private val provenBlockNumberMonitor = object : VertxPeriodicPollingService(
    vertx = vertx,
    pollingIntervalMs = 1.seconds.inWholeMilliseconds,
    log = log,
    name = "ProvenBlockNumberMonitor",
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {
    override fun action(): SafeFuture<*> {
      return lastProvenBlockNumberProvider.getLastProvenBlockNumber()
    }
  }

  private fun newTargetCheckpointPauseController() =
    ConflationTargetCheckpointPauseController(
      ConflationTargetCheckpointPauseController.Config(
        initialLastImportedBlockTimestamp = lastProcessedBlocks.lastConflatedBlock.headerSummary.timestamp,
        targetEndBlocks = (configs.conflation.proofAggregation.targetEndBlocks ?: emptyList()).toSet(),
        targetTimestamps = configs.conflation.proofAggregation.timestampBasedHardForks,
        waitTargetBlockL1Finalization = configs.conflation.proofAggregation.waitTargetBlockL1Finalization,
        waitApiResumeAfterTargetBlock = configs.conflation.proofAggregation.waitApiResumeAfterTargetBlock,
      ),
      latestL1FinalizedBlockProvider = lastProvenBlockNumberProvider,
    )

  // V1 and V2 each get their own controller so that V2 importing the cutover block does not
  // pause V1 before V1's conflation pipeline has had a chance to fire HARD_FORK and seal its
  // last pre-cutover batch.
  private val targetCheckpointPauseControllerV1 = newTargetCheckpointPauseController()
  private val targetCheckpointPauseControllerV2 = newTargetCheckpointPauseController()

  private val conflationAppV1: ConflationAppV1? = if (riscVCutoverCrossed()) {
    null
  } else {
    ConflationAppV1(
      vertx = vertx,
      clock = clock,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository,
      forcedTransactionsDao = forcedTransactionsDao,
      configs = configs,
      metricsFacade = metricsFacade,
      httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      proverClientFactory = proverClientFactory,
      l2EthClient = l2EthClient,
      zkStateClient = zkStateClient,
      tracesClients = tracesClients,
      forcedTransactionsApp = forcedTransactionsApp,
      lastProcessedBlocks = lastProcessedBlocks,
      targetCheckpointPauseController = targetCheckpointPauseControllerV1,
      lastProvenBlockNumberProviderSync = lastProvenBlockNumberProvider,
      lastestL1FinalizedBlockProviderSync = lastProvenBlockNumberProvider,
    )
  }

  private val conflationAppV2: ConflationAppV2? =
    if (configs.conflation.riscvStartingBlockTimestampInclusive != null) {
      ConflationAppV2(
        vertx = vertx,
        batchesRepository = batchesRepository,
        configs = configs,
        forcedTransactionsApp = forcedTransactionsApp,
        forcedTransactionsDao = forcedTransactionsDao,
        metricsFacade = metricsFacade,
        lastProvenBlockNumberProvider = lastProvenBlockNumberProvider,
        targetCheckpointPauseController = targetCheckpointPauseControllerV2,
        lastProcessedBlocks = lastProcessedBlocks,
      )
    } else {
      null
    }

  override fun start(): CompletableFuture<Unit> {
    return (conflationAppV1?.start() ?: SafeFuture.completedFuture(Unit))
      .thenCompose { forcedTransactionsApp.start() }
      .thenCompose { provenBlockNumberMonitor.start() }
      .thenCompose {
        if (riscVCutoverCrossed()) {
          // Already past cutover: V2 resumes from a known block number, completes quickly.
          conflationAppV2?.start() ?: SafeFuture.completedFuture(Unit)
        } else {
          // Pre-cutover: V2 polls until the cutover timestamp arrives on L2. Start it in the
          // background so the rest of CoordinatorApp (L1 relay, API, …) can come up immediately.
          conflationAppV2?.start()?.exceptionally { e ->
            log.error("ConflationAppV2 failed to start: {}", e.message, e)
          }
          SafeFuture.completedFuture(Unit)
        }
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return (conflationAppV1?.stop() ?: SafeFuture.completedFuture(Unit))
      .thenCompose { conflationAppV2?.stop() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { forcedTransactionsApp.stop() }
      .thenCompose { provenBlockNumberMonitor.stop() }
  }

  fun riscVCutoverCrossed(): Boolean =
    riscvCutoverTimestamp != null && lastFinalizedBlockTimestamp >= riscvCutoverTimestamp

  fun updateLatestL1FinalizedBlock(blockNumber: Long): SafeFuture<Unit> =
    lastProvenBlockNumberProvider.updateLatestL1FinalizedBlock(blockNumber)

  fun signalTargetCheckpointResumeFromApi(): Boolean {
    val v1 = targetCheckpointPauseControllerV1.signalResumeFromApi()
    val v2 = targetCheckpointPauseControllerV2.signalResumeFromApi()
    return v1 || v2
  }
}
