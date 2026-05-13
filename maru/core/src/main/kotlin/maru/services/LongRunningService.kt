/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.services
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import linea.LongRunningService as LineaLongRunningService

typealias LongRunningService = LineaLongRunningService

object NoOpLongRunningService : LongRunningService {
  override fun start(): CompletableFuture<Unit> = SafeFuture.completedFuture(Unit)

  override fun stop(): CompletableFuture<Unit> = SafeFuture.completedFuture(Unit)
}
