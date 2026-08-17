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
import maru.executionlayer.mappers.Mappers.toDomainExecutionPayload
import maru.executionlayer.mappers.Mappers.toExecutionPayloadV2
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.metrics.MetricsFacade
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceUpdatedResult
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV2
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadStatusV1
import tech.pegasys.teku.ethereum.executionclient.schema.Response
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.bytes.Bytes8
import java.util.Optional

// https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md
class ShanghaiWeb3JJsonRpcExecutionLayerEngineApiClient(
  web3jClient: Web3JClient,
  metricsFacade: MetricsFacade,
) : BaseWeb3JJsonRpcExecutionLayerEngineApiClient(web3jClient = web3jClient, metricsFacade = metricsFacade) {
  override fun getFork(): ElFork = ElFork.Shanghai

  override fun getPayload(payloadId: Bytes8): SafeFuture<Response<ExecutionPayload>> =
    createRequestTimer<ExecutionPayload>(method = "getPayload").captureTime(
      web3jEngineClient.getPayloadV2(payloadId).thenApply {
        when {
          it.payload != null ->
            Response.fromPayloadReceivedAsJson(it.payload.executionPayload.toDomainExecutionPayload())

          it.errorMessage != null ->
            Response.fromErrorMessage(it.errorMessage)

          else ->
            throw IllegalStateException("Failed to get payload!")
        }
      },
    ).toSafeFuture()

  override fun newPayload(executionPayload: ExecutionPayload): SafeFuture<Response<PayloadStatusV1>> =
    createRequestTimer<PayloadStatusV1>(method = "newPayload").captureTime(
      web3jEngineClient
        .newPayloadV2(executionPayload.toExecutionPayloadV2())
        .thenApply {
          if (it.payload != null) {
            Response.fromPayloadReceivedAsJson(it.payload)
          } else {
            Response.fromErrorMessage(it.errorMessage)
          }
        },
    ).toSafeFuture()

  override fun forkChoiceUpdate(
    forkChoiceState: ForkChoiceStateV1,
    payloadAttributes: PayloadAttributesV1?,
  ): SafeFuture<Response<ForkChoiceUpdatedResult>> =
    createRequestTimer<ForkChoiceUpdatedResult>(
      method = "forkChoiceUpdate",
    ).captureTime(
      web3jEngineClient.forkChoiceUpdatedV2(forkChoiceState, Optional.ofNullable(payloadAttributes?.toV2())),
    ).toSafeFuture()

  private fun PayloadAttributesV1.toV2(): PayloadAttributesV2 =
    PayloadAttributesV2(
      this.timestamp,
      this.prevRandao,
      this.suggestedFeeRecipient,
      emptyList(),
    )
}
