package net.consensys.zkevm.coordinator.blockcreation

import io.vertx.core.Vertx
import linea.domain.Block
import net.consensys.encodeHex
import net.consensys.linea.BlockParameter.Companion.toBlockParameter
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.web3j.ExtendedWeb3J
import net.consensys.zkevm.PeriodicPollingService
import net.consensys.zkevm.ethereum.coordination.blockcreation.BlockCreated
import net.consensys.zkevm.ethereum.coordination.blockcreation.BlockCreationListener
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days

class BlockCreationMonitor(
  private val vertx: Vertx,
  private val web3j: ExtendedWeb3J,
  private val startingBlockNumberExclusive: Long,
  private val blockCreationListener: BlockCreationListener,
  private val lastProvenBlockNumberProviderAsync: LastProvenBlockNumberProviderAsync,
  private val config: Config,
  private val log: Logger = LogManager.getLogger(BlockCreationMonitor::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log
) {
  data class Config(
    val pollingInterval: Duration,
    val blocksToFinalization: Long,
    val blocksFetchLimit: Long,
    val startingBlockWaitTimeout: Duration = 14.days,
    val lastL2BlockNumberToProcessInclusive: ULong? = null
  )

  private val _nexBlockNumberToFetch: AtomicLong = AtomicLong(startingBlockNumberExclusive + 1)
  private val expectedParentBlockHash: AtomicReference<ByteArray> = AtomicReference(null)
  private val reorgDetected: AtomicBoolean = AtomicBoolean(false)
  private var statingBlockAvailabilityFuture: SafeFuture<*>? = null

  val nexBlockNumberToFetch: Long
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
      log.info("Awaiting for block {} to be present", startingBlockNumberExclusive)
      statingBlockAvailabilityFuture = AsyncRetryer.retry(
        vertx,
        backoffDelay = config.pollingInterval,
        timeout = config.startingBlockWaitTimeout,
        stopRetriesPredicate = { block: Block? ->
          if (block == null) {
            log.warn(
              "Block {} not found yet. Retrying in {}",
              startingBlockNumberExclusive,
              config.pollingInterval
            )
            false
          } else {
            log.info("Block {} found. Resuming block monitor", startingBlockNumberExclusive)
            expectedParentBlockHash.set(block.hash)
            true
          }
        }
      ) {
        web3j.ethGetBlock(startingBlockNumberExclusive.toBlockParameter())
      }
    }

    return statingBlockAvailabilityFuture!!
  }

  override fun action(): SafeFuture<*> {
    log.trace("tick start: nexBlockNumberToFetch={}", nexBlockNumberToFetch)
    return lastProvenBlockNumberProviderAsync.getLastProvenBlockNumber()
      .thenCompose { lastProvenBlockNumber ->
        if (!nextBlockNumberWithinLimit(lastProvenBlockNumber)) {
          log.warn(
            "Gap between highest consecutive proven block and L2 block is too big: lastProvenBlock={} " +
              "nextBlockToFetch={} gapOverflow={} gapLimit={}",
            lastProvenBlockNumber,
            _nexBlockNumberToFetch.get(),
            _nexBlockNumberToFetch.get() - lastProvenBlockNumber,
            config.blocksFetchLimit
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
            nexBlockNumberToFetch
          )
          SafeFuture.COMPLETE
        } else {
          getNetNextSafeBlock()
            .thenCompose { block ->
              if (block != null) {
                if (block.parentHash.contentEquals(expectedParentBlockHash.get())) {
                  notifyListener(block)
                    .whenSuccess {
                      log.debug(
                        "updating nexBlockNumberToFetch from {} --> {}",
                        _nexBlockNumberToFetch.get(),
                        _nexBlockNumberToFetch.incrementAndGet()
                      )
                      expectedParentBlockHash.set(block.hash)
                    }
                } else {
                  reorgDetected.set(true)
                  log.error(
                    "Shooting down conflation poller, " +
                      "chain reorg detected: block { blockNumber={} hash={} parentHash={} } should have parentHash={}",
                    block.number,
                    block.hash.encodeHex(),
                    block.parentHash.encodeHex(),
                    expectedParentBlockHash.get().encodeHex()
                  )
                  SafeFuture.failedFuture(IllegalStateException("Reorg detected on block ${block.number}"))
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

  private fun notifyListener(payload: Block): SafeFuture<Unit> {
    log.trace("notifying blockCreationListener: block={}", payload.number)
    return blockCreationListener
      .acceptBlock(BlockCreated(payload))
      .thenApply {
        log.debug(
          "blockCreationListener blockNumber={} resolved with success",
          payload.number
        )
      }
      .whenException { throwable ->
        log.warn(
          "Failed to notify blockCreationListener: blockNumber={} errorMessage={}",
          payload.number,
          throwable.message,
          throwable
        )
      }
  }

  private fun getNetNextSafeBlock(): SafeFuture<Block?> {
    return web3j
      .ethBlockNumber()
      .thenCompose { latestBlockNumber ->
        // Check if is safe to fetch nextWaitingBlockNumber
        if (latestBlockNumber.toLong() >=
          _nexBlockNumberToFetch.get() + config.blocksToFinalization
        ) {
          val blockNumber = _nexBlockNumberToFetch.get()
          web3j.ethGetBlock(blockNumber.toBlockParameter())
            .thenPeek { block ->
              log.trace("requestedBock={} responselock={}", blockNumber, block?.number)
            }
            .whenException {
              log.warn(
                "eth_getBlockByNumber({}) failed: errorMessage={}",
                blockNumber,
                it.message,
                it
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
