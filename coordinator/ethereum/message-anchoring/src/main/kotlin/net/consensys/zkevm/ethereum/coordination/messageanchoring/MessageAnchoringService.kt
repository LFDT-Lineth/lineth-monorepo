package net.consensys.zkevm.ethereum.coordination.messageanchoring

import io.vertx.core.Vertx
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.BlockParameter
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.L2MessageService
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import kotlin.time.Duration

class MessageAnchoringService(
  private val config: Config,
  private val vertx: Vertx,
  private val l1EventQuerier: L1EventQuerier,
  private val l2MessageAnchorer: L2MessageAnchorer,
  private val l2Querier: L2Querier,
  private val lineaRollupSmartContractClient: LineaRollupSmartContractClientReadOnly,
  l2MessageService: L2MessageService,
  private val transactionManager: AsyncFriendlyTransactionManager,
  private val log: Logger = LogManager.getLogger(MessageAnchoringService::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log
) {
  class Config(
    val pollingInterval: Duration,
    val maxMessagesToAnchor: UInt
  )

  private val inboxStatusUnknown: BigInteger = l2MessageService.INBOX_STATUS_UNKNOWN().send()

  override fun action(): SafeFuture<Unit> {
    return l2Querier
      .findLastFinalizedAnchoredEvent()
      .thenCompose(l1EventQuerier::getSendMessageEventsForAnchoredMessage)
      .thenCompose { sentMessages ->
        SafeFuture.collectAll(
          sentMessages
            .map { sendMessageEvent ->
              l2Querier.getMessageHashStatus(sendMessageEvent.messageHash).thenApply { status
                ->
                sendMessageEvent to status
              }
            }
            .stream()
        )
      }
      .thenApply { eventAndStatusPairs ->
        eventAndStatusPairs
          .filter { eventAndStatus -> eventAndStatus.second == inboxStatusUnknown }
          .map { it.first }
      }
      .thenCompose { eventsToAnchor ->
        if (eventsToAnchor.isNotEmpty()) {
          log.debug("Found {} un-anchored events", eventsToAnchor.count())

          val messagesToAnchorWithinLimit = eventsToAnchor.take(config.maxMessagesToAnchor.toInt())

          anchorMessagesUsingRollingHashProtocol(messagesToAnchorWithinLimit)
        } else {
          log.debug("Skipping anchoring as there are no hashes")
          SafeFuture.completedFuture(Unit)
        }
      }.exceptionally(::handleAnchoringError)
  }

  private fun anchorMessagesUsingRollingHashProtocol(messagesEvents: List<SendMessageEvent>): SafeFuture<Unit> {
    return lineaRollupSmartContractClient.getMessageRollingHash(
      blockParameter = BlockParameter.Tag.LATEST,
      messageNumber = messagesEvents.last().messageNumber.toLong()
    ).thenCompose { finalRollingHash ->
      transactionManager.resetNonce().thenCompose {
        l2MessageAnchorer.anchorMessages(
          messagesEvents,
          finalRollingHash
        )
      }.thenApply { anchoringResult ->
        log.info("Message anchoring using rolling hash transactionHash=${anchoringResult.transactionHash}")
      }
    }
  }

  private fun handleAnchoringError(error: Throwable) {
    when {
      (
        error.message != null && (
          error.message!!.contains("replacement transaction underpriced") ||
            error.message!!.contains("already known")
          )
        ) ->
        log.debug("Anchoring transaction wasn't executed due to ${error.message}")

      else ->
        // Since anchoring is stateless and will be retried on the next iteration of the loop, no error is considered
        // as unrecoverable, thus logged as warning, not ERROR. But we still want to see them, thus not DEBUG
        log.warn("Anchoring attempt failed! Anchoring will be re-attempted shortly.", error)
    }
  }

  override fun handleError(error: Throwable) {
    log.error("Failed to anchor messages: errorMessage={}", error.message, error)
  }
}
