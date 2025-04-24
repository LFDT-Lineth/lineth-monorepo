package linea.anchoring.clients

import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.anchoring.events.MessageSentEvent
import linea.contract.l2.L2MessageServiceSmartContractClient
import linea.domain.BlockParameter
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.PriorityBlockingQueue
import kotlin.time.Duration

class L1MessageSentEventsPoller(
  vertx: Vertx,
  pollingInterval: Duration,
  private val l1SmartContractAddress: String,
  private val l1EventsSearcher: EthLogsSearcher,
  private val eventsQueue: PriorityBlockingQueue<MessageSentEvent>,
  private val eventsQueueMaxCapacity: Int,
  private val l2MessageService: L2MessageServiceSmartContractClient,
  private val l1MessagesSentFetchLimit: UInt,
  private val l1MessagesSentFetchTimeout: Duration,
  private val l1BlockSearchChuck: UInt,
  private val highestBlockNumber: BlockParameter,
  private val log: Logger = LogManager.getLogger(L1MessageSentEventsPoller::class.java)
) : PeriodicPollingService(
  vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log
) {
  private val eventsFetcher = L1MessageSentEventsFetcher(
    l1SmartContractAddress = l1SmartContractAddress,
    l1EventsSearcher = l1EventsSearcher,
    highestBlock = highestBlockNumber,
    log = log
  )

  private fun nextMessageNumberToFetchFromL1(): SafeFuture<ULong> {
    val queueLastMessage = eventsQueue.lastOrNull()
    if (queueLastMessage != null) {
      return SafeFuture.completedFuture(queueLastMessage.messageNumber.inc())
    } else {
      return l2MessageService.getLastAnchoredL1MessageNumber(block = highestBlockNumber)
        .thenApply { it.inc() }
    }
  }

  private fun queueRemainingCapacity(): Int {
    return (eventsQueueMaxCapacity - eventsQueue.size).coerceAtLeast(0)
  }

  override fun action(): SafeFuture<*> {
    val remainingCapacity = queueRemainingCapacity()

    if (remainingCapacity == 0) {
      log.debug("MessageSent event queue is full, skipping fetching new events")
      return SafeFuture.completedFuture(null)
    }

    return nextMessageNumberToFetchFromL1()
      .thenCompose { nextMessageNumberToFetchFromL1 ->
        eventsFetcher.findL1MessageSentEvents(
          startingMessageNumber = nextMessageNumberToFetchFromL1,
          messagesToFetch = l1MessagesSentFetchLimit.coerceAtMost(remainingCapacity.toUInt()),
          fetchTimeout = l1MessagesSentFetchTimeout,
          blockChunkSize = l1BlockSearchChuck
        )
      }
      .thenApply { events ->
        eventsQueue.addAll(events.map { it.event })
      }
  }
}
