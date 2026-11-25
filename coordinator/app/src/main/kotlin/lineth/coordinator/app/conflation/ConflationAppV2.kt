package lineth.coordinator.app.conflation

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.da.DaCodecVersion
import linea.da.RollupResumePoint
import linea.domain.Batch
import linea.domain.Block
import linea.domain.BlockCounters
import linea.domain.BlocksConflation
import linea.ethapi.EthApiClient
import linea.ethapi.systemContracts
import linea.ftx.ForcedTransactionsApp
import linea.kotlin.encodeHex
import linea.web3j.createWeb3jHttpService
import linea.web3j.ethapi.Web3jEthConfigClient
import linea.web3j.ethapi.Web3jExecutionPayloadClient
import linea.web3j.ethapi.Web3jExecutionWitnessClient
import linea.web3j.ethapi.createEthApiClient
import linea.web3j.ethapi.validateLinethBlock
import lineth.conflation.ConflationService
import lineth.conflation.calculators.CalculatorsFactory
import lineth.conflation.calculators.GlobalBlockConflationCalculator
import lineth.coordination.SimpleCompositeSafeFutureHandler
import lineth.coordination.blockcreation.BlockCreated
import lineth.coordination.blockcreation.BlockCreationListener
import lineth.coordination.proofcreation.BatchProofHandlerImpl
import lineth.coordination.riscv.da.FakeDaCompressorFactory
import lineth.coordination.riscv.da.FakeDaStreamFactory
import lineth.coordination.riscv.execution.ExecutionProofGeneratingCoordinator
import lineth.coordination.riscv.execution.L2ExecutionProofHandler
import lineth.coordination.riscv.execution.L2ExecutionRequestBuilderImpl
import lineth.coordination.riscv.rollup.RollupProofGeneratingCoordinator
import lineth.coordination.riscv.rollup.RollupProofPersistenceHandler
import lineth.coordination.riscv.rollup.RollupProofPoller
import lineth.coordinator.blockcreation.BlockCreationMonitor
import lineth.coordinator.blockcreation.LastProvenBlockNumberProviderSync
import lineth.coordinator.blockcreation.TargetCheckpointPauseController
import lineth.coordinator.clients.prover.ProverClientFactory
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.coordinator.config.v2.RiscvProverConfig
import lineth.encoding.BlockRLPEncoder
import lineth.persistence.BatchesRepository
import lineth.persistence.ForcedTransactionsDao
import lineth.persistence.RiscvRollupsDao
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.async.toSafeFutureNonNull
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCountersV5
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration
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
  private val chainId: ULong,
  private val batchesRepository: BatchesRepository,
  private val configs: CoordinatorConfig,
  val forcedTransactionsApp: ForcedTransactionsApp,
  private val forcedTransactionsDao: ForcedTransactionsDao,
  private val metricsFacade: MetricsFacade,
  private val proverClientFactory: ProverClientFactory,
  private val lastProvenBlockNumberProvider: LastProvenBlockNumberProviderSync,
  private val targetCheckpointPauseController: TargetCheckpointPauseController,
  private val lastProcessedBlocks: LastProcessedBlocks,
  private val riscvRollupsDao: RiscvRollupsDao,
  private val rollupResumePointSupplier: (() -> SafeFuture<RollupResumePoint>)?,
) : LongRunningService {

  private val log = LogManager.getLogger(ConflationAppV2::class.java)
  private var blockCreationMonitor: BlockCreationMonitor? = null

  init {
    requireNotNull(configs.conflation.riscvStartingBlockTimestampInclusive) {
      "riscvStartingBlockTimestampInclusive must be set to use ConflationAppV2"
    }
    requireNotNull(configs.conflation.l2EngineEndpoint) {
      "conflation.l2-engine-endpoint must be set to use ConflationAppV2"
    }
    require(
      configs.proversConfig.currentProver.riscvConfig != null ||
        configs.proversConfig.nextProver?.riscvConfig != null,
    ) {
      "riscv proversConfig must be set ether in currentProver or nextProver to use ConflationAppV2"
    }
  }

  private val l2EthClient: EthApiClient = createEthApiClient(
    rpcUrl = configs.conflation.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.conflation"),
    requestRetryConfig = configs.conflation.l2RequestRetries,
    vertx = vertx,
    blockValidator = ::validateLinethBlock,
  )

  private val riscvProverConfig: RiscvProverConfig =
    configs.proversConfig.currentProver.riscvConfig ?: configs.proversConfig.nextProver?.riscvConfig!!

  private val executionPipeline: ExecutionPipeline = configs.proversConfig.let {
    val blocksPerBatch = requireNotNull(configs.conflation.blocksLimit) {
      "conflation.blocksLimit must be set when riscv is enabled"
    }

    val riscvCalculators = CalculatorsFactory.createForRiscv(
      lastConflatedBlockNumber = lastProcessedBlocks.lastConflatedBlock.number,
      lastConflatedTimestamp = maxOf(
        configs.conflation.riscvStartingBlockTimestampInclusive!!,
        Instant.fromEpochSeconds(lastProcessedBlocks.lastConflatedBlock.timestamp.toLong()),
      ),
      blocksPerBatch = blocksPerBatch,
      metricsFacade = metricsFacade,
      safeBlockNumberProvider = forcedTransactionsApp.conflationSafeBlockNumberProvider,
      extraSyncCalculators = emptyList(),
      timestampBasedHardForks = configs.conflation.proofAggregation.timestampBasedHardForks,
      aggregationTargetEndBlockNumbers = configs.conflation.proofAggregation.targetEndBlocks?.toSet() ?: emptySet(),
    )
    val conflationCalculator = riscvCalculators.conflationCalculator
    val conflationService = riscvCalculators.conflationService

    val l2ExecutionProverClient = proverClientFactory.l2ExecutionProverClient()

    val web3jService = createWeb3jHttpService(rpcUrl = configs.conflation.l2Endpoint.toString())
    val executionWitnessClient = Web3jExecutionWitnessClient(web3jService)
    val executionPayloadClient = Web3jExecutionPayloadClient(
      createWeb3jHttpService(rpcUrl = requireNotNull(configs.conflation.l2EngineEndpoint).toString()),
    )
    val requestBuilder = L2ExecutionRequestBuilderImpl(
      executionWitnessClient = executionWitnessClient,
      executionPayloadClient = executionPayloadClient,
      forcedTransactionsDao = forcedTransactionsDao,
      chainId = chainId,
      systemContracts = Web3jEthConfigClient(web3jService).ethConfig().get().systemContracts(expectedChainId = chainId),
    )

    val batchProofHandler = BatchProofHandlerImpl(batchesRepository)
    val l2ExecutionProofHandler = L2ExecutionProofHandler { proof, proofIndex ->
      batchProofHandler.acceptNewBatch(
        Batch(
          startBlockNumber = proof.startBlockNumber,
          endBlockNumber = proof.endBlockNumber,
          proofIndexHash = proofIndex.hash,
        ),
      )
    }

    val coordinator = ExecutionProofGeneratingCoordinator(
      l2ExecutionProverClient = l2ExecutionProverClient,
      l2ExecutionRequestBuilder = requestBuilder,
      l2ExecutionProofHandler = l2ExecutionProofHandler,
      vertx = vertx,
      config = ExecutionProofGeneratingCoordinator.Config(
        conflationAndProofGenerationRetryBackoffDelay = configs.conflation.l2RequestRetries.backoffDelay,
        executionProofPollingInterval = riscvProverConfig.l2Execution.pollingInterval,
      ),
      metricsFacade = metricsFacade,
    )
    ExecutionPipeline(conflationCalculator, conflationService, coordinator)
  }

  private val rollupStage: RollupStage? = configs.conflation.rollup?.let { rollupConfig ->
    require(configs.database.schemaVersion >= ROLLUP_MIN_DB_SCHEMA_VERSION) {
      "conflation.rollup requires database.schema-version >= $ROLLUP_MIN_DB_SCHEMA_VERSION: rollup requests carry " +
        "each batch's proof index hash, stored from that version on"
    }
    val resumePointSupplier = requireNotNull(rollupResumePointSupplier) {
      "conflation.rollup is set but no rollup resume point supplier was provided"
    }
    val daCodecVersion = DaCodecVersion.entries.find { it.version == rollupConfig.daCodec }
      ?: throw IllegalArgumentException(
        "unsupported conflation.rollup.da-codec=${rollupConfig.daCodec}, " +
          "supported: ${DaCodecVersion.entries.map { it.version }}",
      )
    val rollupProverClient = proverClientFactory.rollupProverClient()
    val rollupProofPoller = RollupProofPoller(
      rollupProverClient = rollupProverClient,
      rollupProofHandler = RollupProofPersistenceHandler(riscvRollupsDao),
      vertx = vertx,
      config = RollupProofPoller.Config(pollingInterval = riscvProverConfig.rollup.pollingInterval),
      metricsFacade = metricsFacade,
    )
    val coordinator = RollupProofGeneratingCoordinator(
      rollupProverClient = rollupProverClient,
      batchesRepository = batchesRepository,
      resumePointSupplier = resumePointSupplier,
      rollupProofPoller = rollupProofPoller,
      l2BlocksFetcher = EthApiL2BlocksFetcher(l2EthClient),
      daCompressorFactory = when (daCodecVersion) {
        DaCodecVersion.FAKE -> FakeDaCompressorFactory()
      },
      daStreamFactory = when (daCodecVersion) {
        DaCodecVersion.FAKE -> FakeDaStreamFactory()
      },
      blobTriggers = ConflationAppHelper.rollupBlobTriggers(rollupConfig),
      rollupTriggers = ConflationAppHelper.rollupTriggers(rollupConfig),
      blockEncoder = BlockRLPEncoder::encode,
      config = RollupProofGeneratingCoordinator.Config(
        pollingInterval = rollupConfig.pollingInterval,
        daCodecVersion = daCodecVersion,
        dictionaryPath = rollupConfig.dictionaryPath ?: Path.of(""),
        blobDeadline = rollupConfig.blobDeadline,
        rollupDeadline = rollupConfig.deadline,
        maxConflationsToFeedPerTick = rollupConfig.maxConflationsToFeedPerTick.toInt(),
      ),
      vertx = vertx,
    )
    RollupStage(coordinator, rollupProofPoller)
  }

  init {
    // The conflation service has a single handler, so it fans out to every stage that consumes conflations.
    val conflationHandler: (BlocksConflation) -> SafeFuture<*> = SimpleCompositeSafeFutureHandler(
      listOfNotNull(
        executionPipeline.executionProofCoordinator::handleConflatedBatch,
        rollupStage?.coordinator?.let { it::handleConflatedBatch },
      ),
    )
    executionPipeline.conflationService.onConflatedBatch(conflationHandler)
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
            tracesCounters = TracesCountersV5.EMPTY_TRACES_COUNT,
            blockRLPEncoded = blockRlp,
            numOfTransactions = block.transactions.size.toUInt(),
            gasUsed = block.gasUsed,
            coinbase = block.miner.encodeHex(),
          ),
        )
      }.whenException { th ->
        log.error("Failed to conflate block={} errorMessage={}", block.number, th.message, th)
      }.thenApply { }
  }

  private fun resolveStartingPoint(): BlockCreationMonitor.StartingPoint {
    val cutover = configs.conflation.riscvStartingBlockTimestampInclusive!!
    val candidateBlock = lastProcessedBlocks.lastConflatedBlock
    val blockTimestamp = Instant.fromEpochSeconds(candidateBlock.timestamp.toLong())

    return if (blockTimestamp < cutover) {
      log.info(
        "Cold start: no RISC-V progress found. " +
          "Will wait for cutover timestamp={}. candidateBlock={} blockTimestamp={}",
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
      BlockCreationMonitor.StartingPoint.ByBlockNumberExclusive(candidateBlock.number.toLong())
    }
  }

  override fun start(): CompletableFuture<Unit> {
    val startingPoint = resolveStartingPoint()
    blockCreationMonitor =
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
          startingBlockWaitTimeout = Duration.INFINITE,
        ),
        targetCheckpointPauseController = targetCheckpointPauseController,
      )
    return executionPipeline.executionProofCoordinator.start()
      .thenCompose { rollupStage?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenCompose { blockCreationMonitor!!.start() }
      .thenPeek { log.info("ConflationAppV2 started with startingPoint={}", startingPoint) }
  }

  override fun stop(): CompletableFuture<Unit> {
    val monitorStop = blockCreationMonitor?.stop() ?: SafeFuture.completedFuture(Unit)
    val coordinatorStop = executionPipeline.executionProofCoordinator.stop().toSafeFuture()
    val rollupStageStop = rollupStage?.stop() ?: SafeFuture.completedFuture(Unit)
    return SafeFuture.allOf(monitorStop, coordinatorStop, rollupStageStop)
      .thenApply { log.info("ConflationAppV2 stopped") }
  }

  companion object {
    /** The first schema version with `batches.proof_index_hash`. */
    const val ROLLUP_MIN_DB_SCHEMA_VERSION = 5
  }

  private class RollupStage(
    val coordinator: RollupProofGeneratingCoordinator,
    val proofPoller: RollupProofPoller,
  ) {
    fun start(): SafeFuture<Unit> = proofPoller.start().thenCompose { coordinator.start() }

    fun stop(): SafeFuture<Unit> = coordinator.stop().thenCompose { proofPoller.stop() }
  }

  private data class ExecutionPipeline(
    val conflationCalculator: GlobalBlockConflationCalculator,
    val conflationService: ConflationService,
    val executionProofCoordinator: ExecutionProofGeneratingCoordinator,
  )
}
