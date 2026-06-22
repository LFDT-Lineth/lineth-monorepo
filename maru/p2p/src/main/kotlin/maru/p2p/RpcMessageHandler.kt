/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import tech.pegasys.teku.networking.eth2.rpc.core.ResponseCallback

/**
 * Interface for handling incoming RPC messages. Adapted from Teku LocalMessageHandler without use of the Teku Eth2Peer
 * which not applicable to Maru.
 *
 * @param TRequest The type of the request message.
 * @param TResponse The type of the response message.
 */
interface RpcMessageHandler<TRequest, TResponse> {
  fun handleIncomingMessage(
    peer: MaruPeer,
    message: TRequest,
    callback: ResponseCallback<TResponse>,
  )
}
