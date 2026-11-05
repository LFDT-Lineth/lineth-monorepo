/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager

import maru.core.EMPTY_HASH
import maru.core.ExecutionPayload
import maru.core.ext.DataGenerators
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.ValueSource
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import kotlin.random.Random
import kotlin.random.nextULong

class JsonRpcExecutionLayerManagerTest {
  private lateinit var engineApiClient: FakeEngineApiClient
  private lateinit var executionLayerManager: ExecutionLayerManager

  private val feeRecipient: ByteArray = Random.nextBytes(20)
  private val headHash = Random.nextBytes(32)
  private val safeHash = Random.nextBytes(32)
  private val finalizedHash = Random.nextBytes(32)

  @BeforeEach
  fun setUp() {
    engineApiClient = FakeEngineApiClient()
    executionLayerManager = JsonRpcExecutionLayerManager(engineApiClient)
  }

  private fun validStatus() = PayloadStatus(ExecutionPayloadStatus.VALID, Random.nextBytes(32), validationError = null)

  private fun respondToForkChoiceUpdate(payloadId: ByteArray?): ForkChoiceUpdatedResult {
    val result = ForkChoiceUpdatedResult(validStatus(), payloadId)
    engineApiClient.forkChoiceUpdateResponse = { SafeFuture.completedFuture(result) }
    return result
  }

  private fun respondToGetPayload(executionPayload: ExecutionPayload) {
    engineApiClient.getPayloadResponse = { SafeFuture.completedFuture(executionPayload) }
  }

  private fun respondToNewPayload(status: PayloadStatus) {
    engineApiClient.newPayloadResponse = { SafeFuture.completedFuture(status) }
  }

  private fun assertForkChoiceUpdatedWith(payloadAttributes: PayloadAttributes?) {
    assertThat(engineApiClient.forkChoiceUpdateCalls).hasSize(1)
    val call = engineApiClient.forkChoiceUpdateCalls.single()
    assertThat(call.headBlockHash).isEqualTo(headHash)
    assertThat(call.safeBlockHash).isEqualTo(safeHash)
    assertThat(call.finalizedBlockHash).isEqualTo(finalizedHash)
    assertThat(call.payloadAttributes).isEqualTo(payloadAttributes)
  }

  private fun startBlockBuilding(
    nextBlockTimestamp: ULong = 0UL,
    nextBlockSlotNumber: ULong? = null,
    targetGasLimit: ULong? = null,
  ): SafeFuture<ForkChoiceUpdatedResult> =
    executionLayerManager.setHeadAndStartBlockBuilding(
      headHash = headHash,
      safeHash = safeHash,
      finalizedHash = finalizedHash,
      nextBlockTimestamp = nextBlockTimestamp,
      feeRecipient = feeRecipient,
      nextBlockSlotNumber = nextBlockSlotNumber,
      targetGasLimit = targetGasLimit,
    )

  @Test
  fun `setHeadAndStartBlockBuilding stores payloadId for finishBlockBuilding`() {
    val payloadId = Random.nextBytes(8)
    val expectedResult = respondToForkChoiceUpdate(payloadId)

    assertThat(startBlockBuilding().get()).isEqualTo(expectedResult)

    val executionPayload = DataGenerators.randomExecutionPayload()
    respondToGetPayload(executionPayload)

    assertThat(executionLayerManager.finishBlockBuilding().get()).isEqualTo(executionPayload)
    assertThat(engineApiClient.getPayloadCalls).hasSize(1)
    assertThat(engineApiClient.getPayloadCalls.single()).isEqualTo(payloadId)
  }

  @ParameterizedTest(name = "FCU argument propagation with Amsterdam attributes = {0}")
  @ValueSource(booleans = [false, true])
  fun `setHeadAndStartBlockBuilding passes arguments to FCU correctly`(amsterdam: Boolean) {
    val nextTimestamp = Random.nextULong(0U, ULong.MAX_VALUE)
    val nextSlot = if (amsterdam) Random.nextULong(0U, ULong.MAX_VALUE) else null
    val targetGasLimit = if (amsterdam) 60_000_000UL else null
    val expectedResult = respondToForkChoiceUpdate(Random.nextBytes(8))

    val result = startBlockBuilding(nextTimestamp, nextSlot, targetGasLimit).get()

    assertThat(result).isEqualTo(expectedResult)
    assertForkChoiceUpdatedWith(
      PayloadAttributes(
        timestamp = nextTimestamp,
        prevRandao = EMPTY_HASH,
        suggestedFeeRecipient = feeRecipient,
        slotNumber = nextSlot,
        targetGasLimit = targetGasLimit,
      ),
    )
  }

