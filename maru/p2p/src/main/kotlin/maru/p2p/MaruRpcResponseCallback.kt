/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.p2p.messages.RpcExceptionSerDe
import maru.serialization.Serializer
import org.apache.logging.log4j.LogManager
import org.apache.tuweni.bytes.Bytes
import tech.pegasys.teku.infrastructure.async.RootCauseExceptionHandler
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.eth2.rpc.core.ResponseCallback
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcResponseStatus.SUCCESS_RESPONSE_CODE
import tech.pegasys.teku.networking.p2p.peer.PeerDisconnectedException
import tech.pegasys.teku.networking.p2p.rpc.RpcStream
import tech.pegasys.teku.networking.p2p.rpc.StreamClosedException
import java.nio.channels.ClosedChannelException

class MaruRpcResponseCallback<TResponse : Message<*, *>>(
  private val rpcStream: RpcStream,
  private val messageSerializer: Serializer<TResponse>,
  private val rpcExceptionSerializer: Serializer<RpcException> = RpcExceptionSerDe(),
) : ResponseCallback<TResponse> {
  private val log = LogManager.getLogger(this.javaClass)

  override fun respond(data: TResponse): SafeFuture<Void> =
    rpcStream.writeBytes(
      Bytes.concatenate(
        Bytes.of(SUCCESS_RESPONSE_CODE),
        Bytes.wrap(messageSerializer.serialize(data)),
      ),
    )

  override fun respondAndCompleteSuccessfully(data: TResponse) {
    respond(data)
      .thenRun { completeSuccessfully() }
      .finish(
        RootCauseExceptionHandler
          .builder()
          .addCatch(
            ClosedChannelException::class.java,
          ) { err -> log.trace("Failed to write because channel was closed", err) }
          .defaultCatch { err -> log.error("Failed to write req/resp response", err) },
      )
  }

  override fun completeSuccessfully() {
    rpcStream.closeWriteStream().finishWarn(log)
  }

  override fun completeWithErrorResponse(error: RpcException) {
    log.debug("Responding to RPC request with error: {}", error.errorMessageString)
    try {
      rpcStream
        .writeBytes(
          Bytes.concatenate(
            Bytes.of(error.responseCode),
            Bytes.wrap(rpcExceptionSerializer.serialize(error)),
          ),
        ).finishWarn(log)
    } catch (e: StreamClosedException) {
      log.trace(
        "Unable to send error message ({}) to peer, rpc stream already closed: {}",
        error,
        rpcStream,
      )
    }
    rpcStream.closeWriteStream().finishWarn(log)
  }

  override fun completeWithUnexpectedError(error: Throwable) {
    when (error) {
      is PeerDisconnectedException -> {
        log.trace("Not sending RPC response as peer has already disconnected")
        // But close the stream just to be completely sure we don't leak any resources.
        rpcStream.closeAbruptly().finishWarn(log)
      }

      is RpcException -> {
        completeWithErrorResponse(error)
      }

      else -> {
        log.error("Encountered unexpected error from server", error)
        completeWithErrorResponse(RpcException.ServerErrorException())
      }
    }
  }
}
