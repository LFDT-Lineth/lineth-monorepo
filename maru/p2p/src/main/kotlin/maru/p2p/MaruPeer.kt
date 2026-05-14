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
import maru.p2p.messages.BeaconBlocksByRangeRequest
import maru.p2p.messages.BeaconBlocksByRangeResponse
import maru.p2p.messages.Status
import maru.p2p.messages.StatusManager
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.p2p.network.PeerAddress
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.peer.DisconnectRequestHandler
import tech.pegasys.teku.networking.p2p.peer.Peer
import tech.pegasys.teku.networking.p2p.peer.PeerDisconnectedException
import tech.pegasys.teku.networking.p2p.peer.PeerDisconnectedSubscriber
import tech.pegasys.teku.networking.p2p.reputation.ReputationAdjustment
import tech.pegasys.teku.networking.p2p.rpc.RpcMethod
import tech.pegasys.teku.networking.p2p.rpc.RpcRequestHandler
import tech.pegasys.teku.networking.p2p.rpc.RpcResponseHandler
import tech.pegasys.teku.networking.p2p.rpc.RpcStreamController
import tech.pegasys.teku.spec.datastructures.networking.libp2p.rpc.RpcRequest
import tech.pegasys.teku.spec.datastructures.networking.libp2p.rpc.bodyselector.RpcRequestBodySelector
import java.util.Optional
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

interface MaruPeer : Peer {
  fun getStatus(): Status?

  fun sendStatus(): SafeFuture<Unit>

  fun expectStatus(): Unit

  fun updateStatus(newStatus: Status)

  fun sendBeaconBlocksByRange(
    startBlockNumber: ULong,
    count: ULong,
  ): SafeFuture<BeaconBlocksByRangeResponse>
}

fun MaruPeer.toLogString(): String {
  // e.g 16Uiu2HAmNp6gzhT3GwJQjUw6awr3o75SEr9ZVWVfVX3Fq22ERsKS|/ip4/10.42.0.84/tcp/45166|91
  // useful to see which peer we are connected to and reported head
  return "{id=$id, addr=${this.address.toExternalForm()}, lastClBlockNumber=${getStatus()?.latestBlockNumber}}"
}

interface MaruPeerFactory {
  fun createMaruPeer(delegatePeer: Peer): MaruPeer
}

class DefaultMaruPeerFactory(
  private val rpcMethods: RpcMethods,
  private val statusManager: StatusManager,
  private val p2pConfig: P2PConfig,
) : MaruPeerFactory {
  override fun createMaruPeer(delegatePeer: Peer): MaruPeer =
    DefaultMaruPeer(
      delegatePeer = delegatePeer,
      rpcMethods = rpcMethods,
      statusManager = statusManager,
      p2pConfig = p2pConfig,
    )
}

