package net.consensys.zkevm.ethereum.coordination.proofcreation

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import com.fasterxml.jackson.databind.node.ArrayNode
import linea.contrat.events.createL2RollingHashUpdatedEthLogV1
import linea.contrat.events.createMessageSentEthLogV1
import linea.domain.createBlock
import linea.ethapi.FakeEthApiClient
import linea.kotlin.ByteArrayExt
import linea.kotlin.encodeHex
import linea.log4j.configureLoggers
import net.consensys.linea.traces.fakeTracesCountersV1
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofResponse
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import org.apache.logging.log4j.Level
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class ZkProofCreationCoordinatorImplTest {
  private lateinit var l2EthApiClient: FakeEthApiClient
  private lateinit var executionProverClient: ExecutionProverClientV2
  private lateinit var zkProofCreationCoordinator: ZkProofCreationCoordinator
  private val messageServiceAddress = Random.nextBytes(20).encodeHex()

  @BeforeEach
  fun beforeEach() {
    l2EthApiClient = FakeEthApiClient()
    executionProverClient = mock<ExecutionProverClientV2>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    zkProofCreationCoordinator = ZkProofCreationCoordinatorImpl(
      executionProverClient = executionProverClient,
      messageServiceAddress = messageServiceAddress,
      l2EthApiClient = l2EthApiClient
    )
  }

  @Test
  fun `should return batch with correct fields`() {
    val block0 = createBlock(number = 122UL, stateRoot = ByteArray(32) { 0x0 })
    val block1 = createBlock(number = 123UL, stateRoot = ByteArray(32) { 0x1 })
    val block2 = createBlock(number = 124UL, stateRoot = ByteArray(32) { 0x2 })
    val block1Logs = listOf(
      createMessageSentEthLogV1(blockNumber = 123UL, contractAddress = messageServiceAddress),
      createL2RollingHashUpdatedEthLogV1(blockNumber = 123UL, contractAddress = messageServiceAddress)
    )
    val block2Logs = listOf(
      createMessageSentEthLogV1(blockNumber = 124UL, contractAddress = messageServiceAddress),
      createL2RollingHashUpdatedEthLogV1(blockNumber = 124UL, contractAddress = messageServiceAddress)
    )

    l2EthApiClient.addBlocks(listOf(block0, block1, block2))
    l2EthApiClient.setLogs(listOf(block1Logs, block2Logs).flatten())
    l2EthApiClient.setLatestBlockTag(block2.number)

    configureLoggers(Level.DEBUG)

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

    verify(executionProverClient).requestProof(
      BatchExecutionProofRequestV1(
        blocks = listOf(block1, block2),
        tracesResponse = generateTracesResponse,
        bridgeLogs = listOf(block1Logs, block2Logs).flatten(),
        type2StateData = type2StateResponse,
        keccakParentStateRootHash = block0.stateRoot
      )
    )
  }
}
