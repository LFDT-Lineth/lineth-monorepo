/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import maru.consensus.ElFork
import maru.core.ExecutionPayload
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.LatestBlockMetadata
import maru.executionlayer.manager.PayloadAttributes
import maru.executionlayer.manager.PayloadStatus
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Engine API client for a single execution-layer fork, expressed in Maru domain types only.
 *
 * Execution-layer JSON-RPC errors, transport failures and timeouts fail the returned future.
 */
interface EngineApiClient {
  val fork: ElFork

  fun getPayload(payloadId: ByteArray): SafeFuture<ExecutionPayload>

  fun newPayload(executionPayload: ExecutionPayload): SafeFuture<PayloadStatus>

  fun forkChoiceUpdate(
    headBlockHash: ByteArray,
    safeBlockHash: ByteArray,
    finalizedBlockHash: ByteArray,
    payloadAttributes: PayloadAttributes?,
  ): SafeFuture<ForkChoiceUpdatedResult>

  fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata>

  fun isOnline(): SafeFuture<Boolean> =
    getLatestBlockMetadata()
      .thenApply { true }
      .exceptionally { false }
}
