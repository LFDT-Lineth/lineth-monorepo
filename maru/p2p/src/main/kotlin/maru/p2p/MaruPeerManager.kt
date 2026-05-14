/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.config.P2PConfig
import maru.p2p.discovery.MaruDiscoveryService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.networking.p2p.network.P2PNetwork
import tech.pegasys.teku.networking.p2p.network.PeerHandler
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.peer.Peer
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class MaruPeerManager(
  private val maruPeerFactory: MaruPeerFactory,
  private val p2pConfig: P2PConfig,
  private val reputationManager: MaruReputationManager,
  private val isStaticPeer: (NodeId) -> Boolean,
) : PeerHandler,
  PeerLookup {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  private val maxPeers = p2pConfig.maxPeers

  private var discoveryService: MaruDiscoveryService? = null
  private var discoveryTask: PeerDiscoveryTask? = null

  private val peers: ConcurrentHashMap<NodeId, MaruPeer> = ConcurrentHashMap()

  private var scheduler: ScheduledExecutorService? = null

  val peerCount: Int
    get() = connectedPeers().size

  @Volatile
  private var started = AtomicBoolean(false)

  fun start(
    discoveryService: MaruDiscoveryService?,
    p2pNetwork: P2PNetwork<Peer>,
  ) {
    if (!started.compareAndSet(false, true)) {
      log.warn("Trying to start already started MaruPeerManager")
      return
    }
    this.discoveryService = discoveryService
    scheduler = Executors.newSingleThreadScheduledExecutor(Thread.ofPlatform().daemon().factory())
    scheduler!!.scheduleAtFixedRate({
      logConnectedPeers()
    }, 20000, 20000, TimeUnit.MILLISECONDS)
    if (discoveryService != null) {
      discoveryTask = PeerDiscoveryTask(
        discoveryService = discoveryService,
        p2pNetwork = p2pNetwork,
        reputationManager = reputationManager,
        maxPeers = maxPeers,
        getPeerCount = { peerCount },
        discoveryConfig = p2pConfig.discovery!!,
      )
      discoveryTask!!.start()
    }
  }

  fun stop() {
    if (!started.compareAndSet(true, false)) {
      log.warn("Trying to stop stopped MaruPeerManager")
      return
    }
    discoveryTask?.stop()
    discoveryTask = null
    scheduler!!.shutdown()
    scheduler = null
    peers.values.forEach { it.disconnectCleanly(DisconnectReason.SHUTTING_DOWN) }
  }

  private fun logConnectedPeers() {
    log.info("Currently connected peers={}", connectedPeers().keys.toList())
    if (log.isDebugEnabled) {
      discoveryService?.getKnownPeers()?.forEach { peer ->
        log.debug("discovered peer={}", peer)
      }
    }
  }

  override fun onConnect(peer: Peer) {
    val isAStaticPeer = isStaticPeer(peer.id)
    if (!isAStaticPeer && peerCount >= maxPeers) {
      peer.disconnectCleanly(DisconnectReason.TOO_MANY_PEERS)
      return
    }
    if (isAStaticPeer || reputationManager.isConnectionInitiationAllowed(peer.address)) {
      val maruPeer = maruPeerFactory.createMaruPeer(peer)
      sendOrExpectStatusMessage(maruPeer)
      peers[peer.id] = maruPeer
      log.debug("Connected to peer={}, static=$isAStaticPeer", peer.id)
    } else { // not static and not allowed to connect -> disconnect
      peer.disconnectCleanly(DisconnectReason.RATE_LIMITING)
      log.trace("Peer={} is not allowed to connect yet", peer.id)
    }
  }

  private fun sendOrExpectStatusMessage(maruPeer: MaruPeer) {
    if (maruPeer.connectionInitiatedLocally()) {
      maruPeer.sendStatus()
    } else {
      maruPeer.expectStatus()
    }
  }

  private fun connectedPeers(): Map<NodeId, MaruPeer> = peers.filter { it.value.isConnected }

  override fun onDisconnect(peer: Peer) {
    peers.remove(peer.id)
    log.debug("Peer={} disconnected", peer.id)
  }

  override fun getPeer(nodeId: NodeId): MaruPeer? = connectedPeers()[nodeId]

  override fun getPeers(): List<MaruPeer> = connectedPeers().values.toList()
}