  @Test
  fun `setHeadAndStartBlockBuilding fails when FCU returns no payloadId`() {
    respondToForkChoiceUpdate(payloadId = null)

    val exception = assertThrows<ExecutionException> { startBlockBuilding().get() }

    assertThat(exception).cause().hasMessageStartingWith("Unexpected FCU result. Payload ID is null!")
    assertThat(executionLayerManager.finishBlockBuilding()).isCompletedExceptionally()
  }

  @Test
  fun `finishBlockBuilding waits for FCU to complete before calling getPayload`() {
    val pendingFcu = SafeFuture<ForkChoiceUpdatedResult>()
    engineApiClient.forkChoiceUpdateResponse = { pendingFcu }

    startBlockBuilding()
    val finishFuture = executionLayerManager.finishBlockBuilding()

    assertThat(engineApiClient.getPayloadCalls).isEmpty()
    assertThat(finishFuture.isDone).isFalse()

    val payloadId = Random.nextBytes(8)
    val executionPayload = DataGenerators.randomExecutionPayload()
    respondToGetPayload(executionPayload)
    pendingFcu.complete(ForkChoiceUpdatedResult(validStatus(), payloadId))

    assertThat(finishFuture.get()).isEqualTo(executionPayload)
    assertThat(engineApiClient.getPayloadCalls).hasSize(1)
    assertThat(engineApiClient.getPayloadCalls.single()).isEqualTo(payloadId)
  }

  @Test
  fun `finishBlockBuilding can't be called before setHeadAndStartBlockBuilding`() {
    assertThat(executionLayerManager.finishBlockBuilding().isCompletedExceptionally).isTrue()
    assertThat(engineApiClient.getPayloadCalls).isEmpty()
  }

  @Test
  fun `finishBlockBuilding reports getPayload failures with context`() {
    respondToForkChoiceUpdate(Random.nextBytes(8))
    engineApiClient.getPayloadResponse = {
      SafeFuture.failedFuture(JsonRpcErrorResponseException(-38001, "Unknown payload", null))
    }
    startBlockBuilding().get()

    val exception = assertThrows<ExecutionException> { executionLayerManager.finishBlockBuilding().get() }

    assertThat(exception.cause)
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessage(
        "engine_getPayload request failed: fork=Prague " +
          "Cause: code=-38001 message=Unknown payload errorData=null",
      ).hasCauseInstanceOf(JsonRpcErrorResponseException::class.java)
  }

  @Test
  fun `newPayload forwards the call and returns the status`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    val status = validStatus()
    respondToNewPayload(status)

    assertThat(executionLayerManager.newPayload(executionPayload).get()).isEqualTo(status)
    assertThat(engineApiClient.newPayloadCalls).containsExactly(executionPayload)
  }

  @Test
  fun `newPayload returns invalid status with validation error`() {
    respondToNewPayload(PayloadStatus(ExecutionPayloadStatus.INVALID, Random.nextBytes(32), "Invalid payload"))

    val result = executionLayerManager.newPayload(DataGenerators.randomExecutionPayload()).get()

    assertThat(result.validationError).isEqualTo("Invalid payload")
  }

  @Test
  fun `newPayload reports request failures with context`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    engineApiClient.newPayloadResponse = {
      SafeFuture.failedFuture(JsonRpcErrorResponseException(-32000, "Unexpected error!", null))
    }

    val exception = assertThrows<ExecutionException> { executionLayerManager.newPayload(executionPayload).get() }

    assertThat(exception).cause().hasMessage(
      "engine_newPayload request failed: elBlockNumber=${executionPayload.blockNumber} fork=Prague " +
        "Cause: code=-32000 message=Unexpected error! errorData=null",
    )
  }

  @Test
  fun `newPayload discards in-progress block building`() {
    respondToForkChoiceUpdate(Random.nextBytes(8))
    startBlockBuilding().get()
    respondToNewPayload(validStatus())

    executionLayerManager.newPayload(DataGenerators.randomExecutionPayload()).get()

    assertThat(executionLayerManager.finishBlockBuilding()).isCompletedExceptionally()
    assertThat(engineApiClient.getPayloadCalls).isEmpty()
  }

  @Test
  fun `setHead updates fork choice state without payload attributes`() {
    val expectedResult = respondToForkChoiceUpdate(payloadId = null)

    val result = executionLayerManager.setHead(headHash, safeHash, finalizedHash).get()

    assertThat(result).isEqualTo(expectedResult)
    assertForkChoiceUpdatedWith(payloadAttributes = null)
  }

  @Test
  fun `setHead reports request failures with context`() {
    engineApiClient.forkChoiceUpdateResponse = {
      SafeFuture.failedFuture(RuntimeException("connection refused"))
    }

    val exception = assertThrows<ExecutionException> {
      executionLayerManager.setHead(headHash, safeHash, finalizedHash).get()
    }

    assertThat(exception).cause().hasMessage(
      "engine_forkchoiceUpdated request failed! nextBlockTimestamp=null fork=Prague Cause: connection refused",
    )
  }
}
