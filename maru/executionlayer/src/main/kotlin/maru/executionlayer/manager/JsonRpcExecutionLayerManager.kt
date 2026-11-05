/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager

import linea.kotlin.encodeHex
import maru.core.ExecutionPayload
import maru.executionlayer.client.EngineApiClient
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletionException
import java.util.concurrent.atomic.AtomicReference

class JsonRpcExecutionLayerManager(
  private val engineApiClient: EngineApiClient,
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
    nextBlockSlotNumber: ULong?,
    targetGasLimit: ULong?,
  ): SafeFuture<ForkChoiceUpdatedResult> {
    val payloadAttributes =
      PayloadAttributes(
        timestamp = nextBlockTimestamp,
        suggestedFeeRecipient = feeRecipient,
        prevRandao = prevRandao,
        slotNumber = nextBlockSlotNumber,
        targetGasLimit = targetGasLimit,
      )
    log.debug(
      "Starting block building with payloadAttributes={}, fork={}",
      payloadAttributes,
      engineApiClient.fork,
    )
    val fcuFuture =
      forkChoiceUpdate(headHash, safeHash, finalizedHash, payloadAttributes).thenPeek {
        checkNotNull(it.payloadId) { "Unexpected FCU result. Payload ID is null! $it" }
        log.debug(
          "payloadId={}, nextBlockTimestamp={}, fork={}",
          it.payloadId!!.encodeHex(),
          nextBlockTimestamp,
          engineApiClient.fork,
        )
      }
    blockBuildingFuture.set(fcuFuture.thenApply { it.payloadId })
    return fcuFuture
  }

  override fun finishBlockBuilding(): SafeFuture<ExecutionPayload> {
    val future = blockBuildingFuture.getAndSet(null)
    if (future == null) {
      log.warn("finishBlockBuilding called but no block building was started")
      return SafeFuture.failedFuture(
        IllegalStateException("finishBlockBuilding called with no block building in progress"),
      )
    }

    return future.thenCompose { payloadId ->
      if (payloadId == null) {
        SafeFuture.failedFuture(IllegalStateException("finishBlockBuilding: FCU did not return a payloadId"))
      } else {
        log.debug("finishBlockBuilding using payloadId={}", payloadId.encodeHex())
        engineApiClient
          .getPayload(payloadId)
          .failWithContext { "engine_getPayload request failed: fork=${engineApiClient.fork}" }
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
    engineApiClient
      .forkChoiceUpdate(headHash, safeHash, finalizedHash, payloadAttributes)
      .thenPeek { log.debug("engine_forkchoiceUpdated result={} fork={}", it, engineApiClient.fork) }
      .failWithContext {
        "engine_forkchoiceUpdated request failed! nextBlockTimestamp=${payloadAttributes?.timestamp} " +
          "fork=${engineApiClient.fork}"
      }

  override fun newPayload(executionPayload: ExecutionPayload): SafeFuture<PayloadStatus> =
    engineApiClient
      .newPayload(executionPayload)
      .failWithContext {
        "engine_newPayload request failed: elBlockNumber=${executionPayload.blockNumber} fork=${engineApiClient.fork}"
      }.thenPeek {
        // A block was successfully imported; any in-progress build is now stale.
        blockBuildingFuture.set(null)
      }

  override fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata> = engineApiClient.getLatestBlockMetadata()

  override fun isOnline(): SafeFuture<Boolean> = engineApiClient.isOnline()

  private fun <T> SafeFuture<T>.failWithContext(context: () -> String): SafeFuture<T> =
    exceptionally { error ->
      val cause = if (error is CompletionException && error.cause != null) error.cause!! else error
      throw IllegalStateException("${context()} Cause: ${cause.message}", cause)
    }
}
