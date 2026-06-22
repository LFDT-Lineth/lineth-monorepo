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
import maru.p2p.messages.Status
import maru.p2p.messages.StatusManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.p2p.network.PeerAddress
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.peer.DisconnectRequestHandler
import tech.pegasys.teku.networking.p2p.peer.Peer
import tech.pegasys.teku.networking.p2p.peer.PeerDisconnectedSubscriber
import tech.pegasys.teku.networking.p2p.reputation.ReputationAdjustment
import tech.pegasys.teku.networking.p2p.rpc.RpcMethod
import tech.pegasys.teku.networking.p2p.rpc.RpcRequestHandler
import tech.pegasys.teku.networking.p2p.rpc.RpcResponseHandler
import tech.pegasys.teku.networking.p2p.rpc.RpcStreamController
import java.util.Optional
import kotlin.time.Duration.Companion.seconds

class DefaultMaruPeerTest {
  private val delegatePeer = mock<Peer>()
  private val rpcMethods = mock<RpcMethods>()
  private val statusManager = mock<StatusManager>()
  private val maruPeer =
    DefaultMaruPeer(
      delegatePeer,
      rpcMethods,
      statusManager,
      p2pConfig = P2PConfig(ipAddress = "1.1.1.1", port = 9876u),
    )

  @Test
  fun `getStatus returns null initially`() {
    val result = maruPeer.getStatus()
    assertThat(result).isNull()
  }

  @Test
  fun `updateStatus sets the status`() {
    val status = mock<Status>()
    whenever(delegatePeer.address).thenReturn(mock())
    whenever(statusManager.isValidForPeering(any())).thenReturn(true)

    maruPeer.updateStatus(status)

    val result = maruPeer.getStatus()
    assertThat(result).isEqualTo(status)
  }

  @Test
  fun `getAddress delegates to underlying peer`() {
    val expectedAddress = mock<PeerAddress>()

    whenever(delegatePeer.address).thenReturn(expectedAddress)

    val result = maruPeer.getAddress()

    assertThat(result).isEqualTo(expectedAddress)
    verify(delegatePeer).address
  }

  @Test
  fun `getGossipScore delegates to underlying peer`() {
    val expectedScore = 0.85

    whenever(delegatePeer.gossipScore).thenReturn(expectedScore)

    val result = maruPeer.getGossipScore()

    assertThat(result).isEqualTo(expectedScore)
    verify(delegatePeer).gossipScore
  }

  @Test
  fun `isConnected delegates to underlying peer`() {
    whenever(delegatePeer.isConnected).thenReturn(true)

    val result = maruPeer.isConnected()

    assertThat(result).isTrue()
    verify(delegatePeer).isConnected
  }

  @Test
  fun `disconnectImmediately delegates to underlying peer`() {
    val reason = Optional.of(DisconnectReason.REMOTE_FAULT)

    maruPeer.disconnectImmediately(reason, true)

    verify(delegatePeer).disconnectImmediately(reason, true)
  }

  @Test
  fun `disconnectCleanly delegates to underlying peer`() {
    val reason = DisconnectReason.TOO_MANY_PEERS
    val expectedFuture = SafeFuture.completedFuture<Void>(null)

    whenever(delegatePeer.disconnectCleanly(reason)).thenReturn(expectedFuture)

    val result = maruPeer.disconnectCleanly(reason)

    assertThat(result).isEqualTo(expectedFuture)
    verify(delegatePeer).disconnectCleanly(reason)
  }

  @Test
  fun `setDisconnectRequestHandler delegates to underlying peer`() {
    val handler = mock<DisconnectRequestHandler>()

    maruPeer.setDisconnectRequestHandler(handler)

    verify(delegatePeer).setDisconnectRequestHandler(handler)
  }

  @Test
  fun `subscribeDisconnect delegates to underlying peer`() {
    val subscriber = mock<PeerDisconnectedSubscriber>()

    maruPeer.subscribeDisconnect(subscriber)

    verify(delegatePeer).subscribeDisconnect(subscriber)
  }

  @Test
  fun `sendRequest delegates to underlying peer`() {
    val rpcMethod =
      mock<RpcMethod<RpcRequestHandler, RequestMessageAdapter<String, RpcMessageType>, RpcResponseHandler<*>>>()
    val request = RequestMessageAdapter(MessageData(RpcMessageType.STATUS, Version.V1, "test-request"))
    val responseHandler = mock<RpcResponseHandler<*>>()
    val expectedController = mock<RpcStreamController<RpcRequestHandler>>()

    whenever(delegatePeer.sendRequest(rpcMethod, request, responseHandler))
      .thenReturn(SafeFuture.completedFuture(expectedController))

    val result = maruPeer.sendRequest(rpcMethod, request, responseHandler)

    verify(delegatePeer).sendRequest(rpcMethod, request, responseHandler)
    assertThat(result.get()).isEqualTo(expectedController)
  }

  @Test
  fun `connectionInitiatedLocally delegates to underlying peer`() {
    whenever(delegatePeer.connectionInitiatedLocally()).thenReturn(true)

    val result = maruPeer.connectionInitiatedLocally()

    assertThat(result).isTrue()
    verify(delegatePeer).connectionInitiatedLocally()
  }

  @Test
  fun `connectionInitiatedRemotely delegates to connectionInitiatedRemotely on underlying peer`() {
    whenever(delegatePeer.connectionInitiatedRemotely()).thenReturn(true)

    val result = maruPeer.connectionInitiatedRemotely()

    assertThat(result).isTrue()
    verify(delegatePeer).connectionInitiatedRemotely()
  }

  @Test
  fun `adjustReputation delegates to underlying peer`() {
    maruPeer.adjustReputation(ReputationAdjustment.SMALL_PENALTY)

    verify(delegatePeer).adjustReputation(ReputationAdjustment.SMALL_PENALTY)
  }

  @Test
  fun `sendStatus returns failed future when exception is thrown`() {
    whenever(statusManager.createStatusMessage()).thenThrow(RuntimeException("fail"))
    whenever(delegatePeer.address).thenReturn(mock())
    val future = maruPeer.sendStatus()
    assertThat(future).isNotNull()
    assertThat(future.isDone).isTrue()
    assertThat(future.isCompletedExceptionally).isTrue()
  }

  @Test
  fun `updateStatus sets status and cancels scheduled disconnect`() {
    val status = mock<Status>()
    whenever(delegatePeer.address).thenReturn(mock())
    whenever(statusManager.isValidForPeering(any())).thenReturn(true)

    maruPeer.scheduleDisconnectIfStatusNotReceived(1.seconds)
    maruPeer.updateStatus(status)
    assertThat(maruPeer.getStatus()).isEqualTo(status)
    val scheduledDisconnect = maruPeer.scheduledDisconnect
    assertThat(scheduledDisconnect.get().isCancelled).isTrue()
  }
}
