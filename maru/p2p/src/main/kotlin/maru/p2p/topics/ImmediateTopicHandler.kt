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
import io.libp2p.core.pubsub.ValidationResult as Libp2pValidationResult

/**
 * Topic handler that immediately triggers event handling without any queuing.
 * This handler always processes messages immediately and does not maintain any ordering guarantees.
 */
class ImmediateTopicHandler<T>(
  private val subscriptionManager: SubscriptionManager<T>,
  private val deserializer: Deserializer<T>,
  private val topicId: String,
) : TopicHandler {
  private val log: Logger = LogManager.getLogger(this.javaClass)

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

  override fun handleMessage(message: PreparedGossipMessage): SafeFuture<Libp2pValidationResult> =
    try {
      val deserializedMessage = deserializer.deserialize(message.originalMessage.toArray())
      log.trace("Processing message: {}", deserializedMessage)

      handleEvent(deserializedMessage)
    } catch (th: Throwable) {
      log.error("Unexpected exception while handling message=$message with id=${message.messageId}", th)
      SafeFuture.completedFuture(Libp2pValidationResult.Invalid)
    }

  private fun handleEvent(event: T): SafeFuture<Libp2pValidationResult> =
    runCatching {
      subscriptionManager
        .handleEvent(event)
        .thenApply {
          it.code.toLibP2P()
        }.exceptionally {
          log.warn(
            "error handling message={} errorMessage={}",
            event,
            it.message,
          )
          Libp2pValidationResult.Invalid
        }
    }.getOrElse(
      { th ->
        log.warn(
          "error handling message={} errorMessage={}",
          event,
          th.message,
        )
        SafeFuture.completedFuture(Libp2pValidationResult.Invalid)
      },
    )

  override fun getMaxMessageSize(): Int = MAX_MESSAGE_SIZE
}
