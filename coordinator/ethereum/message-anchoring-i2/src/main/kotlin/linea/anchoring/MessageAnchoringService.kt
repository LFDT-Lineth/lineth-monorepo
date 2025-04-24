package linea.anchoring

import io.vertx.core.Vertx
import linea.anchoring.events.L1RollingHashUpdatedEvent
import linea.anchoring.events.MessageSentEvent
import linea.contract.l2.L2MessageServiceSmartContractClient
import linea.domain.BlockParameter
import linea.domain.CommonDomainFunctions
import linea.ethapi.EthLogsClient
import linea.kotlin.toHexStringUInt256
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.PriorityBlockingQueue
import kotlin.time.Duration

class MessageAnchoringService(
  private val vertx: Vertx,
  private val l1ContractAddress: String,
  private val l1EthLogsClient: EthLogsClient,
  private val l2MessageService: L2MessageServiceSmartContractClient,
  private val eventsQueue: PriorityBlockingQueue<MessageSentEvent>,
  private val maxMessagesToAnchorPerL2Transaction: UInt,
  private val l2HighestBlockTag: BlockParameter,
  anchoringTickInterval: Duration,
  private val log: Logger = LogManager.getLogger(MessageAnchoringService::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = anchoringTickInterval.inWholeMilliseconds,
  log = log
) {
  override fun action(): SafeFuture<*> {
    return l2MessageService
      .getLastAnchoredL1MessageNumber(block = l2HighestBlockTag)
      .thenApply { lastAnchoredL1MessageNumber ->

        // clean up the queue of events that are already anchored
        eventsQueue.removeIf { it.messageNumber <= lastAnchoredL1MessageNumber }

        eventsQueue
          .toArray(emptyArray<MessageSentEvent>())
          .filter { it.messageNumber > lastAnchoredL1MessageNumber }
          // needs sorting because PriorityBlockingQueue#toArray does not guarantee order
          .sortedBy { it.messageNumber }
          .take(maxMessagesToAnchorPerL2Transaction.toInt())
      }.thenCompose { eventsToAnchor ->
        if (eventsToAnchor.isEmpty()) {
          log.trace("No messages to anchor")
          SafeFuture.completedFuture(null)
        } else {
          anchorMessages(eventsToAnchor)
        }
      }
  }

  private fun anchorMessages(eventsToAnchor: List<MessageSentEvent>): SafeFuture<String> {
    val messagesInterval = CommonDomainFunctions.blockIntervalString(
      eventsToAnchor.first().messageNumber,
      eventsToAnchor.last().messageNumber
    )
    log.debug("sending anchoring tx messagesNumbers={}", messagesInterval)

    return getRollingHash(messageNumber = eventsToAnchor.last().messageNumber)
      .thenCompose { rollingHash ->
        l2MessageService
          .anchorL1L2MessageHashes(
            messageHashes = eventsToAnchor.map { it.messageHash },
            startingMessageNumber = eventsToAnchor.first().messageNumber,
            finalMessageNumber = eventsToAnchor.last().messageNumber,
            finalRollingHash = rollingHash
          )
      }.thenPeek { txHash ->
        log.info("sent anchoring tx messagesNumbers={} txHash={}", messagesInterval, txHash)
      }
  }

  private fun getRollingHash(messageNumber: ULong): SafeFuture<ByteArray> {
    return l1EthLogsClient
      .getLogs(
        // RollingHashUpdated event has message number indexed and unique
        // so we can query the whole chain
        fromBlock = BlockParameter.Tag.EARLIEST,
        toBlock = BlockParameter.Tag.LATEST,
        address = l1ContractAddress,
        topics = listOf(
          L1RollingHashUpdatedEvent.topic,
          messageNumber.toHexStringUInt256()
        )
      )
      .thenApply { rawLogs ->
        val events = rawLogs.map(L1RollingHashUpdatedEvent::fromEthLog)
        require(events.size == 1) {
          "Expected exactly 1 event RollingHashUpdated(messageNumber=$messageNumber) " +
            "but got ${events.size} events. events=$events"
        }
        events.first().event.rollingHash
      }
  }
}
