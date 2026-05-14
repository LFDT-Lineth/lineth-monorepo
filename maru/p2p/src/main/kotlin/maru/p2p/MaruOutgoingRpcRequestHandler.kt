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
import maru.p2p.messages.RpcExceptionSerDe
import maru.serialization.Deserializer
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcResponseStatus
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.rpc.RpcRequestHandler
import tech.pegasys.teku.networking.p2p.rpc.RpcStream

class MaruOutgoingRpcRequestHandler<TResponse>(
  private val responseHandler: MaruRpcResponseHandler<TResponse>,
  private val responseMessageDeserializer: Deserializer<TResponse>,
  private val rpcExceptionDeserializer: Deserializer<RpcException> = RpcExceptionSerDe(),
) : RpcRequestHandler {
  override fun active(
    nodeId: NodeId,
    rpcStream: RpcStream,
  ) {
  }

  override fun processData(
    nodeId: NodeId,
    rpcStream: RpcStream,
    byteBuf: ByteBuf,
  ) {
    val respCode = byteBuf.readByte()
    val bytes = ByteBufUtil.getBytes(byteBuf)
    rpcStream.closeWriteStream()
    when (respCode) {
      RpcResponseStatus.SUCCESS_RESPONSE_CODE -> {
        val response = responseMessageDeserializer.deserialize(bytes)
        responseHandler.onResponse(response)
      }
      else -> {
        val rpcException = rpcExceptionDeserializer.deserialize(bytes)
        responseHandler.onCompleted(rpcException)
      }
    }
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
