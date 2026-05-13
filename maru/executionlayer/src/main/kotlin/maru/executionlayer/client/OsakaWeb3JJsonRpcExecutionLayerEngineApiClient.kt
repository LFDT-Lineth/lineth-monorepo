/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import maru.consensus.ElFork
import maru.core.ExecutionPayload
import maru.extensions.captureTimeSafeFuture
import maru.mappers.Mappers.toDomainExecutionPayload
import net.consensys.linea.metrics.MetricsFacade
import tech.pegasys.teku.ethereum.executionclient.schema.Response
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.bytes.Bytes8

class OsakaWeb3JJsonRpcExecutionLayerEngineApiClient(
  web3jClient: Web3JClient,
  metricsFacade: MetricsFacade,
) : PragueWeb3JJsonRpcExecutionLayerEngineApiClient(web3jClient = web3jClient, metricsFacade = metricsFacade) {
  override fun getFork(): ElFork = ElFork.Osaka

  override fun getPayload(payloadId: Bytes8): SafeFuture<Response<ExecutionPayload>> =
    createRequestTimer<ExecutionPayload>(method = "getPayload").captureTimeSafeFuture(
      web3jEngineClient.getPayloadV5(payloadId).thenApply {
        when {
          it.payload != null ->
            Response.fromPayloadReceivedAsJson(it.payload.executionPayload.toDomainExecutionPayload())

          it.errorMessage != null ->
            Response.fromErrorMessage(it.errorMessage)

          else ->
            throw IllegalStateException("Failed to get payload!")
        }
      },
    )
}
