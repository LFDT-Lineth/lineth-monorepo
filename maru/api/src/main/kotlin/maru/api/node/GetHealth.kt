/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.node

import io.javalin.http.Context
import io.javalin.http.Handler

class GetHealth : Handler {
  override fun handle(ctx: Context) {
    // Placeholder for actual health check logic
    ctx.status(200).json("Node is ready")
  }

  companion object {
    const val ROUTE = "/eth/v1/node/health"
  }
}
