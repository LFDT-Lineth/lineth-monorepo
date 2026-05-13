/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import io.libp2p.core.pubsub.ValidationResult.Ignore
import io.libp2p.core.pubsub.ValidationResult.Invalid
import io.libp2p.core.pubsub.ValidationResult.Valid
import maru.consensus.ForkSpec
import maru.core.SealedBeaconBlock
import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import org.ethereum.beacon.discovery.schema.NodeRecord
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.p2p.peer.NodeId
import java.io.Closeable

const val LINEA_DOMAIN = "linea"

// Mimicking ValidationResultCode for P2P communication
enum class ValidationResultCode {
  ACCEPT,
  REJECT,
  IGNORE,
}

fun ValidationResultCode.toLibP2P() =
  when (this) {
    ValidationResultCode.ACCEPT -> Valid
    ValidationResultCode.REJECT -> Invalid
    ValidationResultCode.IGNORE -> Ignore
  }

sealed interface ValidationResult {
  val code: ValidationResultCode

  companion object {
    object Valid : ValidationResult {
      override val code = ValidationResultCode.ACCEPT
    }

    data class Invalid(
      val error: String,
      val cause: Throwable? = null,
    ) : ValidationResult {
      override val code = ValidationResultCode.REJECT
    }

    data class Ignore(
      val comment: String,
    ) : ValidationResult {
      override val code = ValidationResultCode.IGNORE
    }

    fun fromForkChoiceUpdatedResult(forkChoiceUpdatedResult: ForkChoiceUpdatedResult): ValidationResult {
      val payloadStatus = forkChoiceUpdatedResult.payloadStatus
      return when (payloadStatus.status) {
        ExecutionPayloadStatus.VALID -> Valid
        ExecutionPayloadStatus.INVALID -> Invalid(payloadStatus.validationError!!)
        else -> Ignore("Payload status is ${payloadStatus.status}")
      }
    }
  }
}

fun interface SealedBeaconBlockHandler<T> {
  fun handleSealedBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<T>
}

fun interface QbftMessageHandler<T> {
  fun handleQbftMessage(qbftMessage: QbftMessage): SafeFuture<T>
}

/**
 * Interface for the P2P Network functionality.
 *
 * This network is used to:
 * 1. Exchange new blocks between nodes
 * 2. Sync new nodes joining the network
 * 3. Exchange QBFT messages between QBFT Validators
 */
interface P2PNetwork : Closeable {
  /**
   * Start the P2P network service.
   *
   * @return A future that completes when the service is started.
   */
  fun start(): SafeFuture<Unit>

  /**
   * Stop the P2P network service.
   *
   * @return A future that completes when the service is stopped.
   */
  fun stop(): SafeFuture<Unit>

  fun broadcastMessage(message: Message<*, GossipMessageType>): SafeFuture<*>

  /**
   * @return subscription id
   */
  fun subscribeToBlocks(subscriber: SealedBeaconBlockHandler<ValidationResult>): Int

  fun unsubscribeFromBlocks(subscriptionId: Int)

  /**
   * @return subscription id
   */
  fun subscribeToQbftMessages(subscriber: QbftMessageHandler<ValidationResult>): Int

  fun unsubscribeFromQbftMessages(subscriptionId: Int)

  val port: UInt

  val nodeId: String

  val nodeAddresses: List<String>

  val discoveryAddresses: List<String>

  val localNodeRecord: NodeRecord?

  val enr: String?

  val peerCount: Int

  fun getPeers(): List<PeerInfo>

  fun getPeer(peerId: String): PeerInfo?

  fun getPeerLookup(): PeerLookup

  fun dropPeer(peer: PeerInfo)

  fun addPeer(address: String)

  fun handleForkTransition(forkSpec: ForkSpec): Unit

  fun isStaticPeer(nodeId: NodeId): Boolean
}

data class PeerInfo(
  val nodeId: String,
  val enr: String?,
  val address: String,
  val status: PeerStatus,
  val direction: PeerDirection,
) {
  enum class PeerStatus {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    DISCONNECTING,
  }

  enum class PeerDirection {
    INBOUND,
    OUTBOUND,
  }
}
