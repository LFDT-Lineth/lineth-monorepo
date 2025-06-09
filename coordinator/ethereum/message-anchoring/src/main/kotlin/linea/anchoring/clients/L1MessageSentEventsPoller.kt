package linea.anchoring.clients

import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.contract.events.MessageSentEvent
import linea.contract.l2.L2MessageServiceSmartContractClient
import linea.domain.BlockParameter
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Deque
import java.util.concurrent.atomic.AtomicLong
import kotlin.time.Duration

class L1MessageSentEventsPoller(
  vertx: Vertx,
  pollingInterval: Duration,
  private val l1SmartContractAddress: String,
  private val l1EventsSearcher: EthLogsSearcher,
  private val eventsQueue: Deque<MessageSentEvent>,
  private val eventsQueueMaxCapacity: Int,
  private val l2MessageService: L2MessageServiceSmartContractClient,
  private val l1MessagesSentFetchLimit: UInt,
  private val l1MessagesSentFetchTimeout: Duration,
  private val l1BlockSearchChuck: UInt,
  private val l1HighestBlock: BlockParameter,
  private val l2HighestBlock: BlockParameter,
  private val log: Logger = LogManager.getLogger(L1MessageSentEventsPoller::class.java),
) : PeriodicPollingService(
  vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
) {
  private val eventsFetcher = L1MessageSentEventsFetcher(
    l1SmartContractAddress = l1SmartContractAddress,
    l1EventsSearcher = l1EventsSearcher,
    l1HighestBlock = l1HighestBlock,
    log = log,
  )
  private val lastFetchedMessageNumber: AtomicLong = AtomicLong(0L)

  private fun nextMessageNumberToFetchFromL1(): SafeFuture<ULong> {
    if (lastFetchedMessageNumber.get() > 0) {
      return SafeFuture.completedFuture(lastFetchedMessageNumber.get().inc().toULong())
    } else {
      return l2MessageService
        .getLastAnchoredL1MessageNumber(block = l2HighestBlock)
        .thenApply {
          lastFetchedMessageNumber.set(it.toLong())
          it.inc()
        }
    }
  }

  private fun queueRemainingCapacity(): Int {
    return (eventsQueueMaxCapacity - eventsQueue.size).coerceAtLeast(0)
  }

  override fun action(): SafeFuture<*> {
    val remainingCapacity = queueRemainingCapacity()

    if (remainingCapacity == 0) {
      log.debug(
        "skipping fetching MessageSent events: queueSize={} reached targetCapacity={}",
        eventsQueue.size,
        eventsQueueMaxCapacity,
      )
      return SafeFuture.completedFuture(null)
    }

    return nextMessageNumberToFetchFromL1()
      .thenCompose { nextMessageNumberToFetchFromL1 ->
        eventsFetcher.findL1MessageSentEvents(
          startingMessageNumber = nextMessageNumberToFetchFromL1,
          targetMessagesToFetch = l1MessagesSentFetchLimit.coerceAtMost(remainingCapacity.toUInt()),
          fetchTimeout = l1MessagesSentFetchTimeout,
          blockChunkSize = l1BlockSearchChuck,
        )
      }
      .thenApply { events ->
        eventsQueue.addAll(events.map { it.event })
        events.lastOrNull()?.also { lastFetchedMessageNumber.set(it.event.messageNumber.toLong()) }
      }
  }
}
