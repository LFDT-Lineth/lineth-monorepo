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
import maru.core.ext.DataGenerators
import maru.executionlayer.client.ExecutionLayerEngineApiClient
import maru.mappers.Mappers.toDomain
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.Mockito.atLeastOnce
import org.mockito.Mockito.mock
import org.mockito.Mockito.reset
import org.mockito.Mockito.verify
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.argThat
import org.mockito.kotlin.eq
import org.mockito.kotlin.isNull
import org.mockito.kotlin.times
import org.mockito.kotlin.whenever
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadStatusV1
import tech.pegasys.teku.ethereum.executionclient.schema.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.bytes.Bytes20
import tech.pegasys.teku.infrastructure.bytes.Bytes8
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import java.util.concurrent.ExecutionException
import kotlin.random.Random
import kotlin.random.nextULong
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceUpdatedResult as TekuForkChoiceUpdatedResult
import tech.pegasys.teku.spec.executionlayer.ExecutionPayloadStatus as TekuExecutionPayloadStatus

class JsonRpcExecutionLayerManagerTest {
  private lateinit var executionLayerEngineApiClient: ExecutionLayerEngineApiClient
  private lateinit var executionLayerManager: ExecutionLayerManager

  private val feeRecipient: ByteArray = Random.nextBytes(20)

  @BeforeEach
  fun setUp() {
    executionLayerEngineApiClient = mock()
    executionLayerManager = createExecutionLayerManager()
  }

  @AfterEach
  fun tearDown() {
    reset(executionLayerEngineApiClient)
  }

  private fun createExecutionLayerManager(): ExecutionLayerManager =
    JsonRpcExecutionLayerManager(
      executionLayerEngineApiClient = executionLayerEngineApiClient,
    )

  private fun mockForkChoiceUpdateWithValidStatus(payloadId: Bytes8?): PayloadStatusV1 {
    val latestValidHash = Bytes32.random()
    val executionStatus = TekuExecutionPayloadStatus.VALID
    val payloadStatus = PayloadStatusV1(executionStatus, latestValidHash, null)
    whenever(executionLayerEngineApiClient.forkChoiceUpdate(any(), anyOrNull()))
      .thenReturn(
        SafeFuture.completedFuture(
          Response.fromPayloadReceivedAsJson(TekuForkChoiceUpdatedResult(payloadStatus, payloadId)),
        ),
      )
    return payloadStatus
  }

  private fun mockGetPayloadWithRandomData(
    payloadId: Bytes8,
    executionPayload: ExecutionPayload,
  ) {
    val getPayloadResponse =
      Response.fromPayloadReceivedAsJson(
        executionPayload,
      )
    whenever(executionLayerEngineApiClient.getPayload(eq(payloadId)))
      .thenReturn(SafeFuture.completedFuture(getPayloadResponse))
  }

  private fun mockNewPayloadWithStatus(payloadStatus: PayloadStatusV1) {
    whenever(executionLayerEngineApiClient.newPayload(any())).thenReturn(
      SafeFuture.completedFuture(Response.fromPayloadReceivedAsJson(payloadStatus)),
    )
  }

  private fun mockFailedNewPayloadWithStatus(errorMessage: String) {
    whenever(executionLayerEngineApiClient.newPayload(any())).thenReturn(
      SafeFuture.completedFuture(Response.fromErrorMessage(errorMessage)),
    )
  }

  @Test
  fun `setHeadAndStartBlockBuilding stores payloadId for finishBlockBuilding`() {
    val newHeadHash = Bytes32.random()
    val newSafeHash = Bytes32.random()
    val newFinalizedHash = Bytes32.random()
    val nextTimestamp = 0UL

    val payloadId = Bytes8(Bytes.random(8))
    val payloadStatus = mockForkChoiceUpdateWithValidStatus(payloadId)

    val result =
      executionLayerManager
        .setHeadAndStartBlockBuilding(
          headHash = newHeadHash.toArray(),
          safeHash = newSafeHash.toArray(),
          finalizedHash = newFinalizedHash.toArray(),
          nextBlockTimestamp = nextTimestamp,
          feeRecipient = feeRecipient,
        ).get()

    val expectedPayloadStatus =
      PayloadStatus(
        ExecutionPayloadStatus.VALID,
        latestValidHash = payloadStatus
          .asInternalExecutionPayload()
          .latestValidHash
          .get()
          .toArray(),
        validationError = null,
      )
    val expectedResult = ForkChoiceUpdatedResult(expectedPayloadStatus, payloadId.wrappedBytes.toArray())
    assertThat(result).isEqualTo(expectedResult)

    val executionPayload = DataGenerators.randomExecutionPayload()
    mockGetPayloadWithRandomData(payloadId, executionPayload)
    mockNewPayloadWithStatus(payloadStatus)

    executionLayerManager.finishBlockBuilding().get()
    verify(executionLayerEngineApiClient, atLeastOnce()).getPayload(eq(payloadId))
  }

