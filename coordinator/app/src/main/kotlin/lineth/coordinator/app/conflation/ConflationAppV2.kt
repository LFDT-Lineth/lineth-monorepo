package lineth.coordinator.app.conflation

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.domain.Batch
import linea.domain.Block
import linea.domain.BlockCounters
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.ftx.ForcedTransactionsApp
import linea.kotlin.encodeHex
import linea.timer.TimerSchedule
import linea.web3j.ethapi.createEthApiClient
import lineth.conflation.AlwaysSafeBlockNumberProvider
import lineth.conflation.ConflationService
import lineth.conflation.calculators.ConflationTriggerCalculatorByBlockLimit
import lineth.conflation.calculators.GlobalBlockConflationCalculator
import lineth.coordination.blockcreation.BlockCreated
import lineth.coordination.blockcreation.BlockCreationListener
import lineth.coordination.conflation.ConflationServiceImpl
import lineth.coordination.proofcreation.BatchProofHandlerImpl
import lineth.coordination.riscv.execution.ExecutionProofGeneratingCoordinator
import lineth.coordination.riscv.execution.L2ExecutionProofHandler
import lineth.coordination.riscv.execution.L2ExecutionRequestBuilderImpl
import lineth.coordinator.blockcreation.BatchesRepoBasedLastProvenBlockNumberProvider
import lineth.coordinator.blockcreation.BlockCreationMonitor
import lineth.coordinator.blockcreation.TargetCheckpointPauseController
import lineth.coordinator.clients.prover.riscv.RiscvProverClientFactory
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.encoding.BlockRLPEncoder
import lineth.persistence.BatchesRepository
import lineth.persistence.ForcedTransactionsDao
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.async.toSafeFutureNonNull
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCountersV2
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Monitors L2 blocks starting from the RISC-V cutover
 * (riscvStartingBlockTimestampInclusive in ConflationConfig) and feeds them into the
 * RISC-V execution proof pipeline.
 *
 * ConflationAppV1 stops at the block immediately before the cutover; this app picks up
 * from the next block onward.
 */
