package lineth.coordinator.blockcreation

import io.vertx.core.Vertx
import linea.domain.Block
import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import linea.kotlin.encodeHex
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import lineth.coordination.blockcreation.BlockCreated
import lineth.coordination.blockcreation.BlockCreationListener
import net.consensys.linea.async.AsyncRetryer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days
import kotlin.time.Instant

class BlockCreationMonitor(
  private val vertx: Vertx,
  private val ethApi: EthApiBlockClient,
  private val startingPoint: StartingPoint,
  private val blockCreationListener: BlockCreationListener,
  private val lastProvenBlockNumberProviderSync: LastProvenBlockNumberProviderSync,
  private val config: Config,
  private val targetCheckpointPauseController: TargetCheckpointPauseController,
  private val log: Logger = LogManager.getLogger(BlockCreationMonitor::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log,
  name = "BlockCreationMonitor",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  sealed class StartingPoint {
    data class ByBlockNumberExclusive(val blockNumberExclusive: Long) : StartingPoint()
    data class ByTimestampInclusive(val timestampInclusive: Instant) : StartingPoint()
  }

  data class Config(
    val pollingInterval: Duration,
    val blocksToFinalization: Long,
    val blocksFetchLimit: Long,
    val startingBlockWaitTimeout: Duration = 14.days,
    val lastL2BlockNumberToProcessInclusive: ULong? = null,
    val lastL2BlockTimestampToProcessInclusive: Instant? = null,
  )

  private val _nexBlockNumberToFetch: AtomicLong = AtomicLong(
    when (startingPoint) {
      is StartingPoint.ByBlockNumberExclusive -> startingPoint.blockNumberExclusive + 1
      is StartingPoint.ByTimestampInclusive -> -1L
    },
  )
  private val expectedParentBlockHash: AtomicReference<ByteArray> = AtomicReference(null)
  private val reorgDetected: AtomicBoolean = AtomicBoolean(false)
  private var statingBlockAvailabilityFuture: SafeFuture<*>? = null

  private val nexBlockNumberToFetch: Long
    get() = _nexBlockNumberToFetch.get()

  override fun handleError(error: Throwable) {
    log.error("Error with block creation monitor: errorMessage={}", error.message, error)
  }

  @Synchronized
  override fun start(): SafeFuture<Unit> {
    if (reorgDetected.get()) {
      return SafeFuture.failedFuture(IllegalStateException("Reorg detect. Cannot restart"))
    }

    return awaitStartingBlockToBePresent()
      .thenApply {
        super.start()
      }
  }

  @Synchronized
  fun awaitStartingBlockToBePresent(): SafeFuture<*> {
    if (statingBlockAvailabilityFuture == null) {
      statingBlockAvailabilityFuture = when (startingPoint) {
        is StartingPoint.ByBlockNumberExclusive -> awaitBlockByNumber(startingPoint.blockNumberExclusive)
        is StartingPoint.ByTimestampInclusive -> awaitBlockByTimestamp(startingPoint.timestampInclusive)
      }
    }
    return statingBlockAvailabilityFuture!!
  }

  private fun awaitBlockByNumber(blockNumberExclusive: Long): SafeFuture<*> {
    log.info("Awaiting for block {} to be present", blockNumberExclusive)
    return AsyncRetryer.retry(
      vertx,
      backoffDelay = config.pollingInterval,
      timeout = config.startingBlockWaitTimeout,
      stopRetriesPredicate = { block: Block? ->
        if (block == null) {
          log.warn(
            "block={} not found yet. Retrying in {}",
            blockNumberExclusive,
            config.pollingInterval,
          )
          false
        } else {
          log.info("Block {} found. Resuming block monitor", blockNumberExclusive)
          expectedParentBlockHash.set(block.hash)
          true
        }
      },
    ) {
      ethApi.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(blockNumberExclusive))
    }
  }

  private fun awaitBlockByTimestamp(targetTimestamp: Instant): SafeFuture<*> {
    log.info("Waiting for cutover timestamp to be reached on L2, timestamp={}", targetTimestamp)
    return AsyncRetryer.retry(
      vertx,
      backoffDelay = config.pollingInterval,
      timeout = config.startingBlockWaitTimeout,
      stopRetriesPredicate = { block: Block? ->
        val reached = block != null && block.timestamp >= targetTimestamp.epochSeconds.toULong()
        if (!reached) {
          log.debug(
            "Latest block hasn't reached cutover. latestBlockTimestamp={} target={}",
            block?.let { Instant.fromEpochSeconds(it.timestamp.toLong()) },
            targetTimestamp,
          )
        }
        reached
      },
    ) {
      ethApi.ethBlockNumber()
        .thenCompose { n -> ethApi.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(n.toLong())) }
    }.thenCompose { latestBlock ->
      log.info(
        "Cutover timestamp reached. Binary searching for exact first block in 0..{}. targetTimestamp={}, " +
          "latestBlockNumber={}, " +
          "latestBlockTimestamp={}",
        latestBlock.number,
        targetTimestamp,
        latestBlock.number,
        Instant.fromEpochSeconds(latestBlock.timestamp.toLong()),
      )
      binarySearchFirstBlockAtOrAfterTimestamp(0L, latestBlock.number.toLong(), targetTimestamp)
    }.thenCompose { firstBlockNumber ->
      _nexBlockNumberToFetch.set(firstBlockNumber)
      log.info(
        "Block creation monitor ready. Starting from block number={}",
        firstBlockNumber,
      )
      val setParentHash = if (firstBlockNumber == 0L) {
        expectedParentBlockHash.set(ByteArray(32))
        SafeFuture.completedFuture(Unit)
      } else {
        ethApi.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(firstBlockNumber - 1))
          .thenApply { parentBlock ->
            expectedParentBlockHash.set(parentBlock.hash)
          }
      }
      // Notify the checkpoint controller with the first cutover block before the polling loop
      // starts, so the very first tick is already paused and no blocks are imported until
      // L1 finalization of the pre-cutover batch allows the loop to proceed.
      setParentHash.thenCompose {
        ethApi.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(firstBlockNumber))
          .thenApply { firstBlock ->
            if (firstBlock != null) {
              targetCheckpointPauseController.importBlock(firstBlock)
            }
          }
      }
    }
  }

  private fun binarySearchFirstBlockAtOrAfterTimestamp(
    low: Long,
    high: Long,
    targetTimestamp: Instant,
  ): SafeFuture<Long> {
    if (low == high) return SafeFuture.completedFuture(low)
    val mid = low + (high - low) / 2
    return ethApi.ethFindBlockByNumberFullTxs(BlockParameter.fromNumber(mid))
      .thenCompose { block ->
        if (block == null || block.timestamp < targetTimestamp.epochSeconds.toULong()) {
          binarySearchFirstBlockAtOrAfterTimestamp(mid + 1, high, targetTimestamp)
        } else {
          binarySearchFirstBlockAtOrAfterTimestamp(low, mid, targetTimestamp)
        }
      }
  }

  override fun action(): SafeFuture<*> {
    if (targetCheckpointPauseController.shouldPauseConflation()) {
      log.trace("target checkpoint pause: skipping tick nexBlockNumberToFetch={}", nexBlockNumberToFetch)
      return SafeFuture.completedFuture(Unit)
    }
    log.trace("tick start: nexBlockNumberToFetch={}", nexBlockNumberToFetch)
    return lastProvenBlockNumberProviderSync.getLastKnownProvenBlockNumber()
      .let { lastProvenBlockNumber ->
        if (!nextBlockNumberWithinLimit(lastProvenBlockNumber)) {
          log.warn(
            "Gap between highest consecutive proven block and L2 block is too big: lastProvenBlock={} " +
              "nextBlockToFetch={} gapOverflow={} gapLimit={}",
            lastProvenBlockNumber,
            _nexBlockNumberToFetch.get(),
            _nexBlockNumberToFetch.get() - lastProvenBlockNumber,
            config.blocksFetchLimit,
          )
          SafeFuture.COMPLETE
        } else if (config.lastL2BlockNumberToProcessInclusive != null &&
          nexBlockNumberToFetch.toULong() > config.lastL2BlockNumberToProcessInclusive
        ) {
          log.warn(
            "stopping conflation at lastL2BlockNumberInclusiveToProcess - 1. " +
              "All blocks upto and including lastL2BlockNumberInclusiveToProcess={} have been processed. " +
              "nextBlockNumberToFetch={}",
            config.lastL2BlockNumberToProcessInclusive,
            nexBlockNumberToFetch,
          )
          this.stop()
          SafeFuture.COMPLETE
        } else {
          getNetNextSafeBlock()
            .thenCompose { block ->
              if (block != null) {
                if (block.parentHash.contentEquals(expectedParentBlockHash.get())) {
                  if (isAfterTargetStopTimeStamp(block)) {
                    log.warn(
                      "stopping conflation: reached lastL2BlockTimestampToProcessInclusive={} " +
                        "last processed blockNumber={} blockTimestamp={} {}",
                      config.lastL2BlockTimestampToProcessInclusive,
                      block.number,
                      block.timestamp,
                      Instant.fromEpochSeconds(block.timestamp.toLong()),
                    )
                    this.stop()
                  }
                  notifyListener(block)
                    .whenSuccess {
                      log.debug(
                        "updating nexBlockNumberToFetch from {} --> {}",
                        _nexBlockNumberToFetch.get(),
                        _nexBlockNumberToFetch.incrementAndGet(),
                      )
                      expectedParentBlockHash.set(block.hash)
                      targetCheckpointPauseController.importBlock(block)
                    }
                } else {
                  reorgDetected.set(true)
                  log.error(
                    "Shooting down conflation poller, chain reorg detected: " +
                      "block { blockNumber={} hash={} parentHash={} } should have parentHash={}",
                    block.number,
                    block.hash.encodeHex(),
                    block.parentHash.encodeHex(),
                    expectedParentBlockHash.get().encodeHex(),
                  )
                  this.stop()
                }
              } else {
                SafeFuture.completedFuture(Unit)
              }
            }
            .whenException { error ->
              log.warn("Block creation monitor failed: errorMessage={}", error.message, error)
            }.whenComplete { _, _ ->
              log.trace("tick end")
            }
        }
      }
  }

  private fun isAfterTargetStopTimeStamp(block: Block): Boolean {
    return config.lastL2BlockTimestampToProcessInclusive != null &&
      block.timestamp >= config.lastL2BlockTimestampToProcessInclusive.epochSeconds.toULong()
  }

  private fun notifyListener(payload: Block): SafeFuture<Unit> {
    log.trace("notifying blockCreationListener: block={}", payload.number)
    return blockCreationListener
      .acceptBlock(BlockCreated(payload))
      .thenApply {
        log.debug(
          "blockCreationListener blockNumber={} resolved with success",
          payload.number,
        )
      }
      .whenException { throwable ->
        log.warn(
          "Failed to notify blockCreationListener: blockNumber={} errorMessage={}",
          payload.number,
          throwable.message,
          throwable,
        )
      }
  }

  private fun getNetNextSafeBlock(): SafeFuture<Block?> {
    return ethApi
      .ethBlockNumber()
      .thenCompose { latestBlockNumber ->
        // Check if is safe to fetch nextWaitingBlockNumber
        if (latestBlockNumber.toLong() >=
          _nexBlockNumberToFetch.get() + config.blocksToFinalization
        ) {
          val blockNumber = _nexBlockNumberToFetch.get()
          ethApi.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(blockNumber))
            .thenPeek { block ->
              log.trace("requestedBlock={} responseBlock={}", blockNumber, block?.number)
            }
            .whenException {
              log.warn(
                "eth_getBlockByNumber({}) failed: errorMessage={}",
                blockNumber,
                it.message,
                it,
              )
            }
        } else {
          SafeFuture.completedFuture(null)
        }
      }
  }

  private fun nextBlockNumberWithinLimit(lastProvenBlockNumber: Long): Boolean {
    return _nexBlockNumberToFetch.get() - lastProvenBlockNumber <= config.blocksFetchLimit
  }
}
