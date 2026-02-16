package linea.ethapi

import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLog
import linea.ethapi.extensions.EthLogConsumer
import linea.ethapi.extensions.EthLogsFilterState
import linea.ethapi.extensions.EthLogsFilterStateListener
import linea.ethapi.extensions.EthLogsFilterSubscriptionFactory
import linea.ethapi.extensions.EthLogsFilterSubscriptionManager
import linea.ethapi.extensions.getAbsoluteBlockNumbers
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.async.RetriedExecutionException
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.concurrent.atomics.AtomicReference
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.time.Duration

class EthLogsFilterSubscriptionFactoryPollingBased(
  private val vertx: Vertx,
  private val ethApiClient: EthApiClient,
  private val l1FtxLogsPollingInterval: Duration,
  private val blockChunkSize: UInt = 1000u,
) : EthLogsFilterSubscriptionFactory {
  override fun create(
    filterOptions: EthLogsFilterOptions,
    logsConsumer: EthLogConsumer?,
  ): EthLogsFilterSubscriptionManager {
    return EthLogsFilterPoller(
      vertx = vertx,
      ethApiClient = ethApiClient,
      filterOptions = filterOptions,
      l1FtxLogsPollingInterval = l1FtxLogsPollingInterval,
      blockChunkSize = blockChunkSize,
    ).also { if (logsConsumer != null) it.setConsumer(logsConsumer) }
  }
}

/**
 * Implements the concept of eth_newFilter + eth_getFilterChanges
 * through polling eth_getLogs in small block chunks,
 * by moving the fromBlock..toBlock window filter forward
 *
 * Provides exactly-once semantics: each log is delivered to the consumer exactly once
 * upon success, and retried on failure until successful.
 *
 * when filter.toBlock is a tag (FINALIZED, SAFE, LATEST) the service will keep on polling forever
 * PENDING tag is not supported.
 */
