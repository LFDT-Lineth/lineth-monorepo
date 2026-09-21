/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import linea.teku.Web3jClient
import maru.executionlayer.manager.LatestBlockMetadata
import maru.metrics.MaruMetricsCategory
import net.consensys.linea.metrics.DynamicTagTimer
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import tech.pegasys.teku.ethereum.executionclient.ExecutionEngineClient
import tech.pegasys.teku.ethereum.executionclient.schema.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture

abstract class BaseWeb3JJsonRpcExecutionLayerEngineApiClient(
  protected val web3jClient: Web3jClient,
  protected val metricsFacade: MetricsFacade,
) : ExecutionLayerEngineApiClient {
  protected val web3jEngineClient: ExecutionEngineClient = web3jClient

  protected fun <T> createRequestTimer(method: String): DynamicTagTimer<Response<T>> =
    metricsFacade.createDynamicTagTimer<Response<T>>(
      category = MaruMetricsCategory.ENGINE_API,
      name = "request.latency",
      description = "Execution Engine API request latency",
      commonTags = listOf(
        Tag("fork", getFork().name),
        Tag("endpoint", web3jClient.endpoint),
        Tag("method", method),
      ),
      tagValueExtractor = {
        when {
          it.payload != null -> listOf(Tag("status", "success"))
          else -> listOf(Tag("status", "failure"))
        }
      },
      tagValueExtractorOnError = { listOf(Tag("status", "failure")) },
    )

  override fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata> =
    web3jEngineClient.powChainHead.thenApply { powBlock ->
      LatestBlockMetadata(
        blockHash = powBlock.blockHash.toArray(),
        timestamp = powBlock.blockTimestamp.longValue().toULong(),
      )
    }

  override fun isOnline(): SafeFuture<Boolean> =
    this
      .getLatestBlockMetadata()
      .thenApply { true }
      .exceptionally { false }
}
