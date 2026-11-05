/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager

import maru.consensus.ElFork
import maru.core.EMPTY_HASH
import maru.core.ext.DataGenerators
import maru.executionlayer.client.EngineApiClient
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.ValueSource
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.isNull
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import kotlin.random.Random
import kotlin.random.nextULong

class JsonRpcExecutionLayerManagerTest {
  private lateinit var engineApiClient: EngineApiClient
  private lateinit var executionLayerManager: ExecutionLayerManager

  private val feeRecipient: ByteArray = Random.nextBytes(20)
  private val headHash = Random.nextBytes(32)
  private val safeHash = Random.nextBytes(32)
  private val finalizedHash = Random.nextBytes(32)

  @BeforeEach
  fun setUp() {
    engineApiClient = mock { on { fork } doReturn ElFork.Prague }
    executionLayerManager = JsonRpcExecutionLayerManager(engineApiClient)
  }

  private fun validStatus() = PayloadStatus(ExecutionPayloadStatus.VALID, Random.nextBytes(32), validationError = null)

  private fun mockForkChoiceUpdate(payloadId: ByteArray?): ForkChoiceUpdatedResult {
    val result = ForkChoiceUpdatedResult(validStatus(), payloadId)
    whenever(engineApiClient.forkChoiceUpdate(any(), any(), any(), anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(result))
    return result
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
    val expectedResult = mockForkChoiceUpdate(payloadId)

    assertThat(startBlockBuilding().get()).isEqualTo(expectedResult)

    val executionPayload = DataGenerators.randomExecutionPayload()
    whenever(engineApiClient.getPayload(eq(payloadId))).thenReturn(SafeFuture.completedFuture(executionPayload))

    assertThat(executionLayerManager.finishBlockBuilding().get()).isEqualTo(executionPayload)
    verify(engineApiClient).getPayload(eq(payloadId))
  }

  @ParameterizedTest(name = "FCU argument propagation with Amsterdam attributes = {0}")
  @ValueSource(booleans = [false, true])
  fun `setHeadAndStartBlockBuilding passes arguments to FCU correctly`(amsterdam: Boolean) {
    val nextTimestamp = Random.nextULong(0U, ULong.MAX_VALUE)
    val nextSlot = if (amsterdam) Random.nextULong(0U, ULong.MAX_VALUE) else null
    val targetGasLimit = if (amsterdam) 60_000_000UL else null
    val expectedResult = mockForkChoiceUpdate(Random.nextBytes(8))

    val result = startBlockBuilding(nextTimestamp, nextSlot, targetGasLimit).get()

    assertThat(result).isEqualTo(expectedResult)
    verify(engineApiClient).forkChoiceUpdate(
      eq(headHash),
      eq(safeHash),
      eq(finalizedHash),
      eq(
        PayloadAttributes(
          timestamp = nextTimestamp,
          prevRandao = EMPTY_HASH,
          suggestedFeeRecipient = feeRecipient,
          slotNumber = nextSlot,
          targetGasLimit = targetGasLimit,
        ),
      ),
    )
  }

  @Test
  fun `setHeadAndStartBlockBuilding fails when FCU returns no payloadId`() {
    mockForkChoiceUpdate(payloadId = null)

    val exception = assertThrows<ExecutionException> { startBlockBuilding().get() }

    assertThat(exception).cause().hasMessageStartingWith("Unexpected FCU result. Payload ID is null!")
    assertThat(executionLayerManager.finishBlockBuilding()).isCompletedExceptionally()
  }

  @Test
  fun `finishBlockBuilding waits for FCU to complete before calling getPayload`() {
    val pendingFcu = SafeFuture<ForkChoiceUpdatedResult>()
    whenever(engineApiClient.forkChoiceUpdate(any(), any(), any(), anyOrNull())).thenReturn(pendingFcu)

    startBlockBuilding()
    val finishFuture = executionLayerManager.finishBlockBuilding()

    verify(engineApiClient, never()).getPayload(any())
    assertThat(finishFuture.isDone).isFalse()

    val payloadId = Random.nextBytes(8)
    val executionPayload = DataGenerators.randomExecutionPayload()
    whenever(engineApiClient.getPayload(eq(payloadId))).thenReturn(SafeFuture.completedFuture(executionPayload))
    pendingFcu.complete(ForkChoiceUpdatedResult(validStatus(), payloadId))

    assertThat(finishFuture.get()).isEqualTo(executionPayload)
    verify(engineApiClient, times(1)).getPayload(eq(payloadId))
  }

  @Test
  fun `finishBlockBuilding can't be called before setHeadAndStartBlockBuilding`() {
    assertThat(executionLayerManager.finishBlockBuilding().isCompletedExceptionally).isTrue()
  }

  @Test
  fun `finishBlockBuilding reports getPayload failures with context`() {
    val payloadId = Random.nextBytes(8)
    mockForkChoiceUpdate(payloadId)
    whenever(engineApiClient.getPayload(eq(payloadId)))
      .thenReturn(SafeFuture.failedFuture(JsonRpcErrorResponseException(-38001, "Unknown payload", null)))
    startBlockBuilding().get()

    val exception = assertThrows<ExecutionException> { executionLayerManager.finishBlockBuilding().get() }

    assertThat(exception.cause)
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessage(
        "engine_getPayload request failed: fork=Prague " +
          "Cause: code=-38001 message=Unknown payload errorData=null",
      )
      .hasCauseInstanceOf(JsonRpcErrorResponseException::class.java)
  }

  @Test
  fun `newPayload forwards the call and returns the status`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    val status = validStatus()
    whenever(engineApiClient.newPayload(any())).thenReturn(SafeFuture.completedFuture(status))

    assertThat(executionLayerManager.newPayload(executionPayload).get()).isEqualTo(status)
    verify(engineApiClient, times(1)).newPayload(eq(executionPayload))
  }

