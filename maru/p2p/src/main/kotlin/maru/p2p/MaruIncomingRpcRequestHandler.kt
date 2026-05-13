/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import io.netty.buffer.ByteBuf
import io.netty.buffer.ByteBufUtil
import maru.serialization.SerDe
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.rpc.RpcRequestHandler
import tech.pegasys.teku.networking.p2p.rpc.RpcStream

class MaruIncomingRpcRequestHandler<
  TRequest : RequestMessageAdapter<*, RpcMessageType>,
  TResponse : Message<*, RpcMessageType>,
  >(
  private val rpcMessageHandler: RpcMessageHandler<TRequest, TResponse>,
  private val requestMessageSerDe: SerDe<TRequest>,
  private val responseMessageSerDe: SerDe<TResponse>,
  private val peerLookup: PeerLookup,
) : RpcRequestHandler {
  private val log = LogManager.getLogger(this.javaClass)

  override fun active(
    nodeId: NodeId,
    rpcStream: RpcStream,
  ) {
  }

  override fun processData(
    nodeId: NodeId,
    rpcStream: RpcStream,
    byteBuffer: ByteBuf,
  ) {
    val bytes = ByteBufUtil.getBytes(byteBuffer)
    val maybePeer = peerLookup.getPeer(nodeId)
    val message = requestMessageSerDe.deserialize(bytes)
    maybePeer?.let { peer ->
      rpcMessageHandler.handleIncomingMessage(
        peer = peer,
        message = message,
        callback = MaruRpcResponseCallback(
          rpcStream = rpcStream,
          messageSerializer = responseMessageSerDe,
        ),
      )
    } ?: { log.trace("Ignoring message of type {} because peer has been disconnected", message.type) }
  }

  override fun readComplete(
    nodeId: NodeId,
    rpcStream: RpcStream,
  ) {
  }

  override fun closed(
    nodeId: NodeId,
    rpcStream: RpcStream,
  ) {
  }
}
