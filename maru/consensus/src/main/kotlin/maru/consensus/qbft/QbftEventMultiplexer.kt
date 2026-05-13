/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.consensus.common.bft.events.BftEvent
import org.hyperledger.besu.consensus.common.bft.events.BftEvents
import org.hyperledger.besu.consensus.common.bft.events.BlockTimerExpiry
import org.hyperledger.besu.consensus.common.bft.events.RoundExpiry
import org.hyperledger.besu.consensus.qbft.core.types.QbftEventHandler
import org.hyperledger.besu.consensus.qbft.core.types.QbftNewChainHead
import org.hyperledger.besu.consensus.qbft.core.types.QbftReceivedMessageEvent

class QbftEventMultiplexer(
  private val eventHandler: QbftEventHandler,
) {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  /**
   * Optional observer called immediately when BLOCK_TIMER_EXPIRY fires, before the event is
   * dispatched to the QBFT state machine. Parameters: (blockNumber).
   * Intended for benchmarking/testing — the callback implementer captures timestamps themselves.
   */
  var onBlockTimerFired: ((blockNumber: Long) -> Unit)? = null

  fun handleEvent(event: BftEvent) {
    try {
      log.trace("Received event type: {}, event: {},", event.type, event)
      when (event.type) {
        BftEvents.Type.ROUND_EXPIRY -> {
          eventHandler.handleRoundExpiry(event as RoundExpiry)
        }

        BftEvents.Type.NEW_CHAIN_HEAD -> {
          eventHandler.handleNewBlockEvent(event as QbftNewChainHead)
        }

        BftEvents.Type.BLOCK_TIMER_EXPIRY -> {
          val timerExpiry = event as BlockTimerExpiry
          onBlockTimerFired?.invoke(timerExpiry.roundIdentifier.sequenceNumber)
          eventHandler.handleBlockTimerExpiry(timerExpiry)
        }

        BftEvents.Type.MESSAGE -> {
          eventHandler.handleMessageEvent(event as QbftReceivedMessageEvent)
        }

        else -> {
          throw IllegalStateException("Unhandled event type: ${event.type}")
        }
      }
    } catch (e: Exception) {
      log.error("State machine threw exception while processing event \\{$event\\}", e)
    }
  }
}
