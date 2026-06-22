/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.NextBlockTimestampProvider
import maru.consensus.blockimport.BlockBuildingBeaconBlockImporter
import maru.consensus.state.FinalizationState
import maru.core.BeaconState
import maru.core.ExecutionPayload
import maru.core.ext.DataGenerators
import maru.core.ext.DataGenerators.randomExecutionPayload
import maru.executionlayer.manager.ExecutionLayerManager
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.LatestBlockMetadata
import maru.executionlayer.manager.PayloadStatus
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import maru.executionlayer.manager.ext.DataGenerators as ExecutionLayerDataGenerators

class FollowerBeaconBlockImporterTest {
  private lateinit var executionLayerManagerDouble: FakeExecutionLayerManager
  private var nextBlockTimestamp = 123456789UL
  private var shouldBuildNextBlock: Boolean = false
  private lateinit var beaconBlockImporter: BlockBuildingBeaconBlockImporter
  private lateinit var finalizationState: FinalizationState
  private val prevRandaoProvider = { _: ULong, _: ByteArray -> Bytes32.random().toArray() }
  private val feeRecipient = Random.nextBytes(20)

  @BeforeEach
  fun setUp() {
    executionLayerManagerDouble = FakeExecutionLayerManager()
    finalizationState = FinalizationState(Random.nextBytes(32), Random.nextBytes(32))

    beaconBlockImporter = BlockBuildingBeaconBlockImporter(
      executionLayerManager = executionLayerManagerDouble,
      finalizationStateProvider = { finalizationState },
      nextBlockTimestampProvider = { nextBlockTimestamp },
      prevRandaoProvider = prevRandaoProvider,
      shouldBuildNextBlock = { _: BeaconState, _: ConsensusRoundIdentifier, _: ULong ->
        shouldBuildNextBlock
      },
      feeRecipient = feeRecipient,
    )
  }

  @Test
  fun `importBlock should call setHeadAndStartBlockBuilding when shouldBuildNextBlock returns true`() {
    shouldBuildNextBlock = true
    val randomBeaconBlock = DataGenerators.randomBeaconBlock(
      1UL,
    )
    val randomBeaconState = DataGenerators.randomBeaconState(
      1UL,
    )

    val result = beaconBlockImporter.importBlock(randomBeaconState, randomBeaconBlock)

    assertEquals(executionLayerManagerDouble.expectedResponse, result)

    val call = executionLayerManagerDouble.setHeadAndStartBlockBuildingCalls.single()
    assertEquals(randomBeaconBlock.beaconBlockBody.executionPayload.blockHash, call.headHash)
    assertEquals(finalizationState.safeBlockHash, call.safeHash)
    assertEquals(finalizationState.finalizedBlockHash, call.finalizedHash)
    assertEquals(nextBlockTimestamp, call.nextBlockTimestamp)
    assertEquals(feeRecipient.contentToString(), call.feeRecipient.contentToString())

    assertTrue(executionLayerManagerDouble.setHeadCalls.isEmpty())
  }

  @Test
  fun `importBlock should call setHead when shouldBuildNextBlock returns false`() {
    shouldBuildNextBlock = false
    val randomBeaconBlock = DataGenerators.randomBeaconBlock(
      1UL,
    )
    val randomBeaconState = DataGenerators.randomBeaconState(
      1UL,
    )

    val result = beaconBlockImporter.importBlock(randomBeaconState, randomBeaconBlock)

    assertEquals(executionLayerManagerDouble.expectedResponse, result)

    val call = executionLayerManagerDouble.setHeadCalls.single()
    assertEquals(randomBeaconBlock.beaconBlockBody.executionPayload.blockHash, call.headHash)
    assertEquals(finalizationState.safeBlockHash, call.safeHash)
    assertEquals(finalizationState.finalizedBlockHash, call.finalizedHash)

    assertTrue(executionLayerManagerDouble.setHeadAndStartBlockBuildingCalls.isEmpty())
  }

  @Test
  fun `importBlock should pass last block timestamp and next block's round identifier`() {
    val randomBeaconBlock = DataGenerators.randomBeaconBlock(
      1UL,
    )
    val randomBeaconState = DataGenerators.randomBeaconState(
      1UL,
    )
    val expectedConsensusRoundIdentifier = ConsensusRoundIdentifier(2, 0)

    val shouldBuildNextBlockDouble = FakeShouldBuildNextBlockPredicate()
    val nextBlockTimestampProviderDouble = FakeNextBlockTimestampProvider(nextBlockTimestamp)
    val expectedParentTimestamp = randomBeaconState.beaconBlockHeader.timestamp

    beaconBlockImporter = BlockBuildingBeaconBlockImporter(
      executionLayerManager = executionLayerManagerDouble,
      finalizationStateProvider = { finalizationState },
      nextBlockTimestampProvider = nextBlockTimestampProviderDouble,
      prevRandaoProvider = prevRandaoProvider,
      shouldBuildNextBlock = shouldBuildNextBlockDouble,
      feeRecipient = feeRecipient,
    )

    beaconBlockImporter.importBlock(randomBeaconState, randomBeaconBlock)

    // Verify shouldBuildNextBlock was called with correct parameters
    val shouldBuildCall = shouldBuildNextBlockDouble.calls.single()
    assertEquals(randomBeaconState, shouldBuildCall.beaconState)
    assertEquals(expectedConsensusRoundIdentifier, shouldBuildCall.roundIdentifier)
    assertEquals(nextBlockTimestamp, shouldBuildCall.timestamp)

    // Verify nextBlockTimestampProvider was called with correct parameter
    val timestampCall = nextBlockTimestampProviderDouble.calls.single()
    assertEquals(expectedParentTimestamp, timestampCall)
  }

