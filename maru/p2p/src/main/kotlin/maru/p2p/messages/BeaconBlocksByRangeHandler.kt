/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.database.BeaconChain
import maru.p2p.MaruPeer
import maru.p2p.Message
import maru.p2p.MessageData
import maru.p2p.RequestMessageAdapter
import maru.p2p.RpcMessageHandler
import maru.p2p.RpcMessageType
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.networking.eth2.rpc.core.ResponseCallback
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcResponseStatus

/**
 * A configurable handler that uses composition with BlockRetrievalStrategy
 * instead of requiring inheritance.
 */
class BeaconBlocksByRangeHandler(
  private val beaconChain: BeaconChain,
  private val blockRetrievalStrategy: BlockRetrievalStrategy = DefaultBlockRetrievalStrategy(),
) : RpcMessageHandler<
  RequestMessageAdapter<BeaconBlocksByRangeRequest, RpcMessageType>,
  Message<BeaconBlocksByRangeResponse, RpcMessageType>,
  > {
  private val log = LogManager.getLogger(this.javaClass)

  companion object {
    const val MAX_BLOCKS_PER_REQUEST = 256UL
  }

  override fun handleIncomingMessage(
    peer: MaruPeer,
    message: RequestMessageAdapter<BeaconBlocksByRangeRequest, RpcMessageType>,
    callback: ResponseCallback<Message<BeaconBlocksByRangeResponse, RpcMessageType>>,
  ) {
    try {
      val request = message.payload

      // Limit the number of blocks to prevent excessive resource usage
      val maxBlocks = minOf(request.count, MAX_BLOCKS_PER_REQUEST)

      val blocks = blockRetrievalStrategy.getBlocks(beaconChain, request, maxBlocks)

      val response = BeaconBlocksByRangeResponse(blocks = blocks)
      val responseMessage =
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          payload = response,
        )

      callback.respondAndCompleteSuccessfully(responseMessage)
    } catch (e: RpcException) {
      log.error("handling request failed with RpcException", e)
      callback.completeWithErrorResponse(e)
    } catch (e: IllegalStateException) {
      val errorMessage = "Handling request failed with IllegalStateException: " + e.message
      if (e.message != null && e.message!!.contains("Missing sealed beacon block")) {
        callback.completeWithErrorResponse(
          RpcException(RpcResponseStatus.RESOURCE_UNAVAILABLE, errorMessage),
        )
      } else {
        callback.completeWithUnexpectedError(
          RpcException(RpcResponseStatus.SERVER_ERROR_CODE, errorMessage),
        )
      }
    } catch (th: Throwable) {
      log.error("handling request failed with unexpected error", th)
      callback.completeWithUnexpectedError(
        RpcException(
          RpcResponseStatus.SERVER_ERROR_CODE,
          "Handling request failed with unexpected error: " + th.message,
        ),
      )
    }
  }
}