@OptIn(ExperimentalAtomicApi::class)
class EthLogsFilterPoller(
  vertx: Vertx,
  private val ethApiClient: EthApiClient,
  private val filterOptions: EthLogsFilterOptions,
  private val l1FtxLogsPollingInterval: Duration,
  private val blockChunkSize: UInt = 1000u,
  private val log: Logger = LogManager.getLogger(EthLogsFilterPoller::class.java),
) :
  EthLogsFilterSubscriptionManager,
  VertxPeriodicPollingService(
    vertx = vertx,
    pollingIntervalMs = l1FtxLogsPollingInterval.inWholeMilliseconds,
    log = log,
    name = "EthLogsFilterPoller-${filterOptions.topics.firstOrNull()?.takeLast(8)}",
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {
  private data class LogPosition(val blockNumber: ULong, val logIndex: ULong) : Comparable<LogPosition> {
    override fun compareTo(other: LogPosition): Int {
      return compareValuesBy(this, other, { it.blockNumber }, { it.logIndex })
    }
  }

  private val ethLogsSearcher: EthLogsSearcher =
    EthLogsSearcherImpl(
      vertx,
      ethApiClient,
      config = EthLogsSearcherImpl.Config(),
    )
  private lateinit var consumer: EthLogConsumer
  private var lastSearchedBlock: ULong? = null

  // Tracks the position of the last successfully processed log for exactly-once semantics
  private var lastProcessedPosition: LogPosition? = null

  // State tracking
  private var stateListener: EthLogsFilterStateListener? = null

  private val currentState: AtomicReference<EthLogsFilterState> = AtomicReference(EthLogsFilterState.Idle)

  override fun setConsumer(logsConsumer: EthLogConsumer) {
    this.consumer = logsConsumer
  }

  override fun setStateListener(stateListener: EthLogsFilterStateListener?) {
    this.stateListener = stateListener
  }

  private fun transitionTo(newState: EthLogsFilterState) {
    val lastState = currentState.load()
    if (lastState != newState) {
      currentState.store(newState)
      stateListener?.onStateChange(lastState, newState)
      log.debug("state transition: {} -> {}", lastState, newState)
    }
  }

  override fun start(): SafeFuture<Unit> {
    if (!this::consumer.isInitialized) {
      throw IllegalStateException("Please setConsumer() before stating to poll for the logs")
    }
    return super.start()
  }

  private fun isLogAlreadyProcessed(ethLog: EthLog): Boolean {
    val position = LogPosition(ethLog.blockNumber, ethLog.logIndex)
    return lastProcessedPosition?.let { position <= it } ?: false
  }

  override fun action(): SafeFuture<*> {
    val fromBlock = lastSearchedBlock?.let { (it + 1u).toBlockParameter() } ?: filterOptions.fromBlock
    val filter = EthLogsFilterOptions(
      fromBlock = fromBlock,
      toBlock = filterOptions.toBlock,
      address = filterOptions.address,
      topics = filterOptions.topics,
    )

    return ethApiClient.getAbsoluteBlockNumbers(
      filter.fromBlock,
      filter.toBlock,
    ).thenCompose { (start, end) ->
      if (start > end) {
        // it means we reached toBlock tag (eg SAFE/FINALIZED) on L1, wait for the next tick
        transitionTo(EthLogsFilterState.CaughtUp(lastSearchedBlockNumber = lastSearchedBlock ?: 0u))
        log.trace(
          "skipping search iteration, block interval is empty: from={} to={}({})",
          fromBlock,
          end,
          filter.toBlock,
        )
        return@thenCompose SafeFuture.completedFuture(Unit)
      }

      // Searching through available blocks
      transitionTo(EthLogsFilterState.Searching(startBlockNumber = start))
      log.debug("fetching logs: filter={}", filter)

      ethLogsSearcher.getLogsRollingForward(
        filter = filter,
        chunkSize = blockChunkSize,
        searchTimeout = l1FtxLogsPollingInterval,
        stopAfterTargetLogsCount = null, // No limit, fetch all available logs
      ).thenApply { result ->
        var processedCount = 0
        var skippedCount = 0

        // Process logs sequentially, tracking position for exactly-once semantics
        for (ethLog in result.logs) {
          // Skip logs that have already been successfully processed
          if (isLogAlreadyProcessed(ethLog)) {
            skippedCount++
            continue
          }

          try {
            consumer.accept(ethLog)

            // Update position immediately after successful processing
            // This ensures exactly-once: if we crash/restart, we won't reprocess this log
            lastProcessedPosition = LogPosition(ethLog.blockNumber, ethLog.logIndex)
            processedCount++
          } catch (e: Exception) {
            log.error(
              "error processing eth log, will retry on next tick: " +
                "ethLogBlockNumber={} ethLogIndex={} errorMessage={}",
              ethLog.blockNumber,
              ethLog.logIndex,
              e.message,
              e,
            )
            // Don't update position - this log will be retried on next poll
            // Stop processing further logs to maintain order
            throw e
          }
        }

        // Update last searched block to the end of the range we searched
        // This allows us to move forward even if some logs failed
        if (result.logs.isNotEmpty()) {
          lastSearchedBlock = result.endBlockNumber
          log.debug(
            "processed {} logs, skipped {} already-processed logs, lastSearchedBlock={}, lastProcessedPosition={}",
            processedCount,
            skippedCount,
            lastSearchedBlock,
            lastProcessedPosition,
          )
        } else if (result.startBlockNumber <= result.endBlockNumber) {
          // No logs found but we did search a range, move forward
          lastSearchedBlock = result.endBlockNumber
          log.trace("no new logs found in block range {}..{}", result.startBlockNumber, result.endBlockNumber)
        }
      }
    }
  }

  override fun handleError(error: Throwable) {
    if (error.cause is RetriedExecutionException &&
      error.cause?.message?.startsWith("Stop condition wasn't met after timeout") ?: false
    ) {
      // ignore timeout errors, we're just waiting for the next tick'
      return
    } else {
      super.handleError(error)
    }
  }
}
