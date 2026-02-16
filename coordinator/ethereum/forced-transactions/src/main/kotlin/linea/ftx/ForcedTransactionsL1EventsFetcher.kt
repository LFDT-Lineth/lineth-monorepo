package linea.ftx

import linea.LongRunningService
import linea.contract.events.ForcedTransactionAddedEvent
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLog
import linea.ethapi.EthApiClient
import linea.ethapi.EthLogsFilterOptions
import linea.ethapi.extensions.EthLogsFilterState
import linea.ethapi.extensions.EthLogsFilterSubscriptionFactory
import linea.ethapi.extensions.EthLogsFilterSubscriptionManager
import linea.ftx.conflation.ForcedTransactionsSafeBlockNumberManager
import linea.kotlin.toHexStringUInt256
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Queue
import java.util.concurrent.CompletableFuture
import kotlin.concurrent.atomics.AtomicLong
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.concurrent.atomics.decrementAndFetch
import kotlin.time.Instant

@OptIn(ExperimentalAtomicApi::class)
internal class ForcedTransactionsL1EventsFetcher(
  private val address: String,
  private val ethLogsClient: EthApiClient,
  private val resumePointProvider: ForcedTransactionsResumePointProvider,
  private val ethLogsFilterSubscriptionFactory: EthLogsFilterSubscriptionFactory,
  private val safeBlockNumberManager: ForcedTransactionsSafeBlockNumberManager,
  private val l1EarliestBlock: BlockParameter = BlockParameter.Tag.EARLIEST,
  private val l1HighestBlock: BlockParameter = BlockParameter.Tag.FINALIZED,
  private val ftxQueue: Queue<ForcedTransactionWithTimestamp>,
  private val log: Logger = LogManager.getLogger(ForcedTransactionsL1EventsFetcher::class.java),
) : LongRunningService {
  private lateinit var eventsSubscription: EthLogsFilterSubscriptionManager
  private var nextExpectedFtx = AtomicLong(0)

  private fun getStartBlockNumberToListenForLogs(): SafeFuture<BlockParameter> {
    return resumePointProvider
      .getLastProcessedForcedTransactionNumber()
      .thenCompose { finalizedForcedTransactionNumber ->
        nextExpectedFtx.store(finalizedForcedTransactionNumber.toLong() + 1)

        val l1BlockToStartListeningFromFuture = if (finalizedForcedTransactionNumber == 0UL) {
          // no finalized forced transactions yet, start from the beginning
          SafeFuture.completedFuture(l1EarliestBlock)
        } else {
          // get l1 block number of the latest finalized forced transaction,
          // and start listening from there
          ethLogsClient.getLogs(
            fromBlock = l1EarliestBlock,
            toBlock = l1HighestBlock,
            address = address,
            topics = listOf(
              ForcedTransactionAddedEvent.topic,
              finalizedForcedTransactionNumber.toHexStringUInt256(),
            ),
          )
            .thenApply { logs ->
              val eventLog = logs.firstOrNull()
                ?: throw IllegalStateException(
                  "No eth log found for finalized forced transaction number $finalizedForcedTransactionNumber",
                )

              val event = ForcedTransactionAddedEvent.fromEthLog(eventLog)
              log.debug(
                "latest finalized forced transaction: l1Block={} event={}",
                event.log.blockNumber,
                event.event,
              )
              // only one FTX per L1 block, so the next one can be on the next L1 block at best
              event.log.blockNumber.inc().toBlockParameter()
            }
        }

        l1BlockToStartListeningFromFuture
          .thenPeek { l1BlockToStartListeningFrom ->
            log.info(
              "start listening to ForcedTransactionAdded: from l1BlockNumber={} finalizedFtx={}",
              l1BlockToStartListeningFrom,
              finalizedForcedTransactionNumber,
            )
          }
      }
  }

  override fun start(): CompletableFuture<Unit> {
    if (::eventsSubscription.isInitialized) {
      // already started
      return CompletableFuture.completedFuture(Unit)
    }

    return getStartBlockNumberToListenForLogs()
      .thenCompose { l1BlockSinceLastFinalizedFtx ->
        val filterOptions = EthLogsFilterOptions(
          fromBlock = l1BlockSinceLastFinalizedFtx,
          toBlock = l1HighestBlock,
          address = address,
          topics = listOf(ForcedTransactionAddedEvent.topic),
        )
        this.log.info("start listening to ForcedTransactionAdded filter={}", filterOptions)
        val tmpEventsSubscription = ethLogsFilterSubscriptionFactory
          .create(
            filterOptions = filterOptions,
            logsConsumer = ::onNewForcedTransaction,
          )

        // Set up state listener to release safe block number lock when caught up
        tmpEventsSubscription.setStateListener(this::onSearchStateUpdated)
        tmpEventsSubscription
          .start()
          .toSafeFuture()
          .thenPeek {
            // only update local reference if subscription started successfully
            this.eventsSubscription = tmpEventsSubscription
          }
      }
  }

  private fun onNewForcedTransaction(ethLog: EthLog) {
    val event = ForcedTransactionAddedEvent.fromEthLog(ethLog)
    val eventIsInOrder = nextExpectedFtx
      .compareAndSet(
        event.event.forcedTransactionNumber.toLong(),
        event.event.forcedTransactionNumber.toLong() + 1,
      )
    if (eventIsInOrder) {
      // if queue is full or something fails, will throw and events fetch will retry on next tick
      runCatching {
        // Fetch L1 block timestamp
        // Using blocking call since FTX events are rare and we need the timestamp immediately
        val l1Block = ethLogsClient
          .ethGetBlockByNumberTxHashes(event.log.blockNumber.toBlockParameter())
          .get()
        val l1BlockTimestamp = Instant.fromEpochSeconds(l1Block.timestamp.toLong())

        log.info(
          "event ForcedTransactionAdded: l1Block={} l1BlockTimestamp={} event={}",
          event.log.blockNumber,
          l1BlockTimestamp,
          event.event,
        )

        // Wrap event with L1 block timestamp
        val ftxWithTimestamp = ForcedTransactionWithTimestamp(
          event = event.event,
          l1BlockTimestamp = l1BlockTimestamp,
        )
        ftxQueue.add(ftxWithTimestamp)
      }
        .onFailure {
          log.warn("failed to add event to queue, it will retry on next tick: errorMessage={}", it.message)
          // rollback expected ftx number, it was optimistically incremented above
          nextExpectedFtx.decrementAndFetch()
        }.getOrThrow()
    } else {
      val message =
        "event ForcedTransactionAdded is out of order: expectedFtx=${nextExpectedFtx.load()}, gotFtx=${event.event}"
      log.error(message)
      this.stop()
      throw IllegalStateException(message)
    }
  }

  private fun onSearchStateUpdated(prevState: EthLogsFilterState, newState: EthLogsFilterState) {
    log.info("l1 events search state updated: prevState={} newState={}", prevState, newState)
    if (newState is EthLogsFilterState.CaughtUp) {
      log.info("caught up with l1 events, releasing safe block number lock")
      safeBlockNumberManager.caughtUpWithChainHeadAfterStartUp()
    }
  }

  override fun stop(): CompletableFuture<Unit> {
    return if (this::eventsSubscription.isInitialized) {
      this.eventsSubscription.stop()
    } else {
      CompletableFuture.completedFuture(Unit)
    }
  }
}