  @Test
  fun `newPayload returns invalid status with validation error`() {
    val status = PayloadStatus(ExecutionPayloadStatus.INVALID, Random.nextBytes(32), "Invalid payload")
    whenever(engineApiClient.newPayload(any())).thenReturn(SafeFuture.completedFuture(status))

    val result = executionLayerManager.newPayload(DataGenerators.randomExecutionPayload()).get()

    assertThat(result.validationError).isEqualTo("Invalid payload")
  }

  @Test
  fun `newPayload reports request failures with context`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    whenever(engineApiClient.newPayload(any()))
      .thenReturn(SafeFuture.failedFuture(JsonRpcErrorResponseException(-32000, "Unexpected error!", null)))

    val exception = assertThrows<ExecutionException> { executionLayerManager.newPayload(executionPayload).get() }

    assertThat(exception).cause().hasMessage(
      "engine_newPayload request failed: elBlockNumber=${executionPayload.blockNumber} fork=Prague " +
        "Cause: code=-32000 message=Unexpected error! errorData=null",
    )
  }

  @Test
  fun `newPayload discards in-progress block building`() {
    mockForkChoiceUpdate(Random.nextBytes(8))
    startBlockBuilding().get()
    whenever(engineApiClient.newPayload(any())).thenReturn(SafeFuture.completedFuture(validStatus()))

    executionLayerManager.newPayload(DataGenerators.randomExecutionPayload()).get()

    assertThat(executionLayerManager.finishBlockBuilding()).isCompletedExceptionally()
  }

  @Test
  fun `setHead updates fork choice state without payload attributes`() {
    val expectedResult = mockForkChoiceUpdate(payloadId = null)

    val result = executionLayerManager.setHead(headHash, safeHash, finalizedHash).get()

    assertThat(result).isEqualTo(expectedResult)
    verify(engineApiClient).forkChoiceUpdate(eq(headHash), eq(safeHash), eq(finalizedHash), isNull())
  }

  @Test
  fun `setHead reports request failures with context`() {
    whenever(engineApiClient.forkChoiceUpdate(any(), any(), any(), anyOrNull()))
      .thenReturn(SafeFuture.failedFuture(RuntimeException("connection refused")))

    val exception = assertThrows<ExecutionException> {
      executionLayerManager.setHead(headHash, safeHash, finalizedHash).get()
    }

    assertThat(exception).cause().hasMessage(
      "engine_forkchoiceUpdated request failed! nextBlockTimestamp=null fork=Prague Cause: connection refused",
    )
  }
}
