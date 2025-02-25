package net.consensys.zkevm.ethereum.coordination.proofcreation

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import com.fasterxml.jackson.databind.node.ArrayNode
import linea.domain.createBlock
import linea.kotlin.ByteArrayExt
import linea.kotlin.encodeHex
import net.consensys.linea.traces.fakeTracesCountersV1
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofResponse
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import net.consensys.zkevm.coordinator.clients.L2MessageServiceLogsClient
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.CommonTestData
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.response.EthBlock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class ZkProofCreationCoordinatorImplTest {

  private lateinit var l2MessageServiceLogsClient: L2MessageServiceLogsClient
  private lateinit var l2Web3jClient: Web3j
  private lateinit var executionProverClient: ExecutionProverClientV2
  private lateinit var zkProofCreationCoordinator: ZkProofCreationCoordinator

  @BeforeEach
  fun beforeEach() {
    l2MessageServiceLogsClient = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    l2Web3jClient = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    executionProverClient = mock<ExecutionProverClientV2>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    zkProofCreationCoordinator = ZkProofCreationCoordinatorImpl(
      executionProverClient = executionProverClient,
      l2MessageServiceLogsClient = l2MessageServiceLogsClient,
      l2Web3jClient = l2Web3jClient
    )
  }

  @Test
  fun `should return batch with correct fields`() {
    val block1 = createBlock(number = 123UL)
    val block2 = createBlock(number = 124UL)
    val type2StateResponse = GetZkEVMStateMerkleProofResponse(
      zkStateMerkleProof = ArrayNode(null),
      zkParentStateRootHash = ByteArrayExt.random32(),
      zkEndStateRootHash = ByteArrayExt.random32(),
      zkStateManagerVersion = "2.0.0"
    )
    val generateTracesResponse = GenerateTracesResponse(
      tracesFileName = "123-124-conflated-traces.json",
      tracesEngineVersion = "1.0.0"
    )

    val stateRoot = Random.nextBytes(32).encodeHex()
    whenever(l2Web3jClient.ethGetBlockByNumber(any(), any()).sendAsync())
      .thenAnswer {
        val mockedEthBlock = mock<EthBlock>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS) {
          on { block.stateRoot } doReturn stateRoot
        }
        SafeFuture.completedFuture(mockedEthBlock)
      }

    whenever(l2MessageServiceLogsClient.getBridgeLogs(eq(block1.number.toLong())))
      .thenReturn(SafeFuture.completedFuture(listOf(CommonTestData.bridgeLogs[0])))
    whenever(l2MessageServiceLogsClient.getBridgeLogs(eq(block2.number.toLong())))
      .thenReturn(SafeFuture.completedFuture(listOf(CommonTestData.bridgeLogs[1])))

    whenever(executionProverClient.requestProof(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          BatchExecutionProofResponse(
            startBlockNumber = 123UL,
            endBlockNumber = 124UL
          )
        )
      )

    val batch = zkProofCreationCoordinator.createZkProof(
      blocksConflation = BlocksConflation(
        blocks = listOf(block1, block2),
        conflationResult = ConflationCalculationResult(
          startBlockNumber = 123UL,
          endBlockNumber = 124UL,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = fakeTracesCountersV1(0u)
        )
      ),
      traces = BlocksTracesConflated(
        tracesResponse = generateTracesResponse,
        zkStateTraces = type2StateResponse
      )
    ).get()

    assertThat(batch.startBlockNumber).isEqualTo(123UL)
    assertThat(batch.endBlockNumber).isEqualTo(124UL)
  }
}
