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
import maru.config.SyncingConfig
import maru.syncing.CLSyncStatus
import maru.syncing.ELSyncStatus
import maru.syncing.FakeSyncStatusProvider
import maru.syncing.SyncStatusProvider
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import tech.pegasys.teku.networking.p2p.network.P2PNetwork
import tech.pegasys.teku.networking.p2p.network.PeerAddress
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.peer.Peer
import java.util.concurrent.ConcurrentHashMap
import tech.pegasys.teku.networking.p2p.network.P2PNetwork as TekuP2PNetwork

class MaruPeerManagerTest {
  private object Fixtures {
    fun peerAddress(id: NodeId = mock()): PeerAddress = mock<PeerAddress>().also { whenever(it.id).thenReturn(id) }

    fun tekuPeer(
      id: NodeId = mock(),
      address: PeerAddress = peerAddress(id),
      initiatedLocally: Boolean = true,
    ): Peer =
      mock<Peer>().also {
        whenever(it.id).thenReturn(id)
        whenever(it.address).thenReturn(address)
        whenever(it.connectionInitiatedLocally()).thenReturn(initiatedLocally)
        whenever(it.connectionInitiatedRemotely()).thenReturn(!initiatedLocally)
      }

    fun maruPeer(
      initiatedLocally: Boolean = true,
      isConnected: Boolean = true,
      address: PeerAddress = mock(),
    ): MaruPeer {
      val id = address.id ?: mock<NodeId>()

      return mock<MaruPeer>().also {
        whenever(it.connectionInitiatedLocally()).thenReturn(initiatedLocally)
        whenever(it.isConnected).thenReturn(isConnected)
        whenever(it.address).thenReturn(address)
        whenever(it.id).thenReturn(id)
      }
    }
  }

  companion object {
    val reputationManager =
      MaruReputationManager(
        metricsSystem = NoOpMetricsSystem(),
        timeProvider = SystemTimeProvider(),
        isStaticPeer = { _: NodeId -> false },
        reputationConfig = P2PConfig.Reputation(),
      )

    private fun createSyncStatusProvider(): SyncStatusProvider =
      FakeSyncStatusProvider(
        clStatus = CLSyncStatus.SYNCED,
        elStatus = ELSyncStatus.SYNCED,
        beaconSyncDistanceValue = 0UL,
        clSyncTarget = 0UL,
      )

    val syncStatusProvider: SyncStatusProvider = createSyncStatusProvider()
  }

  @Test
  fun `does not schedule timeout when connection is initiated locally`() {
    val peer = Fixtures.tekuPeer(id = mock(), initiatedLocally = true)
    val maruPeerFactory = mock<MaruPeerFactory>()
    val maruPeer = Fixtures.maruPeer()
    val p2pConfig = mock<P2PConfig>()
    val syncConfig = mock<SyncingConfig>()

    whenever(maruPeerFactory.createMaruPeer(peer)).thenReturn(maruPeer)
    whenever(p2pConfig.maxPeers).thenReturn(10)
    whenever(syncConfig.desyncTolerance).thenReturn(32u)

    whenever(maruPeerFactory.createMaruPeer(peer)).thenReturn(maruPeer)
    whenever(maruPeer.connectionInitiatedLocally()).thenReturn(true)
    whenever(maruPeer.address).thenReturn(mock())
    whenever(p2pConfig.maxPeers).thenReturn(10)
    whenever(syncConfig.desyncTolerance).thenReturn(32u)

    val manager =
      MaruPeerManager(
        maruPeerFactory = maruPeerFactory,
        p2pConfig = p2pConfig,
        reputationManager = reputationManager,
        isStaticPeer = { false },
      )

    // Act
    manager.start(discoveryService = null, p2pNetwork = mock())
    manager.onConnect(peer)

    verify(maruPeerFactory).createMaruPeer(peer)
  }

