/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.consensus.ForkSpec
import org.apache.logging.log4j.LogManager
import org.ethereum.beacon.discovery.schema.NodeRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.p2p.peer.NodeId
import java.util.UUID

object NoOpP2PNetwork : P2PNetwork {
  private val log = LogManager.getLogger(this.javaClass)

  override fun start(): SafeFuture<Unit> =
    SafeFuture
      .fromRunnable {
        log.debug("NoopP2PNetwork started")
      }.thenApply { }

  override fun stop(): SafeFuture<Unit> =
    SafeFuture
      .fromRunnable {
        log.debug("NoopP2PNetwork stopped")
      }.thenApply { }

  override fun broadcastMessage(message: Message<*, GossipMessageType>): SafeFuture<Unit> {
    log.debug("Doing nothing for message={}", message)
    return SafeFuture.completedFuture(Unit)
  }

  override fun subscribeToBlocks(subscriber: SealedBeaconBlockHandler<ValidationResult>): Int {
    log.debug("Subscription called for subscriber={}", subscriber)
    return 0
  }

  override fun unsubscribeFromBlocks(subscriptionId: Int) {
    log.debug("Unsubscription called for subscriptionId={}", subscriptionId)
  }

  override fun subscribeToQbftMessages(subscriber: QbftMessageHandler<ValidationResult>): Int {
    log.debug("QBFT subscription called for subscriber={}", subscriber)
    return 0
  }

  override fun unsubscribeFromQbftMessages(subscriptionId: Int) {
    log.debug("QBFT unsubscription called for subscriptionId={}", subscriptionId)
  }

  override val port: UInt
    get() = 0u

  override val nodeId: String = UUID.randomUUID().toString()
  override val nodeAddresses: List<String> = emptyList()
  override val discoveryAddresses: List<String> = emptyList()
  override val localNodeRecord: NodeRecord?
    get() = null
  override val enr: String? = null
  override val peerCount: Int = 0

  override fun getPeers(): List<PeerInfo> = emptyList()

  override fun getPeer(peerId: String): PeerInfo? = null

  override fun getPeerLookup(): PeerLookup {
    log.debug("Get peer lookup")
    return object : PeerLookup {
      override fun getPeer(nodeId: NodeId): MaruPeer? {
        log.debug("NoOpP2PNetwork.getPeer called for nodeId={}", nodeId)
        return null
      }

      override fun getPeers(): List<MaruPeer> {
        log.debug("NoOpP2PNetwork.getPeers called")
        return emptyList()
      }
    }
  }

  override fun dropPeer(peer: PeerInfo) = Unit

  override fun addPeer(address: String) = Unit

  override fun handleForkTransition(forkSpec: ForkSpec) = Unit

  override fun isStaticPeer(nodeId: NodeId): Boolean = false

  override fun close() {
  }
}
