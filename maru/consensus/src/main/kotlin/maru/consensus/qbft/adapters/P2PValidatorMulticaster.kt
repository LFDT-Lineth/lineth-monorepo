/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import io.libp2p.pubsub.MessageAlreadySeenException
import io.libp2p.pubsub.NoPeersForOutboundMessageException
import maru.p2p.P2PNetwork
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.consensus.common.bft.network.ValidatorMulticaster
import org.hyperledger.besu.datatypes.Address
import tech.pegasys.teku.infrastructure.async.SafeFuture
import org.hyperledger.besu.ethereum.p2p.rlpx.wire.MessageData as BesuMessageData

/**
 * Adapter that implements the Hyperledger Besu ValidatorMulticaster interface and delegates to a P2PNetwork.
 *
 * This adapter is used by the QBFT consensus protocol to send messages to validators.
 *
 * The send waits for the broadcast future to complete. Missing peers and already-seen messages
 * are benign libp2p outcomes and are not propagated. Other broadcast failures propagate to the
 * caller.
 */
class P2PValidatorMulticaster(
  private val p2pNetwork: P2PNetwork,
) : ValidatorMulticaster {
  private val log = LogManager.getLogger(this.javaClass)

  /**
   * Send a message to all connected validators.
   *
   * @param message The message to send.
   */
  override fun send(message: BesuMessageData) {
    @Suppress("UNCHECKED_CAST")
    (p2pNetwork.broadcastMessage(message.toDomain()) as SafeFuture<Any?>)
      .exceptionally { e ->
        when {
          hasCause<NoPeersForOutboundMessageException>(e) ->
            log.debug("No gossip peers subscribed to QBFT topic, message not delivered")
          hasCause<MessageAlreadySeenException>(e) ->
            log.debug("QBFT gossip message was already seen, message not re-published")
          else -> throw e
        }
        null
      }.get()
  }

  /**
   * Send a message to all connected validators except those in the denyList.
   *
   * @param message The message to send.
   * @param denyList This becomes irrelevant because it's a broadcasting under the hood, but needs to be there for the
   * completeness of the interface
   */
  override fun send(
    message: BesuMessageData,
    denyList: Collection<Address>,
  ) {
    send(message)
  }

  private inline fun <reified T : Throwable> hasCause(e: Throwable): Boolean {
    var current: Throwable? = e
    while (current != null) {
      if (current is T) return true
      current = current.cause
    }
    return false
  }
}
