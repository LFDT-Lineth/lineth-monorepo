package lineth.coordinator.app.conflation

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.domain.Block
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.web3j.ethapi.createEthApiClient
import lineth.coordination.blockcreation.BlockCreationListener
import lineth.coordinator.blockcreation.BatchesRepoBasedLastProvenBlockNumberProvider
import lineth.coordinator.blockcreation.BlockCreationMonitor
import lineth.coordinator.blockcreation.TargetCheckpointPauseController
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.persistence.BatchesRepository
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
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
) : LongRunningService {

  private val log = LogManager.getLogger(ConflationAppV2::class.java)
  private var blockCreationMonitor: BlockCreationMonitor? = null

  init {
    requireNotNull(configs.conflation.riscvStartingBlockTimestampInclusive) {
      "riscvStartingBlockTimestampInclusive must be set to use ConflationAppV2"
    }
  }

  private val blockCreationListener = BlockCreationListener { blockCreated ->
    log.info("ConflationAppV2: received block number={}", blockCreated.block.number)
    SafeFuture.completedFuture(Unit)
  }

  private val lastProvenBlockNumberProvider =
    BatchesRepoBasedLastProvenBlockNumberProvider(
      startingBlockNumberExclusive = lastFinalizedBlock.toLong(),
      latestL1FinalizedBlock = lastFinalizedBlock.toLong(),
      batchesRepository = batchesRepository,
    )

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
        monitor
          .start()
          .thenPeek { log.info("ConflationAppV2 started with startingPoint={}", startingPoint) }
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return blockCreationMonitor
      ?.let { SafeFuture.allOf(it.stop()).thenApply { log.info("ConflationAppV2 stopped") } }
      ?: SafeFuture.completedFuture(Unit)
  }
}
