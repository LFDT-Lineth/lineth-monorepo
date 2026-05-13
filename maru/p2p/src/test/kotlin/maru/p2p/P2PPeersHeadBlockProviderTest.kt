/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.p2p.messages.Status
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.networking.p2p.network.PeerAddress
import tech.pegasys.teku.networking.p2p.peer.NodeId

class P2PPeersHeadBlockProviderTest {
  private lateinit var peerLookup: TestPeerLookup
  private lateinit var provider: P2PPeersHeadBlockProvider

  @BeforeEach
  fun setUp() {
    peerLookup = TestPeerLookup()
    provider = P2PPeersHeadBlockProvider(peerLookup)
  }

  @Test
  fun `should return empty map when no peers are connected`() {
    // Arrange - empty peer lookup by default

    // Act
    val result = provider.getPeersHeads()

    // Assert
    assertThat(result).isEmpty()
  }

  @Test
  fun `should return map of peer IDs to head block numbers for peers with status`() {
    // Arrange
    val peer1 = mockPeerWithStatus("peer1", 100UL)
    val peer2 = mockPeerWithStatus("peer2", 200UL)
    val peer3 = mockPeerWithStatus("peer3", 150UL)

    peerLookup.addPeer(peer1)
    peerLookup.addPeer(peer2)
    peerLookup.addPeer(peer3)

    // Act
    val result = provider.getPeersHeads()

    // Assert
    assertThat(result)
      .containsExactlyInAnyOrderEntriesOf(
        mapOf(
          "peer1" to 100UL,
          "peer2" to 200UL,
          "peer3" to 150UL,
        ),
      )
  }

  @Test
  fun `should exclude peers without status`() {
    // Arrange
    val peer1 = mockPeerWithStatus("peer1", 100UL)
    val peer2 = mockPeerWithoutStatus("peer2")
    val peer3 = mockPeerWithStatus("peer3", 150UL)

    peerLookup.addPeer(peer1)
    peerLookup.addPeer(peer2)
    peerLookup.addPeer(peer3)

    // Act
    val result = provider.getPeersHeads()

    // Assert
    assertThat(result)
      .containsExactlyInAnyOrderEntriesOf(
        mapOf(
          "peer1" to 100UL,
          "peer3" to 150UL,
        ),
      ).doesNotContainKey("peer2")
  }

  @Test
  fun `should handle exceptions when getting peer status`() {
    // Arrange
    val peer1 = mockPeerWithStatus("peer1", 100UL)
    val peer2 = mockPeerWithException("peer2")
    val peer3 = mockPeerWithStatus("peer3", 150UL)

    peerLookup.addPeer(peer1)
    peerLookup.addPeer(peer2)
    peerLookup.addPeer(peer3)

    // Act
    val result = provider.getPeersHeads()

    // Assert
    assertThat(result)
      .containsExactlyInAnyOrderEntriesOf(
        mapOf(
          "peer1" to 100UL,
          "peer3" to 150UL,
        ),
      ).doesNotContainKey("peer2")
  }

  /**
   * Simple test double for PeerLookup that stores a list of peers
   */
  private class TestPeerLookup : PeerLookup {
    private val peers = mutableListOf<MaruPeer>()

    fun addPeer(peer: MaruPeer) {
      peers.add(peer)
    }

    override fun getPeer(nodeId: NodeId): MaruPeer? = peers.find { it.id.toBase58() == nodeId.toBase58() }

    override fun getPeers(): List<MaruPeer> = peers.toList()
  }

  private fun mockPeerWithStatus(
    peerId: String,
    blockNumber: ULong,
  ): MaruPeer {
    val peer = mock(MaruPeer::class.java)
    val nodeId = mock(NodeId::class.java)

    whenever(peer.id).thenReturn(nodeId)
    whenever(nodeId.toBase58()).thenReturn(peerId)
    whenever(peer.address).thenReturn(PeerAddress(nodeId))

    val status =
      Status(
        forkIdHash = ByteArray(32) { 0 },
        latestStateRoot = ByteArray(32) { 0 },
        latestBlockNumber = blockNumber,
      )

    whenever(peer.getStatus()).thenReturn(status)

    return peer
  }

  private fun mockPeerWithoutStatus(peerId: String): MaruPeer {
    val peer = mock(MaruPeer::class.java)
    val nodeId = mock(NodeId::class.java)

    whenever(peer.id).thenReturn(nodeId)
    whenever(nodeId.toBase58()).thenReturn(peerId)
    whenever(peer.getStatus()).thenReturn(null)

    return peer
  }

  private fun mockPeerWithException(peerId: String): MaruPeer {
    val peer = mock(MaruPeer::class.java)
    val nodeId = mock(NodeId::class.java)

    whenever(peer.id).thenReturn(nodeId)
    whenever(nodeId.toBase58()).thenReturn(peerId)
    whenever(peer.getStatus()).thenThrow(RuntimeException("Failed to get status"))

    return peer
  }
}
