/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators.randomSealedBeaconBlock
import maru.core.ext.DataGenerators.randomStatus
import maru.p2p.MaruPeer
import maru.p2p.PeerLookup
import maru.p2p.messages.BeaconBlocksByRangeResponse
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.catchThrowable
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.async.SafeFuture.completedFuture
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.reputation.ReputationAdjustment
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeoutException
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class DownloadBlocksTest {
  private val defaultEndBlock = 11UL
  private val defaultBackoffDelay = 15.milliseconds

  private val defaultConfig =
    DownloadBlocksStep.Config(
      maxRetries = 5u,
      blockRangeRequestTimeout = 5.seconds,
      backoffDelay = defaultBackoffDelay,
    )

  @Test
  fun `downloads blocks successfully from peer`() {
    val peer = mock<MaruPeer>()
    val peerLookup = mock<PeerLookup>()
    val blocks = listOf(randomSealedBeaconBlock(10u), randomSealedBeaconBlock(defaultEndBlock))
    whenever(peer.sendBeaconBlocksByRange(10uL, 2uL)).thenReturn(completedFuture(BeaconBlocksByRangeResponse(blocks)))
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))
    whenever(peer.getStatus()).thenReturn(randomStatus(defaultEndBlock))

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(10uL, defaultEndBlock)
    val result = task.apply(range).get()

    assertThat(result.size).isEqualTo(2)
    assertThat(result.stream().map({ it.sealedBeaconBlock }).toList()).containsAll(blocks)
  }

  @Test
  fun `downloads blocks in multiple requests when peer returns partial results`() {
    val endBlock = 12UL
    val peer = mock<MaruPeer>()
    val peerLookup = mock<PeerLookup>()
    val block1 = randomSealedBeaconBlock(10uL)
    val block2 = randomSealedBeaconBlock(11uL)
    val block3 = randomSealedBeaconBlock(endBlock)

    // First call returns only block1, second call returns block2 and block3
    whenever(peer.sendBeaconBlocksByRange(10uL, 3uL))
      .thenReturn(completedFuture(BeaconBlocksByRangeResponse(listOf(block1))))
    whenever(peer.sendBeaconBlocksByRange(11uL, 2uL))
      .thenReturn(completedFuture(BeaconBlocksByRangeResponse(listOf(block2, block3))))
    whenever(peer.getStatus()).thenReturn(randomStatus(endBlock))
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(10uL, endBlock)
    val result = task.apply(range).get()

    assertThat(result.stream().map({ it.sealedBeaconBlock }).toList()).containsExactly(block1, block2, block3)
  }

  @Test
  fun `applies small penalty when peer returns empty blocks`() {
    val endBlock = 2uL
    val peer = mock<MaruPeer>()
    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))
    whenever(peer.sendBeaconBlocksByRange(1uL, 2uL)).thenReturn(
      completedFuture(
        BeaconBlocksByRangeResponse(emptyList()),
      ),
    )
    whenever(peer.getStatus()).thenReturn(randomStatus(endBlock))

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(1uL, endBlock)

    assertThrows<Exception> { task.apply(range).get() }
    verify(peer, atLeastOnce()).adjustReputation(ReputationAdjustment.SMALL_PENALTY)
  }

  @Test
  fun `applies large penalty on timeout exception`() {
    val endBlock = 1uL
    val peer = mock<MaruPeer>()
    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))
    val future = SafeFuture<BeaconBlocksByRangeResponse>()
    future.completeExceptionally(TimeoutException("timeout"))
    whenever(peer.getStatus()).thenReturn(randomStatus(endBlock))
    whenever(peer.sendBeaconBlocksByRange(1uL, 1uL)).thenReturn(future)

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(1uL, endBlock)

    assertThrows<Exception> { task.apply(range).get() }
    verify(peer, atLeastOnce()).adjustReputation(ReputationAdjustment.LARGE_PENALTY)
  }

  @Test
  fun `applies large penalty if the request future times out`() {
    val endBlock = 1uL
    val peer = mock<MaruPeer>()
    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))
    val future = SafeFuture<BeaconBlocksByRangeResponse>() // leave it uncompleted
    whenever(peer.getStatus()).thenReturn(randomStatus(endBlock))
    whenever(peer.sendBeaconBlocksByRange(1uL, 1uL)).thenReturn(future)

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig.copy(blockRangeRequestTimeout = 50.milliseconds),
      )
    val range = SyncTargetRange(1uL, endBlock)

    assertThrows<Exception> { task.apply(range).get() }
    verify(peer, atLeastOnce()).adjustReputation(ReputationAdjustment.LARGE_PENALTY)
  }

  @Test
  fun `throws after exceeding max retries`() {
    val endBlock = 1uL
    val nodeId = mock<NodeId>()
    val peerLookup = mock<PeerLookup>()
    val peer = mock<MaruPeer>()
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))
    whenever(peer.id).thenReturn(nodeId)
    whenever(peer.getStatus()).thenReturn(randomStatus(endBlock))
    val future = SafeFuture<BeaconBlocksByRangeResponse>()
    future.completeExceptionally(Exception("fail"))
    whenever(peer.sendBeaconBlocksByRange(any(), any())).thenReturn(future)

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(1uL, endBlock)

    val ex = assertThrows<Exception> { task.apply(range).get() }
    assertThat(ex.cause).isInstanceOf(DownloadBlocksStep.MaxRetriesReachedException::class.java)
  }

  @Test
  fun `throws when no peers available`() {
    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(emptyList())

    val task =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(1uL, 1uL)

    val ex = assertThrows<ExecutionException> { task.apply(range).get() }
    assertThat(ex.cause).isInstanceOf(DownloadBlocksStep.MaxRetriesReachedException::class.java)
  }

  @Test
  fun `downloads blocks from a random peer`() {
    val peer = mock<MaruPeer>()
    val peerLookup = mock<PeerLookup>()
    val blocks = listOf(randomSealedBeaconBlock(10u), randomSealedBeaconBlock(defaultEndBlock))
    val response = mock<BeaconBlocksByRangeResponse>()
    whenever(response.blocks).thenReturn(blocks)
    whenever(peer.sendBeaconBlocksByRange(10u, 2u)).thenReturn(completedFuture(response))
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer))
    whenever(peer.getStatus()).thenReturn(randomStatus(defaultEndBlock))

    val step =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(10u, defaultEndBlock)
    val result = step.apply(range).get()
    assertThat(result).isEqualTo(createSealedBlocksWithPeers(blocks, peer))
  }

  @Test
  fun `downloads blocks from a random peer with acceptable status`() {
    val peerLookup = mock<PeerLookup>()
    val blocks = listOf(randomSealedBeaconBlock(10u), randomSealedBeaconBlock(defaultEndBlock))
    val response = mock<BeaconBlocksByRangeResponse>()
    whenever(response.blocks).thenReturn(blocks)
    val acceptablePeer = mock<MaruPeer>()
    whenever(acceptablePeer.sendBeaconBlocksByRange(10u, 2u)).thenReturn(completedFuture(response))
    whenever(peerLookup.getPeers()).thenReturn(listOf(acceptablePeer))
    whenever(acceptablePeer.getStatus()).thenReturn(randomStatus(defaultEndBlock))

    val unacceptablePeer = mock<MaruPeer>()
    whenever(unacceptablePeer.sendBeaconBlocksByRange(10u, 2u)).thenReturn(
      SafeFuture.failedFuture(RuntimeException("Block not found!")),
    )
    whenever(peerLookup.getPeers()).thenReturn(listOf(acceptablePeer, unacceptablePeer))
    whenever(unacceptablePeer.getStatus()).thenReturn(randomStatus(defaultEndBlock.dec()))

    val step =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(10u, defaultEndBlock)
    val result = step.apply(range).get()
    assertThat(result).isEqualTo(createSealedBlocksWithPeers(blocks, acceptablePeer))
    verify(unacceptablePeer, never()).sendBeaconBlocksByRange(any(), any())
  }

  @Test
  fun `throws if no peers are returned from DownloadPeerProvider`() {
    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(emptyList())
    val step =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(peerLookup, false),
        config = defaultConfig,
      )
    val range = SyncTargetRange(0u, 0u)

    val throwable =
      catchThrowable {
        step.apply(range).get()
      }
    assertThat(throwable.cause).isInstanceOf(DownloadBlocksStep.MaxRetriesReachedException::class.java)
  }

  @Test
  fun `always select a random peer if there are available peers when purely random selection is true`() {
    val peer1 = mock<MaruPeer>()
    whenever(peer1.getStatus()).thenReturn(randomStatus(0U))

    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(listOf(peer1))

    val selectedPeer = DownloadPeerProviderImpl(peerLookup, true).getDownloadingPeer(100U)
    assertThat(selectedPeer).isEqualTo(peer1)
    verify(peer1, never()).getStatus()
  }

  @Test
  fun `returns null if no peers have latest block number more or equal to 100U with filter enabled`() {
    val peerLookup = mock<PeerLookup>()

    val peer1 = mock<MaruPeer>()
    whenever(peer1.getStatus()).thenReturn(randomStatus(50U))

    val peer2 = mock<MaruPeer>()
    whenever(peer2.getStatus()).thenReturn(randomStatus(80U))

    val peer3 = mock<MaruPeer>()
    whenever(peer3.getStatus()).thenReturn(randomStatus(99U))

    whenever(peerLookup.getPeers()).thenReturn(listOf(peer1, peer2, peer3))

    val peerWithFilter = DownloadPeerProviderImpl(peerLookup, false).getDownloadingPeer(100U)
    assertThat(peerWithFilter).isNull()
  }

  @Test
  fun `returns null if no peers are available regardless of purely random selection or not`() {
    val peerLookup = mock<PeerLookup>()
    whenever(peerLookup.getPeers()).thenReturn(emptyList())
    val peerWithFilter = DownloadPeerProviderImpl(peerLookup, false).getDownloadingPeer(100U)
    assertThat(peerWithFilter).isNull()

    val peerWithoutFilter = DownloadPeerProviderImpl(peerLookup, true).getDownloadingPeer(100U)
    assertThat(peerWithoutFilter).isNull()
  }

  private fun createSealedBlocksWithPeers(
    blocks: List<SealedBeaconBlock>,
    peer: MaruPeer,
  ): List<SealedBlockWithPeer> =
    blocks
      .stream()
      .map({
        SealedBlockWithPeer(it, peer)
      })
      .toList()
}