class ConflationAppV2(
  private val vertx: Vertx,
  private val lastFinalizedBlock: ULong,
  private val batchesRepository: BatchesRepository,
  private val configs: CoordinatorConfig,
  val forcedTransactionsApp: ForcedTransactionsApp,
  private val metricsFacade: MetricsFacade,
) : LongRunningService {

  private val log = LogManager.getLogger(ConflationAppV2::class.java)
  private var blockCreationMonitor: BlockCreationMonitor? = null

  init {
    requireNotNull(configs.conflation.riscvStartingBlockTimestampInclusive) {
      "riscvStartingBlockTimestampInclusive must be set to use ConflationAppV2"
    }
    requireNotNull(configs.riscvProversConfig) {
      "riscvProversConfig must be set to use ConflationAppV2"
    }
  }

  private val lastProvenBlockNumberProvider =
    BatchesRepoBasedLastProvenBlockNumberProvider(
      startingBlockNumberExclusive = lastFinalizedBlock.toLong(),
      latestL1FinalizedBlock = lastFinalizedBlock.toLong(),
      batchesRepository = batchesRepository,
    )

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

  private val targetCheckpointPauseController =
    object : TargetCheckpointPauseController {
      override fun shouldPauseConflation() = false
      override fun importBlock(block: Block) = Unit
      override fun signalResumeFromApi() = false
    }

  private val l2EthClient: EthApiClient = createEthApiClient(
    rpcUrl = configs.conflation.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.conflation"),
    requestRetryConfig = configs.conflation.l2RequestRetries,
    vertx = vertx,
  )

  private val coinbase: String = l2EthClient.ethCoinbase().get().encodeHex()
  private val chainId: ULong = l2EthClient.ethChainId().get()

  private val riscvProverClientFactory = RiscvProverClientFactory(
    vertx = vertx,
    config = configs.riscvProversConfig!!,
    l2MessageServiceAddress = configs.protocol.l2.contractAddress,
    coinbase = coinbase,
    metricsFacade = metricsFacade,
  )

  private val chainConfigProvider = ChainConfigProvider(
    chainId = chainId,
    proversConfig = configs.riscvProversConfig!!,
  )

  private val executionPipeline: ExecutionPipeline = configs.riscvProversConfig!!.let { riscvProversConfig ->
    val blocksPerBatch = requireNotNull(configs.conflation.blocksLimit) {
      "conflation.blocksLimit must be set when riscv is enabled"
    }

    val conflationCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = lastFinalizedBlock,
      syncCalculators = listOf(ConflationTriggerCalculatorByBlockLimit(blocksPerBatch)),
      deferredTriggerConflationCalculators = emptyList(),
      emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
    )
    val conflationService = ConflationServiceImpl(
      calculator = conflationCalculator,
      safeBlockNumberProvider = AlwaysSafeBlockNumberProvider(),
      metricsFacade = metricsFacade,
    )

    val l2ExecutionProverClient = riscvProverClientFactory.executionProverClient()

    val executionWitnessClient = Web3jExecutionWitnessClient(
      web3jService = createWeb3jHttpService(rpcUrl = configs.conflation.l2Endpoint.toString()),
    )
    val requestBuilder = L2ExecutionRequestBuilderImpl(
      executionWitnessClient = executionWitnessClient,
      forcedTransactionsDao = forcedTransactionsDao,
      chainConfigProvider = chainConfigProvider,
    )

    val batchProofHandler = BatchProofHandlerImpl(batchesRepository)
    val l2ExecutionProofHandler = L2ExecutionProofHandler { proof ->
      batchProofHandler.acceptNewBatch(
        Batch(startBlockNumber = proof.startBlockNumber, endBlockNumber = proof.endBlockNumber),
      )
    }

    val coordinator = ExecutionProofGeneratingCoordinator(
      l2ExecutionProverClient = l2ExecutionProverClient,
      l2ExecutionRequestBuilder = requestBuilder,
      l2ExecutionProofHandler = l2ExecutionProofHandler,
      vertx = vertx,
      config = ExecutionProofGeneratingCoordinator.Config(
        conflationAndProofGenerationRetryBackoffDelay = configs.conflation.l2RequestRetries.backoffDelay,
        executionProofPollingInterval = riscvProversConfig.proverA.execution.pollingInterval,
      ),
      metricsFacade = metricsFacade,
    )
    conflationService.onConflatedBatch(coordinator::handleConflatedBatch)

    ExecutionPipeline(conflationCalculator, conflationService, coordinator)
  }

  // When pipeline is active, on the first block we initialize the calculator's lastBlockNumber
  // to firstBlock - 1. This handles the ByTimestampInclusive cold-start case where the
  // monitor binary-searches to an arbitrary block number rather than lastFinalizedBlock + 1.
  // The listener returns the encoding+newBlock future so the monitor processes blocks
  // sequentially, avoiding any initialization race.
  private var firstBlockSeen = false

  private fun encodeBlock(block: Block): SafeFuture<ByteArray> =
    vertx.executeBlocking(Callable { BlockRLPEncoder.encode(block) }).toSafeFutureNonNull()

  private val blockCreationListener: BlockCreationListener = BlockCreationListener { blockCreated: BlockCreated ->
    val block = blockCreated.block
    encodeBlock(block)
      .thenApply { blockRlp ->
        if (!firstBlockSeen) {
          executionPipeline.conflationCalculator.lastBlockNumber = block.number - 1uL
          firstBlockSeen = true
        }
        executionPipeline.conflationService.newBlock(
          block,
          BlockCounters(
            blockNumber = block.number,
            blockTimestamp = Instant.fromEpochSeconds(block.timestamp.toLong()),
            tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
            blockRLPEncoded = blockRlp,
            numOfTransactions = block.transactions.size.toUInt(),
            gasUsed = block.gasUsed,
          ),
        )
      }.whenException { th ->
        log.error("Failed to conflate block={} errorMessage={}", block.number, th.message, th)
      }.thenApply { }
  }

  /**
   * Returns the block number of the last block processed by the RISC-V proof pipeline,
   * or null if no RISC-V blocks have been processed yet (cold start).
   *
   * Stubbed to null until the RISC-V proof repository is implemented.
   */
  private fun getLastRiscVConflatedBlock(): SafeFuture<ULong?> = SafeFuture.completedFuture(null)

  private fun resolveStartingPoint(): SafeFuture<BlockCreationMonitor.StartingPoint> {
    val cutover = configs.conflation.riscvStartingBlockTimestampInclusive!!
    return getLastRiscVConflatedBlock().thenCompose { riscvLastBlock ->
      val candidateBlock = maxOf(lastFinalizedBlock, riscvLastBlock ?: lastFinalizedBlock)
      l2EthClient
        .ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(candidateBlock.toLong()))
        .thenApply { block ->
          val blockTimestamp = Instant.fromEpochSeconds(block.timestamp.toLong())
          if (blockTimestamp < cutover) {
            log.info(
              "Cold start: no RISC-V progress found. " +
                "Will wait for cutover timestamp {}. candidateBlock={} blockTimestamp={}",
              cutover,
              candidateBlock,
              blockTimestamp,
            )
            BlockCreationMonitor.StartingPoint.ByTimestampInclusive(cutover)
          } else {
            log.info(
              "Resuming RISC-V conflation from block {}. blockTimestamp={} cutover={}",
              candidateBlock,
              blockTimestamp,
              cutover,
            )
            BlockCreationMonitor.StartingPoint.ByBlockNumberExclusive(candidateBlock.toLong())
          }
        }
    }
  }

  override fun start(): CompletableFuture<Unit> {
    return resolveStartingPoint()
      .thenCompose { startingPoint ->
        val monitor =
          BlockCreationMonitor(
            vertx = vertx,
            ethApi = l2EthClient,
            startingPoint = startingPoint,
            blockCreationListener = blockCreationListener,
            lastProvenBlockNumberProviderSync = lastProvenBlockNumberProvider,
            config =
            BlockCreationMonitor.Config(
              pollingInterval = configs.conflation.blocksPollingInterval,
              blocksToFinalization = 0L,
              blocksFetchLimit = configs.conflation.l2FetchBlocksLimit.toLong(),
            ),
            targetCheckpointPauseController = targetCheckpointPauseController,
          )
        blockCreationMonitor = monitor
        val coordinatorStart = executionPipeline.executionProofCoordinator.start().toSafeFuture()
        val provenMonitorStart = provenBlockNumberMonitor.start().toSafeFuture()
        SafeFuture.allOf(coordinatorStart, provenMonitorStart)
          .thenCompose { monitor.start() }
          .thenPeek { log.info("ConflationAppV2 started with startingPoint={}", startingPoint) }
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    val monitorStop = blockCreationMonitor
      ?.let { SafeFuture.allOf(it.stop()).thenApply { log.info("ConflationAppV2 stopped") } }
      ?: SafeFuture.completedFuture(Unit)
    val coordinatorStop = executionPipeline.executionProofCoordinator.stop().toSafeFuture()
    val provenMonitorStop = provenBlockNumberMonitor.stop().toSafeFuture()
    return SafeFuture.allOf(monitorStop, coordinatorStop, provenMonitorStop).thenApply { }
  }

  private data class ExecutionPipeline(
    val conflationCalculator: GlobalBlockConflationCalculator,
    val conflationService: ConflationService,
    val executionProofCoordinator: ExecutionProofGeneratingCoordinator,
  )
}