  @Test
  fun `onConnect successfully connects peer when all conditions are met`() {
    val nodeId = mock<NodeId>()
    val peer = Fixtures.tekuPeer(nodeId, initiatedLocally = true)
    val maruPeerFactory = mock<MaruPeerFactory>()
    val maruPeer = Fixtures.maruPeer()
    val p2pConfig = mock<P2PConfig>()
    val p2pNetwork = mock<P2PNetwork<Peer>>()
    val syncConfig = mock<SyncingConfig>()

    whenever(p2pConfig.maxPeers).thenReturn(10)
    whenever(maruPeerFactory.createMaruPeer(peer)).thenReturn(maruPeer)
    whenever(maruPeer.sendStatus()).thenReturn(SafeFuture.completedFuture(Unit))
    whenever(p2pConfig.maxPeers).thenReturn(10)
    whenever(syncConfig.desyncTolerance).thenReturn(32u)

    val manager =
      MaruPeerManager(
        maruPeerFactory = maruPeerFactory,
        p2pConfig = p2pConfig,
        reputationManager = reputationManager,
        isStaticPeer = { false },
      )
    manager.start(discoveryService = null, p2pNetwork = p2pNetwork)
    manager.onConnect(peer)

    // Verify peer was not disconnected
    verify(maruPeer, never()).disconnectCleanly(any())
    verify(peer, never()).disconnectCleanly(any())

    // Verify peer is stored and retrievable
    assertThat(manager.getPeer(nodeId)).isEqualTo(maruPeer)
  }

  @Test
  fun `onConnect disconnects peer when max peers limit exceeded`() {
    val maruPeerFactory = mock<MaruPeerFactory>()
    val p2pConfig = mock<P2PConfig>()
    val syncConfig = mock<SyncingConfig>()

    whenever(p2pConfig.maxPeers).thenReturn(5)
    whenever(syncConfig.desyncTolerance).thenReturn(32u)

    val manager =
      MaruPeerManager(
        maruPeerFactory = maruPeerFactory,
        p2pConfig = p2pConfig,
        reputationManager = reputationManager,
        isStaticPeer = { false },
      )

    val p2pNetwork = mock<P2PNetwork<Peer>>()
    manager.start(discoveryService = null, p2pNetwork = p2pNetwork)
    // Add 5 connected peers to equal the maxPeers limit of 5
    addConnectedPeers(5, manager)

    val peer = Fixtures.tekuPeer(id = mock(), initiatedLocally = true)

    manager.onConnect(peer)

    verify(peer).disconnectCleanly(DisconnectReason.TOO_MANY_PEERS)
  }

  @Test
  fun `does not connect or add peer if reputation manager disallows connection`() {
    // Arrange
    val nodeId: NodeId = mock()
    val address = Fixtures.peerAddress(nodeId)
    val peer = Fixtures.tekuPeer(id = nodeId, address = address, initiatedLocally = true)
    val maruPeerFactory = mock<MaruPeerFactory>()
    val maruPeer = Fixtures.maruPeer()
    val p2pConfig = P2PConfig()
    val p2pNetwork = mock<TekuP2PNetwork<Peer>>()

    val reputationManager = mock<MaruReputationManager>()
    whenever(reputationManager.isConnectionInitiationAllowed(address)).thenReturn(false)

    whenever(maruPeerFactory.createMaruPeer(peer)).thenReturn(maruPeer)

    val manager =
      MaruPeerManager(
        maruPeerFactory = maruPeerFactory,
        p2pConfig = p2pConfig,
        reputationManager = reputationManager,
        isStaticPeer = { false },
      )

    // Act
    manager.start(discoveryService = null, p2pNetwork = p2pNetwork)
    manager.onConnect(peer)

    // Assert
    verify(maruPeerFactory, never()).createMaruPeer(peer)
    assertThat(manager.getPeer(nodeId)).isNull()
  }