  data class SetHeadAndStartBlockBuildingCall(
    val headHash: ByteArray,
    val safeHash: ByteArray,
    val finalizedHash: ByteArray,
    val nextBlockTimestamp: ULong,
    val feeRecipient: ByteArray,
    val prevRandao: ByteArray,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as SetHeadAndStartBlockBuildingCall

      if (!headHash.contentEquals(other.headHash)) return false
      if (!safeHash.contentEquals(other.safeHash)) return false
      if (!finalizedHash.contentEquals(other.finalizedHash)) return false
      if (nextBlockTimestamp != other.nextBlockTimestamp) return false
      if (!feeRecipient.contentEquals(other.feeRecipient)) return false
      if (!prevRandao.contentEquals(other.prevRandao)) return false

      return true
    }

    override fun hashCode(): Int {
      var result = headHash.contentHashCode()
      result = 31 * result + safeHash.contentHashCode()
      result = 31 * result + finalizedHash.contentHashCode()
      result = 31 * result + nextBlockTimestamp.hashCode()
      result = 31 * result + feeRecipient.contentHashCode()
      result = 31 * result + prevRandao.contentHashCode()
      return result
    }
  }

  data class SetHeadCall(
    val headHash: ByteArray,
    val safeHash: ByteArray,
    val finalizedHash: ByteArray,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as SetHeadCall

      if (!headHash.contentEquals(other.headHash)) return false
      if (!safeHash.contentEquals(other.safeHash)) return false
      if (!finalizedHash.contentEquals(other.finalizedHash)) return false

      return true
    }

    override fun hashCode(): Int {
      var result = headHash.contentHashCode()
      result = 31 * result + safeHash.contentHashCode()
      result = 31 * result + finalizedHash.contentHashCode()
      return result
    }
  }

  class FakeExecutionLayerManager : ExecutionLayerManager {
    val expectedResponse: SafeFuture<ForkChoiceUpdatedResult> =
      SafeFuture.completedFuture(
        ExecutionLayerDataGenerators.randomValidForkChoiceUpdatedResult(),
      )
    val setHeadAndStartBlockBuildingCalls = mutableListOf<SetHeadAndStartBlockBuildingCall>()
    val setHeadCalls = mutableListOf<SetHeadCall>()

    override fun setHeadAndStartBlockBuilding(
      headHash: ByteArray,
      safeHash: ByteArray,
      finalizedHash: ByteArray,
      nextBlockTimestamp: ULong,
      feeRecipient: ByteArray,
      prevRandao: ByteArray,
    ): SafeFuture<ForkChoiceUpdatedResult> {
      setHeadAndStartBlockBuildingCalls.add(
        SetHeadAndStartBlockBuildingCall(
          headHash,
          safeHash,
          finalizedHash,
          nextBlockTimestamp,
          feeRecipient,
          prevRandao,
        ),
      )
      return expectedResponse
    }

    override fun setHead(
      headHash: ByteArray,
      safeHash: ByteArray,
      finalizedHash: ByteArray,
    ): SafeFuture<ForkChoiceUpdatedResult> {
      setHeadCalls.add(SetHeadCall(headHash, safeHash, finalizedHash))
      return expectedResponse
    }

    // Implement required abstract methods that aren't used in tests
    override fun finishBlockBuilding(): SafeFuture<ExecutionPayload> =
      SafeFuture.completedFuture(randomExecutionPayload())

    override fun getLatestBlockMetadata(): SafeFuture<LatestBlockMetadata> =
      SafeFuture.completedFuture(LatestBlockMetadata(blockHash = Random.nextBytes(32), timestamp = 0UL))

    override fun isOnline(): SafeFuture<Boolean> = SafeFuture.completedFuture(true)

    override fun newPayload(executionPayload: ExecutionPayload): SafeFuture<PayloadStatus> =
      SafeFuture.completedFuture(ExecutionLayerDataGenerators.randomValidPayloadStatus())
  }

  data class ShouldBuildNextBlockCall(
    val beaconState: BeaconState,
    val roundIdentifier: ConsensusRoundIdentifier,
    val timestamp: ULong,
  )

  class FakeShouldBuildNextBlockPredicate : (BeaconState, ConsensusRoundIdentifier, ULong) -> Boolean {
    val calls = mutableListOf<ShouldBuildNextBlockCall>()
    var returnValue = true

    override fun invoke(
      beaconState: BeaconState,
      roundIdentifier: ConsensusRoundIdentifier,
      timestamp: ULong,
    ): Boolean {
      calls.add(ShouldBuildNextBlockCall(beaconState, roundIdentifier, timestamp))
      return returnValue
    }
  }

  class FakeNextBlockTimestampProvider(
    private val returnValue: ULong,
  ) : NextBlockTimestampProvider {
    val calls = mutableListOf<ULong>()

    override fun nextTargetBlockUnixTimestamp(lastBlockTimestamp: ULong): ULong {
      calls.add(lastBlockTimestamp)
      return returnValue
    }
  }
}
