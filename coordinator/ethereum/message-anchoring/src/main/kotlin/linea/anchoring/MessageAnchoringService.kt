package linea.anchoring

import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.SearchDirection
import linea.contract.events.L1RollingHashUpdatedEvent
import linea.contract.events.MessageSentEvent
import linea.contract.l2.L2MessageServiceSmartContractClient
import linea.domain.BlockParameter
import linea.domain.CommonDomainFunctions
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Queue
import kotlin.time.Duration

class MessageAnchoringService(
  vertx: Vertx,
  private val l1ContractAddress: String,
  private val l1EthLogsSearcher: EthLogsSearcher,
  private val l2MessageService: L2MessageServiceSmartContractClient,
  private val eventsQueue: Queue<MessageSentEvent>,
  private val maxMessagesToAnchorPerL2Transaction: UInt,
  private val l2HighestBlockTag: BlockParameter,
  private val l1EventSearchMaxBlockRange: UInt,
  anchoringTickInterval: Duration,
  private val log: Logger = LogManager.getLogger(MessageAnchoringService::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = anchoringTickInterval.inWholeMilliseconds,
  log = log,
  name = "MessageAnchoringService",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  override fun action(): SafeFuture<*> {
    if (eventsQueue.isEmpty()) {
      log.trace("No messages in the queue to anchor")
      return SafeFuture.completedFuture(null)
    }
    return l2MessageService
      .getLastAnchoredL1MessageNumber(block = l2HighestBlockTag)
      .thenApply { lastAnchoredL1MessageNumber ->

        // clean up the queue of events that are already anchored
        eventsQueue.removeIf { it.messageNumber <= lastAnchoredL1MessageNumber }

        eventsQueue
          .toTypedArray()
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
      eventsToAnchor.last().messageNumber,
    )
    log.debug("sending anchoring tx messagesNumbers={}", messagesInterval)

    return getRollingHash(messageNumber = eventsToAnchor.last().messageNumber)
      .thenCompose { rollingHash ->
        l2MessageService
          .anchorL1L2MessageHashes(
            messageHashes = eventsToAnchor.map { it.messageHash },
            startingMessageNumber = eventsToAnchor.first().messageNumber,
            finalMessageNumber = eventsToAnchor.last().messageNumber,
            finalRollingHash = rollingHash,
          )
      }.thenPeek { txHash ->
        log.info("sent anchoring tx messagesNumbers={} txHash={}", messagesInterval, txHash)
      }
  }

  private fun getRollingHash(messageNumber: ULong): SafeFuture<ByteArray> {
    return l1EthLogsSearcher
      .findLog(
        fromBlock = BlockParameter.Tag.EARLIEST,
        toBlock = BlockParameter.Tag.LATEST,
        chunkSize = l1EventSearchMaxBlockRange.toInt(),
        address = l1ContractAddress,
        topics = listOf(L1RollingHashUpdatedEvent.topic),
      ) { ethLog ->
        val foundMessageNumber = L1RollingHashUpdatedEvent.fromEthLog(ethLog).event.messageNumber
        when {
          foundMessageNumber < messageNumber -> SearchDirection.FORWARD
          foundMessageNumber > messageNumber -> SearchDirection.BACKWARD
          else -> null
        }
      }
      .thenApply { ethLog ->
        requireNotNull(ethLog) {
          "Expected exactly 1 event RollingHashUpdated(messageNumber=$messageNumber) but got none."
        }
        L1RollingHashUpdatedEvent.fromEthLog(ethLog).event.rollingHash
      }
  }
}