  @Test
  fun `connects and adds peer if reputation manager allows connection`() {
    // Arrange
    val nodeId: NodeId = mock()
    val address = Fixtures.peerAddress(nodeId)
    val peer = Fixtures.tekuPeer(id = nodeId, address = address, initiatedLocally = true)
    val maruPeerFactory = mock<MaruPeerFactory>()
    val maruPeer = Fixtures.maruPeer()
    val p2pConfig = P2PConfig(maxPeers = 10)
    val reputationManager = mock<MaruReputationManager>()
    val p2pNetwork = mock<TekuP2PNetwork<Peer>>()

    whenever(peer.id).thenReturn(nodeId)
    whenever(peer.address).thenReturn(address)
    whenever(maruPeerFactory.createMaruPeer(peer)).thenReturn(maruPeer)
    whenever(reputationManager.isConnectionInitiationAllowed(address)).thenReturn(true)

    val manager =
      MaruPeerManager(
        maruPeerFactory = maruPeerFactory,
        p2pConfig = p2pConfig,
        reputationManager = reputationManager,
        isStaticPeer = { false },
      )

    // Act
    manager.start(discoveryService = null, p2pNetwork = p2pNetwork)
    manager.onConnect(peer)

    // Assert
    assertThat(manager.getPeer(nodeId)).isEqualTo(maruPeer)
    verify(maruPeerFactory).createMaruPeer(peer)
  }

  @Test
  fun `getPeers and peerCount only include actually connected peers`() {
    // Arrange
    val nodeId1: NodeId = mock()
    val nodeId2: NodeId = mock()

    val libp2pPeer1 = Fixtures.tekuPeer(id = nodeId1, initiatedLocally = true)
    val libp2pPeer2 = Fixtures.tekuPeer(id = nodeId2, initiatedLocally = true)

    val maruPeerFactory = mock<MaruPeerFactory>()

    val maruPeer1 =
      Fixtures.maruPeer(
        initiatedLocally = true,
        isConnected = true,
      )
    val maruPeer2 =
      Fixtures.maruPeer(
        initiatedLocally = true,
        isConnected = true,
      )
    val p2pConfig = P2PConfig()

    whenever(maruPeerFactory.createMaruPeer(libp2pPeer1)).thenReturn(maruPeer1)
    whenever(maruPeerFactory.createMaruPeer(libp2pPeer2)).thenReturn(maruPeer2)
    whenever(maruPeer1.sendStatus()).thenReturn(SafeFuture.completedFuture(Unit))
    whenever(maruPeer2.sendStatus()).thenReturn(SafeFuture.completedFuture(Unit))

    val manager =
      MaruPeerManager(
        maruPeerFactory = maruPeerFactory,
        p2pConfig = p2pConfig,
        reputationManager = reputationManager,
        isStaticPeer = { true },
      )

    // Act
    manager.start(discoveryService = null, p2pNetwork = mock())
    manager.onConnect(libp2pPeer1)
    manager.onConnect(libp2pPeer2)

    // Assert
    assertThat(manager.peerCount).isEqualTo(2)
    assertThat(manager.getPeers()).containsExactlyInAnyOrder(maruPeer1, maruPeer2)

    // Arrange 2
    whenever(maruPeer2.isConnected).thenReturn(false)

    // Assert 2
    assertThat(manager.peerCount).isEqualTo(1)
    assertThat(manager.getPeers()).containsExactly(maruPeer1)
    assertThat(manager.getPeer(nodeId2)).isNull()
    assertThat(manager.getPeer(nodeId1)).isEqualTo(maruPeer1)
  }

  private fun addConnectedPeer(
    manager: MaruPeerManager,
    maruPeer: MaruPeer,
  ) {
    // Use reflection to add the existing peer to connectedPeers
    val peersField = MaruPeerManager::class.java.getDeclaredField("peers")
    peersField.isAccessible = true
    @Suppress("UNCHECKED_CAST")
    val peers = peersField.get(manager) as ConcurrentHashMap<NodeId, MaruPeer>
    peers[maruPeer.id] = maruPeer
  }

  private fun addConnectedPeers(
    number: Int,
    manager: MaruPeerManager,
  ): List<MaruPeer> {
    val peers = mutableListOf<MaruPeer>()
    for (i in 1..number) {
      val nodeId = mock<NodeId>()
      val address = Fixtures.peerAddress(nodeId)
      val maruPeer = Fixtures.maruPeer(address = address)

      peers.add(maruPeer)
      addConnectedPeer(manager, maruPeer)
    }
    return peers
  }
}
