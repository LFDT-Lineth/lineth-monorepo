/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager

import maru.consensus.ElFork
import maru.core.ExecutionPayload
import maru.executionlayer.client.EngineApiClient
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList

/**
 * In-memory [EngineApiClient] that records every call and answers with the scripted responses. Unscripted calls fail.
 */
class FakeEngineApiClient(
  override val fork: ElFork = ElFork.Prague,
) : EngineApiClient {
  class ForkChoiceUpdateCall(
    val headBlockHash: ByteArray,
    val safeBlockHash: ByteArray,
    val finalizedBlockHash: ByteArray,
    val payloadAttributes: PayloadAttributes?,
  )

  val forkChoiceUpdateCalls = CopyOnWriteArrayList<ForkChoiceUpdateCall>()
  val getPayloadCalls = CopyOnWriteArrayList<ByteArray>()
  val newPayloadCalls = CopyOnWriteArrayList<ExecutionPayload>()

  var forkChoiceUpdateResponse: (ForkChoiceUpdateCall) -> SafeFuture<ForkChoiceUpdatedResult> =
    { notScripted("forkChoiceUpdate") }
  var getPayloadResponse: (payloadId: ByteArray) -> SafeFuture<ExecutionPayload> = { notScripted("getPayload") }
  var newPayloadResponse: (ExecutionPayload) -> SafeFuture<PayloadStatus> = { notScripted("newPayload") }
  var latestBlockMetadataResponse: () -> SafeFuture<LatestBlockMetadata> = { notScripted("getLatestBlockMetadata") }

  override fun getPayload(payloadId: ByteArray): SafeFuture<ExecutionPayload> {
    getPayloadCalls.add(payloadId)
    return getPayloadResponse(payloadId)
  }

  override fun newPayload(executionPayload: ExecutionPayload): SafeFuture<PayloadStatus> {
    newPayloadCalls.add(executionPayload)
    return newPayloadResponse(executionPayload)
  }

  override fun forkChoiceUpdate(
    headBlockHash: ByteArray,
    safeBlockHash: ByteArray,
    finalizedBlockHash: ByteArray,
    payloadAttributes: PayloadAttributes?,
  ): SafeFuture<ForkChoiceUpdatedResult> {
    val call = ForkChoiceUpdateCall(headBlockHash, safeBlockHash, finalizedBlockHash, payloadAttributes)
    forkChoiceUpdateCalls.add(call)
    return forkChoiceUpdateResponse(call)
  }

  override fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata> = latestBlockMetadataResponse()

  private fun <T> notScripted(method: String): SafeFuture<T> =
    SafeFuture.failedFuture(IllegalStateException("FakeEngineApiClient: no response scripted for $method"))
}
