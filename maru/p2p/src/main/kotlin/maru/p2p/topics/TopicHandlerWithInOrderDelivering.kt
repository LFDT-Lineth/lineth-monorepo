/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.topics

import maru.p2p.LINEA_DOMAIN
import maru.p2p.MaruPreparedGossipMessage
import maru.p2p.SubscriptionManager
import maru.p2p.toLibP2P
import maru.serialization.Deserializer
import maru.serialization.MAX_MESSAGE_SIZE
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import tech.pegasys.teku.networking.p2p.gossip.PreparedGossipMessage
import tech.pegasys.teku.networking.p2p.gossip.TopicHandler
import java.util.Optional
import java.util.PriorityQueue
import io.libp2p.core.pubsub.ValidationResult as Libp2pValidationResult

fun interface SequenceNumberExtractor<T> {
  fun extractSequenceNumber(event: T): ULong
}

/**
 * Topic handler which triggers event handling only in case there's a "next" event, defined by its sequence number
 *
 * When there's a P2P message that is from the future, it will be passed to subscriptionManager later, when all the
 * previous events are handled
 * Messages behind the current expected sequence number are not validated and ignored.
 *
 * Note that messages ahead of the current expected sequence number won't be propagated over the network until they're
 * handled
 * @param sequenceNumberExtractor definition of sequentiality for T
 */
class TopicHandlerWithInOrderDelivering<T>(
  private val subscriptionManager: SubscriptionManager<T>,
  private val deserializer: Deserializer<T>,
  private val sequenceNumberExtractor: SequenceNumberExtractor<T>,
  private val topicId: String,
  private val maxQueueSize: Int = 1000,
  private val isHandlingEnabled: () -> Boolean,
  private val nextExpectedSequenceNumberProvider: () -> ULong,
) : TopicHandler {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  private val pendingEvents =
    run {
      val comparator: Comparator<Pair<T, SafeFuture<Libp2pValidationResult>>> =
        Comparator.comparing {
          sequenceNumberExtractor.extractSequenceNumber(it.first)
        }
      PriorityQueue<Pair<T, SafeFuture<Libp2pValidationResult>>>(comparator)
    }

  override fun prepareMessage(
    payload: Bytes,
    arrivalTimestamp: Optional<UInt64>,
  ): PreparedGossipMessage =
    MaruPreparedGossipMessage(
      origMessage = payload,
      arrTimestamp = arrivalTimestamp,
      domain = LINEA_DOMAIN,
      topicId = topicId,
    )

  @Synchronized
  override fun handleMessage(message: PreparedGossipMessage): SafeFuture<Libp2pValidationResult> =
    try {
      val deserializedMessage = deserializer.deserialize(message.originalMessage.toArray())
      val sequenceNumber = sequenceNumberExtractor.extractSequenceNumber(deserializedMessage)
      val nextExpectedSequenceNumber = nextExpectedSequenceNumberProvider()

      when {
        sequenceNumber >= nextExpectedSequenceNumber -> {
          if (pendingEvents.size < maxQueueSize) {
            log.trace(
              "enqueuing message with sequenceNumber={} next expectedSequenceNumber={}",
              sequenceNumber,
              nextExpectedSequenceNumber,
            )
          } else {
            // Queue is full - drop the oldest message and add the new one
            val oldestMessage = pendingEvents.remove()
            if (oldestMessage != null) {
              oldestMessage.second.complete(Libp2pValidationResult.Ignore)
              log.debug(
                "Dropped oldest message with sequenceNumber={} to make room for new message with sequenceNumber={}",
                sequenceNumberExtractor.extractSequenceNumber(oldestMessage.first),
                sequenceNumber,
              )
            }
          }
          addMessageToTheQueue(deserializedMessage)
        }
        else -> {
          log.debug(
            "ignoring outdated message with sequenceNumber={} next expectedSequenceNumber={}",
            sequenceNumber,
            nextExpectedSequenceNumber,
          )
          SafeFuture.completedFuture(Libp2pValidationResult.Ignore)
        }
      }.also {
        processPendingEvents()
      }
    } catch (th: Throwable) {
      log.error("Unexpected exception while handling message=$message with id=${message.messageId}", th)
      SafeFuture.completedFuture(Libp2pValidationResult.Invalid)
    }

  private fun addMessageToTheQueue(deserializedMessage: T): SafeFuture<Libp2pValidationResult> {
    val delayedHandlingFuture = SafeFuture<Libp2pValidationResult>()
    pendingEvents.add(deserializedMessage to delayedHandlingFuture)
    // Note that it will be completed only when it's handled
    return delayedHandlingFuture
  }

  // Not synchronized, because it's only being called from the synchronized handleMessage
  private fun cleanUpMessagesBehind(nextExpectedSequenceNumber: ULong) {
    val sizeBefore = pendingEvents.size
    while (pendingEvents.isNotEmpty() &&
      sequenceNumberExtractor.extractSequenceNumber(pendingEvents.peek().first) <
      nextExpectedSequenceNumber
    ) {
      val futureToComplete = pendingEvents.remove().second
      futureToComplete.complete(Libp2pValidationResult.Ignore)
    }

    val eventsRemoved = sizeBefore - pendingEvents.size
    if (eventsRemoved > 0) {
      log.debug(
        "Cleaned up {} old messages that are now behind expectedSequenceNumber={}",
        eventsRemoved,
        nextExpectedSequenceNumber,
      )
    }
  }

  @Synchronized
  private fun processPendingEvents() {
    cleanUpMessagesBehind(nextExpectedSequenceNumberProvider())
    if (pendingEvents.isNotEmpty() &&
      isHandlingEnabled() &&
      sequenceNumberExtractor.extractSequenceNumber(pendingEvents.peek().first) ==
      nextExpectedSequenceNumberProvider()
    ) {
      val (nextEventToHandle, future) = pendingEvents.remove()
      handleEvent(nextEventToHandle)
        .whenSuccess { processPendingEvents() }
        .propagateTo(future)
    }
  }

  private fun handleEvent(event: T): SafeFuture<Libp2pValidationResult> =
    runCatching {
      subscriptionManager
        .handleEvent(event)
        .thenApply {
          it.code.toLibP2P()
        }.exceptionally {
          log.warn(
            "error handling message with sequenceNumber={} errorMessage={}",
            sequenceNumberExtractor.extractSequenceNumber(event),
            it.message,
          )
          Libp2pValidationResult.Invalid
        }
    }.getOrElse(
      { th ->
        log.warn(
          "error handling message sequenceNumber={} errorMessage={}",
          sequenceNumberExtractor.extractSequenceNumber(event),
          th.message,
        )
        SafeFuture.completedFuture(Libp2pValidationResult.Invalid)
      },
    )

  override fun getMaxMessageSize(): Int = MAX_MESSAGE_SIZE
}
