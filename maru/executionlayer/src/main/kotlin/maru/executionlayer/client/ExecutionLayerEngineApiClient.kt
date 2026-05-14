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
import maru.executionlayer.manager.LatestBlockMetadata
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceUpdatedResult
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadStatusV1
import tech.pegasys.teku.ethereum.executionclient.schema.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.bytes.Bytes8

interface ExecutionLayerEngineApiClient {
  fun getPayload(payloadId: Bytes8): SafeFuture<Response<ExecutionPayload>>

  fun newPayload(executionPayload: ExecutionPayload): SafeFuture<Response<PayloadStatusV1>>

  fun forkChoiceUpdate(
    forkChoiceState: ForkChoiceStateV1,
    payloadAttributes: PayloadAttributesV1?,
  ): SafeFuture<Response<ForkChoiceUpdatedResult>>

  fun getLatestBlockHash(): SafeFuture<ByteArray> = getLatestBlockMetadata().thenApply { it.blockHash }

  fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata>

  fun isOnline(): SafeFuture<Boolean>

  fun getFork(): ElFork
}