  @Test
  fun `setHeadAndStartBlockBuilding passes arguments to FCU correctly`() {
    val newHeadHash = Bytes32.random()
    val newSafeHash = Bytes32.random()
    val newFinalizedHash = Bytes32.random()
    val nextTimestamp = Random.nextULong(0U, ULong.MAX_VALUE)

    val payloadId = Bytes8(Bytes.random(8))
    val payloadStatus = mockForkChoiceUpdateWithValidStatus(payloadId)

    val result =
      executionLayerManager
        .setHeadAndStartBlockBuilding(
          headHash = newHeadHash.toArray(),
          safeHash = newSafeHash.toArray(),
          finalizedHash = newFinalizedHash.toArray(),
          nextBlockTimestamp = nextTimestamp,
          feeRecipient = feeRecipient,
        ).get()

    val expectedPayloadStatus =
      PayloadStatus(
        ExecutionPayloadStatus.VALID,
        latestValidHash = payloadStatus
          .asInternalExecutionPayload()
          .latestValidHash
          .get()
          .toArray(),
        validationError = null,
      )
    val expectedResult = ForkChoiceUpdatedResult(expectedPayloadStatus, payloadId.wrappedBytes.toArray())
    assertThat(result).isEqualTo(expectedResult)
    verify(executionLayerEngineApiClient, atLeastOnce()).forkChoiceUpdate(
      argThat { forkChoiceState ->
        forkChoiceState == ForkChoiceStateV1(newHeadHash, newSafeHash, newFinalizedHash)
      },
      argThat { payloadAttributes ->
        payloadAttributes ==
          PayloadAttributesV1(
            UInt64.fromLongBits(nextTimestamp.toLong()),
            Bytes32.ZERO,
            Bytes20(Bytes.wrap(feeRecipient)),
          )
      },
    )
  }

  @Test
  fun `finishBlockBuilding waits for FCU to complete before calling getPayload`() {
    val newHeadHash = Bytes32.random()
    val newSafeHash = Bytes32.random()
    val newFinalizedHash = Bytes32.random()

    val pendingFcu = SafeFuture<Response<TekuForkChoiceUpdatedResult>>()
    whenever(executionLayerEngineApiClient.forkChoiceUpdate(any(), anyOrNull()))
      .thenReturn(pendingFcu)

    executionLayerManager.setHeadAndStartBlockBuilding(
      headHash = newHeadHash.toArray(),
      safeHash = newSafeHash.toArray(),
      finalizedHash = newFinalizedHash.toArray(),
      nextBlockTimestamp = 0UL,
      feeRecipient = feeRecipient,
    )

    val finishFuture = executionLayerManager.finishBlockBuilding()

    // FCU hasn't completed yet — getPayload must NOT have been called
    verify(executionLayerEngineApiClient, times(0)).getPayload(any())
    assertThat(finishFuture.isDone).isFalse()

    // Now complete the FCU
    val payloadId = Bytes8(Bytes.random(8))
    val payloadStatus = PayloadStatusV1(TekuExecutionPayloadStatus.VALID, Bytes32.random(), null)
    val executionPayload = DataGenerators.randomExecutionPayload()
    mockGetPayloadWithRandomData(payloadId, executionPayload)
    pendingFcu.complete(Response.fromPayloadReceivedAsJson(TekuForkChoiceUpdatedResult(payloadStatus, payloadId)))

    val result = finishFuture.get()
    assertThat(result).isEqualTo(executionPayload)
    verify(executionLayerEngineApiClient, times(1)).getPayload(eq(payloadId))
  }

  @Test
  fun `finishBlockBuilding can't be called before setHeadAndStartBlockBuilding`() {
    val result = executionLayerManager.finishBlockBuilding()
    assertThat(result.isCompletedExceptionally).isTrue()
  }

  @Test
  fun `importPayload forwards the call`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    val payloadStatus = PayloadStatusV1(TekuExecutionPayloadStatus.VALID, Bytes32.random(), null)
    mockNewPayloadWithStatus(payloadStatus)
    mockForkChoiceUpdateWithValidStatus(null)
    executionLayerManager.newPayload(executionPayload).get()

    verify(executionLayerEngineApiClient, times(1)).newPayload(eq(executionPayload))
  }

  @Test
  fun `importPayload throws exception on validation failure`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    val payloadStatus = PayloadStatusV1(TekuExecutionPayloadStatus.INVALID, Bytes32.random(), "Invalid payload")
    mockNewPayloadWithStatus(payloadStatus)

    assertThat(executionLayerManager.newPayload(executionPayload).get().validationError).isEqualTo("Invalid payload")
  }

  @Test
  fun `importPayload throws exception on general failure`() {
    val executionPayload = DataGenerators.randomExecutionPayload()
    mockFailedNewPayloadWithStatus("Unexpected error!")

    val exception =
      assertThrows<ExecutionException> {
        executionLayerManager.newPayload(executionPayload).get()
      }

    assertThat(exception).cause().hasMessage(
      "engine_newPayload request failed: " +
        "elBlockNumber=${executionPayload.blockNumber} fork=null Cause: Unexpected error!",
    )
  }

  @Test
  fun `setHead updates fork choice state and returns result`() {
    val newHeadHash = Bytes32.random()
    val newSafeHash = Bytes32.random()
    val newFinalizedHash = Bytes32.random()

    val payloadId = Bytes8(Bytes.random(8))
    val payloadStatus = mockForkChoiceUpdateWithValidStatus(payloadId)

    val result =
      executionLayerManager
        .setHead(
          headHash = newHeadHash.toArray(),
          safeHash = newSafeHash.toArray(),
          finalizedHash = newFinalizedHash.toArray(),
        ).get()

    val expectedResult =
      ForkChoiceUpdatedResult(payloadStatus.asInternalExecutionPayload().toDomain(), payloadId.wrappedBytes.toArray())
    assertThat(result).isEqualTo(expectedResult)

    verify(executionLayerEngineApiClient).forkChoiceUpdate(
      argThat { forkChoiceState ->
        forkChoiceState == ForkChoiceStateV1(newHeadHash, newSafeHash, newFinalizedHash)
      },
      isNull(),
    )
  }
}
