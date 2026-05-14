/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager

import maru.core.ExecutionPayload
import maru.executionlayer.client.ExecutionLayerEngineApiClient
import maru.mappers.Mappers.toDomain
import maru.mappers.Mappers.toPayloadAttributesV1
import org.apache.logging.log4j.LogManager
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.bytes.Bytes8
import java.util.concurrent.atomic.AtomicReference

class JsonRpcExecutionLayerManager(
  private val executionLayerEngineApiClient: ExecutionLayerEngineApiClient,
) : ExecutionLayerManager {
  private val log = LogManager.getLogger(this.javaClass)

  private var blockBuildingFuture = AtomicReference<SafeFuture<ByteArray?>?>()

  override fun setHeadAndStartBlockBuilding(
    headHash: ByteArray,
    safeHash: ByteArray,
    finalizedHash: ByteArray,
    nextBlockTimestamp: ULong,
    feeRecipient: ByteArray,
    prevRandao: ByteArray,
  ): SafeFuture<ForkChoiceUpdatedResult> {
    log.debug(
      "Trying to create a new block with timestamp={}, fork={}",
      nextBlockTimestamp,
      executionLayerEngineApiClient.getFork(),
    )
    val payloadAttributes =
      PayloadAttributes(
        timestamp = nextBlockTimestamp,
        suggestedFeeRecipient = feeRecipient,
        prevRandao = prevRandao,
      )
    log.debug(
      "Starting block building with payloadAttributes={}, fork={}",
      payloadAttributes,
      executionLayerEngineApiClient.getFork(),
    )
    val fcuFuture =
      forkChoiceUpdate(headHash, safeHash, finalizedHash, payloadAttributes).thenPeek {
        if (it.payloadId == null) {
          throw IllegalStateException("Unexpected FCU result. Payload ID is null! $it")
        } else {
          log.debug(
            "payloadId={}, nextBlockTimestamp={}, fork={}",
            Bytes.wrap(it.payloadId!!).toHexString(),
            nextBlockTimestamp,
            executionLayerEngineApiClient.getFork(),
          )
        }
      }
    blockBuildingFuture.set(fcuFuture.thenApply { it.payloadId })
    return fcuFuture
  }

  override fun finishBlockBuilding(): SafeFuture<ExecutionPayload> {
    val future = blockBuildingFuture.getAndSet(null)
    if (future == null) {
      log.warn("finishBlockBuilding called but no block building was started")
      return SafeFuture.failedFuture(
        IllegalStateException(
          "finishBlockBuilding called with no block building in progress",
        ),
      )
    }

    return future
      .thenCompose { pid ->
        if (pid == null) {
          SafeFuture.failedFuture(
            IllegalStateException(
              "finishBlockBuilding: FCU did not return a payloadId",
            ),
          )
        } else {
          log.debug("finishBlockBuilding using payloadId={}", Bytes.wrap(pid).toHexString())
          executionLayerEngineApiClient
            .getPayload(Bytes8(Bytes.wrap(pid)))
            .thenApply { payloadResponse ->
              if (payloadResponse.isSuccess) {
                payloadResponse.payload
              } else {
                throw IllegalStateException(
                  "engine_getPayload request failed: " +
                    "fork=${executionLayerEngineApiClient.getFork()} " +
                    "Cause: " + payloadResponse.errorMessage,
                )
              }
            }
        }
      }
  }

  override fun setHead(
    headHash: ByteArray,
    safeHash: ByteArray,
    finalizedHash: ByteArray,
  ): SafeFuture<ForkChoiceUpdatedResult> {
    blockBuildingFuture.set(null)
    return forkChoiceUpdate(
      headHash = headHash,
      safeHash = safeHash,
      finalizedHash = finalizedHash,
      payloadAttributes = null,
    )
  }

  private fun forkChoiceUpdate(
    headHash: ByteArray,
    safeHash: ByteArray,
    finalizedHash: ByteArray,
    payloadAttributes: PayloadAttributes?,
  ): SafeFuture<ForkChoiceUpdatedResult> =
    executionLayerEngineApiClient
      .forkChoiceUpdate(
        ForkChoiceStateV1(
          Bytes32.wrap(headHash),
          Bytes32.wrap(safeHash),
          Bytes32.wrap(finalizedHash),
        ),
        payloadAttributes?.toPayloadAttributesV1(),
      ).thenApply { response ->
        log.debug(
          "engine_forkchoiceUpdated response={} fork={}",
          response,
          executionLayerEngineApiClient.getFork(),
        )
        if (response.isFailure) {
          throw IllegalStateException(
            "engine_forkchoiceUpdated request failed! nextBlockTimestamp=${payloadAttributes?.timestamp} " +
              "fork=${executionLayerEngineApiClient.getFork()} " +
              "Cause: " + response.errorMessage,
          )
        } else {
          response.payload.toDomain()
        }
      }

  override fun newPayload(executionPayload: ExecutionPayload): SafeFuture<PayloadStatus> =
    executionLayerEngineApiClient
      .newPayload(executionPayload)
      .thenApply { payloadStatusResponse ->
        if (payloadStatusResponse.isSuccess) {
          if (payloadStatusResponse.payload == null) {
            throw IllegalStateException(
              "engine_newPayload request failed! elBlockNumber=${executionPayload.blockNumber} " +
                "fork=${executionLayerEngineApiClient.getFork()} " +
                "response=" + payloadStatusResponse,
            )
          }
          payloadStatusResponse.payload.asInternalExecutionPayload().toDomain()
        } else {
          throw IllegalStateException(
            "engine_newPayload request failed: elBlockNumber=${executionPayload.blockNumber} " +
              "fork=${executionLayerEngineApiClient.getFork()} " +
              "Cause: " + payloadStatusResponse.errorMessage,
          )
        }
      }.thenPeek {
        // A block was successfully imported; any in-progress build is now stale.
        blockBuildingFuture.set(null)
      }

  override fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata> =
    executionLayerEngineApiClient.getLatestBlockMetadata()

  override fun isOnline(): SafeFuture<Boolean> = executionLayerEngineApiClient.isOnline()
}
