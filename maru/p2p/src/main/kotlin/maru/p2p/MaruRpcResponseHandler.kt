/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.p2p.rpc.RpcResponseHandler
import java.util.Optional

class MaruRpcResponseHandler<TResponse> : RpcResponseHandler<TResponse> {
  val future = SafeFuture<TResponse>()

  override fun onResponse(response: TResponse): SafeFuture<*> {
    future.complete(response)
    return SafeFuture.completedFuture(response)
  }

  override fun onCompleted(error: Optional<out Throwable>) {
    if (error.isEmpty) {
      return // if needed to do something when the response is completed successfully
    } else {
      future.completeExceptionally(error.get())
    }
  }

  fun response(): SafeFuture<TResponse> = future
}