class DefaultMaruPeer(
  private val delegatePeer: Peer,
  private val rpcMethods: RpcMethods,
  private val statusManager: StatusManager,
  private val scheduler: ScheduledExecutorService =
    Executors.newSingleThreadScheduledExecutor(
      Thread.ofVirtual().factory(),
    ),
  private val p2pConfig: P2PConfig,
) : MaruPeer {
  init {
    delegatePeer.subscribeDisconnect { _, _ ->
      scheduledDisconnect.ifPresent { it.cancel(true) }
      scheduler.shutdown()
    }
  }

  private val log: Logger = LogManager.getLogger(this.javaClass)
  private val status = AtomicReference<Status?>(null)
  internal var scheduledDisconnect: Optional<ScheduledFuture<*>> = Optional.empty()

  override fun getStatus(): Status? = status.get()

  override fun sendStatus(): SafeFuture<Unit> {
    try {
      val statusMessage = statusManager.createStatusMessage()
      val sendRpcMessage: SafeFuture<Message<Status, RpcMessageType>> =
        sendRpcMessage(RequestMessageAdapter(statusMessage), rpcMethods.status())
      scheduleDisconnectIfStatusNotReceived(p2pConfig.statusUpdate.timeout)
      return sendRpcMessage
        .thenApply { message -> message.payload }
        .whenComplete { status, error ->
          if (error != null) {
            disconnectImmediately(Optional.of(DisconnectReason.UNRESPONSIVE), false)
            if (error.cause !is PeerDisconnectedException) {
              log.debug("Failed to send status message to peer={}: errorMessage={}", this.id, error.message, error)
            }
          } else {
            updateStatus(status)
            try {
              if (!scheduler.isShutdown) {
                scheduler.schedule(
                  this::sendStatus,
                  p2pConfig.statusUpdate.refreshInterval.inWholeSeconds,
                  TimeUnit.SECONDS,
                )
              }
            } catch (e: Exception) {
              log.trace("Failed to schedule sendStatus to peerId={}", this.id, e)
            }
          }
        }.thenApply {}
    } catch (e: Exception) {
      if (e.cause !is PeerDisconnectedException) {
        log.error("Failed to send status message to peer={}", id, e)
      }
      return SafeFuture.failedFuture(e)
    }
  }

  override fun updateStatus(newStatus: Status) {
    scheduledDisconnect.ifPresent { it.cancel(false) }
    if (!statusManager.isValidForPeering(newStatus)) {
      disconnectCleanly(DisconnectReason.IRRELEVANT_NETWORK)
      return
    }
    status.set(newStatus)
    log.trace("Received status update from peer={}: status={}", id, newStatus)
    if (connectionInitiatedRemotely()) {
      scheduleDisconnectIfStatusNotReceived(
        p2pConfig.statusUpdate.refreshInterval + p2pConfig.statusUpdate.refreshIntervalLeeway,
      )
    }
  }

  fun scheduleDisconnectIfStatusNotReceived(delay: Duration) {
    scheduledDisconnect.ifPresent { it.cancel(false) }
    if (!scheduler.isShutdown) {
      try {
        scheduledDisconnect = Optional.of(
          scheduler.schedule(
            {
              log.debug("Disconnecting from peerId={} by timeout", this.id)
              disconnectCleanly(DisconnectReason.UNRESPONSIVE)
            },
            delay.inWholeMilliseconds,
            TimeUnit.MILLISECONDS,
          ),
        )
      } catch (e: Exception) {
        log.trace("Failed to schedule disconnect for peerId={}", this.id, e)
      }
    }
  }

  override fun expectStatus() {
    scheduleDisconnectIfStatusNotReceived(
      p2pConfig.statusUpdate.timeout + p2pConfig.statusUpdate.refreshIntervalLeeway,
    )
  }

  override fun sendBeaconBlocksByRange(
    startBlockNumber: ULong,
    count: ULong,
  ): SafeFuture<BeaconBlocksByRangeResponse> {
    val request = BeaconBlocksByRangeRequest(startBlockNumber, count)
    val message = RequestMessageAdapter(MessageData(RpcMessageType.BEACON_BLOCKS_BY_RANGE, Version.V1, request))
    return sendRpcMessage(message, rpcMethods.beaconBlocksByRange())
      .thenApply { responseMessage -> responseMessage.payload }
  }

  fun <TRequest : RequestMessageAdapter<*, RpcMessageType>, TResponse : Message<*, RpcMessageType>> sendRpcMessage(
    message: TRequest,
    rpcMethod: MaruRpcMethod<TRequest, TResponse>,
  ): SafeFuture<TResponse> {
    val responseHandler = MaruRpcResponseHandler<TResponse>()
    return sendRequest<MaruOutgoingRpcRequestHandler<TResponse>, TRequest, MaruRpcResponseHandler<TResponse>>(
      rpcMethod,
      message,
      responseHandler,
    ).thenCompose {
      responseHandler.response()
    }
  }

  override fun getAddress(): PeerAddress = delegatePeer.address

  override fun getGossipScore(): Double = delegatePeer.gossipScore

  override fun isConnected(): Boolean = delegatePeer.isConnected

  override fun disconnectImmediately(
    reason: Optional<DisconnectReason>,
    locallyInitiated: Boolean,
  ) {
    scheduler.shutdown()
    delegatePeer.disconnectImmediately(reason, locallyInitiated)
  }

  override fun disconnectCleanly(reason: DisconnectReason?): SafeFuture<Void> {
    scheduler.shutdown()
    return delegatePeer.disconnectCleanly(reason)
  }

  override fun setDisconnectRequestHandler(handler: DisconnectRequestHandler) =
    delegatePeer.setDisconnectRequestHandler(handler)

  override fun subscribeDisconnect(subscriber: PeerDisconnectedSubscriber) =
    delegatePeer.subscribeDisconnect(subscriber)

  override fun <
    TOutgoingHandler : RpcRequestHandler,
    TRequest : RpcRequest,
    RespHandler : RpcResponseHandler<*>,
    > sendRequest(
    rpcMethod: RpcMethod<TOutgoingHandler, TRequest, RespHandler>,
    request: TRequest,
    responseHandler: RespHandler,
  ): SafeFuture<RpcStreamController<TOutgoingHandler>> = delegatePeer.sendRequest(rpcMethod, request, responseHandler)

  override fun <
    TOutgoingHandler : RpcRequestHandler,
    TRequest : RpcRequest,
    RespHandler : RpcResponseHandler<*>,
    > sendRequest(
    rpcMethod: RpcMethod<TOutgoingHandler, TRequest, RespHandler>,
    rpcRequestBodySelector: RpcRequestBodySelector<TRequest>,
    responseHandler: RespHandler,
  ): SafeFuture<RpcStreamController<TOutgoingHandler>> =
    delegatePeer.sendRequest(rpcMethod, rpcRequestBodySelector, responseHandler)

  override fun connectionInitiatedLocally(): Boolean = delegatePeer.connectionInitiatedLocally()

  override fun connectionInitiatedRemotely(): Boolean = delegatePeer.connectionInitiatedRemotely()

  override fun adjustReputation(adjustment: ReputationAdjustment) = delegatePeer.adjustReputation(adjustment)

  override fun toString(): String =
    "DefaultMaruPeer(id=${id.toBase58()}, status=${status.get()}, address=${getAddress()}, " +
      "gossipScore=${getGossipScore()}, connected=$isConnected)"
}
