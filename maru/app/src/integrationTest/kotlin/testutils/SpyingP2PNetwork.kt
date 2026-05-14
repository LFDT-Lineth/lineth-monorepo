/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import maru.consensus.qbft.adapters.QbftBlockCodecAdapter
import maru.core.SealedBeaconBlock
import maru.p2p.GossipMessageType
import maru.p2p.Message
import maru.p2p.P2PNetwork
import maru.p2p.SealedBeaconBlockHandler
import maru.p2p.ValidationResult
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.consensus.common.bft.messagewrappers.BftMessage
import org.hyperledger.besu.consensus.qbft.core.messagedata.CommitMessageData
import org.hyperledger.besu.consensus.qbft.core.messagedata.PrepareMessageData
import org.hyperledger.besu.consensus.qbft.core.messagedata.ProposalMessageData
import org.hyperledger.besu.consensus.qbft.core.messagedata.RoundChangeMessageData
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import org.hyperledger.besu.ethereum.p2p.rlpx.wire.MessageData as BesuMessageData

class SpyingP2PNetwork(
  val p2pNetwork: P2PNetwork,
) : P2PNetwork by p2pNetwork {
  companion object {
    private fun Message<*, *>.toQbftMessage(): QbftMessage {
      require(this.type == GossipMessageType.QBFT) {
        "Unsupported message type: ${this.type}"
      }
      require(this.payload is QbftMessage) {
        "Message is QBFT, but its payload is of type: ${this.payload
          ?.javaClass}"
      }
      return this.payload as QbftMessage
    }
  }

  private val log = LogManager.getLogger(this.javaClass)
  val emittedQbftMessages = CopyOnWriteArrayList<BftMessage<*>>()
  val emittedBlockMessages = CopyOnWriteArrayList<SealedBeaconBlock>()

  private fun decodedMessage(message: BesuMessageData): BftMessage<*> =
    when (message) {
      is CommitMessageData -> message.decode()
      is PrepareMessageData -> message.decode()
      is ProposalMessageData -> message.decode(QbftBlockCodecAdapter)
      is RoundChangeMessageData -> message.decode(QbftBlockCodecAdapter)
      else -> throw IllegalArgumentException("Unknown message $message, don't know how to decode!")
    }

  override fun start(): SafeFuture<Unit> = SafeFuture.completedFuture(Unit)

  override fun stop(): SafeFuture<Unit> = SafeFuture.completedFuture(Unit)

  override fun broadcastMessage(message: Message<*, GossipMessageType>): SafeFuture<Unit> {
    when (message.type) {
      GossipMessageType.QBFT -> {
        val decodedMessage = decodedMessage(message.toQbftMessage().data)
        log.debug("Got new message {}", decodedMessage)
        emittedQbftMessages.add(decodedMessage)
        p2pNetwork.broadcastMessage(message)
      }

      GossipMessageType.BEACON_BLOCK -> emittedBlockMessages.add(message.payload as SealedBeaconBlock)
    }

    return SafeFuture.completedFuture(Unit)
  }

  override fun subscribeToBlocks(subscriber: SealedBeaconBlockHandler<ValidationResult>): Int =
    p2pNetwork.subscribeToBlocks(subscriber)

  override fun unsubscribeFromBlocks(subscriptionId: Int) {
    p2pNetwork.unsubscribeFromBlocks(subscriptionId)
  }

  override val port: UInt = 0u
}
