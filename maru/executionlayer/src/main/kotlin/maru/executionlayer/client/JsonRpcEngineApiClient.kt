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
import maru.metrics.MaruMetricsCategory
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.metrics.DynamicTagTimer
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URL

/**
 * Engine API client for one fork. A single [rpcClient] per execution-layer endpoint can be shared by the clients of
 * all forks.
 */
class JsonRpcEngineApiClient(
  private val rpcClient: JsonRpcV2Client,
  override val fork: ElFork,
  endpoint: URL,
  metricsFacade: MetricsFacade,
) : EngineApiClient {
  private val methods = EngineApiForkMethods.of(fork)
  private val codec = EngineApiCodec(methods)
  private val getPayloadTimer = requestTimer<ExecutionPayload>(metricsFacade, endpoint, "getPayload")
  private val newPayloadTimer = requestTimer<PayloadStatus>(metricsFacade, endpoint, "newPayload")
  private val forkChoiceUpdateTimer = requestTimer<ForkChoiceUpdatedResult>(metricsFacade, endpoint, "forkChoiceUpdate")

  override fun getPayload(payloadId: ByteArray): SafeFuture<ExecutionPayload> =
    getPayloadTimer.timed {
      request(methods.getPayload, { codec.getPayloadParams(payloadId) }, codec::decodeGetPayloadResult)
    }

  override fun newPayload(executionPayload: ExecutionPayload): SafeFuture<PayloadStatus> =
    newPayloadTimer.timed {
      request(methods.newPayload, { codec.newPayloadParams(executionPayload) }, codec::decodeNewPayloadResult)
    }

  override fun forkChoiceUpdate(
    headBlockHash: ByteArray,
    safeBlockHash: ByteArray,
    finalizedBlockHash: ByteArray,
    payloadAttributes: PayloadAttributes?,
  ): SafeFuture<ForkChoiceUpdatedResult> =
    forkChoiceUpdateTimer.timed {
      request(
        methods.forkChoiceUpdated,
        { codec.forkChoiceUpdatedParams(headBlockHash, safeBlockHash, finalizedBlockHash, payloadAttributes) },
        codec::decodeForkChoiceUpdatedResult,
      )
    }

  override fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata> =
    request("eth_getBlockByNumber", { listOf("latest", false) }, EngineApiCodec::decodeLatestBlockMetadata)

  private fun <T> request(method: String, params: () -> List<Any?>, decode: (Any?) -> T): SafeFuture<T> =
    runCatching { rpcClient.makeRequest(method = method, params = params(), resultMapper = decode) }
      .getOrElse { SafeFuture.failedFuture(it) }

  private fun <T> DynamicTagTimer<T>.timed(request: () -> SafeFuture<T>): SafeFuture<T> =
    captureTime(request()).toSafeFuture()

  private fun <T> requestTimer(metricsFacade: MetricsFacade, endpoint: URL, method: String): DynamicTagTimer<T> =
    metricsFacade.createDynamicTagTimer(
      category = MaruMetricsCategory.ENGINE_API,
      name = "request.latency",
      description = "Execution Engine API request latency",
      commonTags = listOf(
        Tag("fork", fork.name),
        Tag("endpoint", endpoint.toString()),
        Tag("method", method),
      ),
      tagValueExtractor = { listOf(Tag("status", "success")) },
      tagValueExtractorOnError = { listOf(Tag("status", "failure")) },
    )
}
