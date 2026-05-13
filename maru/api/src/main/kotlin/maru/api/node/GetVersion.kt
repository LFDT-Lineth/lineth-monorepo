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
import maru.VersionProvider

data class GetVersionResponse(
  val data: VersionData,
)

data class VersionData(
  val version: String,
)

class GetVersion(
  val versionProvider: VersionProvider,
) : Handler {
  override fun handle(ctx: Context) {
    ctx.status(200).json(
      GetVersionResponse(
        data = VersionData(version = versionProvider.getVersion()),
      ),
    )
  }

  companion object {
    const val ROUTE = "/eth/v1/node/version"
  }
}
