package net.consensys.zkevm.coordinator.clients.prover

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import com.fasterxml.jackson.databind.node.ArrayNode
import linea.domain.Block
import linea.domain.createBlock
import net.consensys.ByteArrayExt
import net.consensys.encodeHex
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import net.consensys.zkevm.coordinator.clients.L2MessageServiceLogsClient
import net.consensys.zkevm.domain.RlpBridgeLogsData
import net.consensys.zkevm.encoding.BlockEncoder
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.spy
import org.mockito.kotlin.whenever
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.response.EthBlock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class ExecutionProofRequestDataDecoratorTest {

  private lateinit var l2MessageServiceLogsClient: L2MessageServiceLogsClient
  private lateinit var l2Web3jClient: Web3j
  private lateinit var encoder: BlockEncoder
  private lateinit var requestDatDecorator: ExecutionProofRequestDataDecorator
  private val fakeEncoder: BlockEncoder = object : BlockEncoder {
    override fun encode(block: Block): ByteArray {
      return block.number.toString().toByteArray()
    }
  }

  @BeforeEach
  fun beforeEach() {
    l2MessageServiceLogsClient = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    l2Web3jClient = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    encoder = spy(fakeEncoder)
    requestDatDecorator = ExecutionProofRequestDataDecorator(l2MessageServiceLogsClient, l2Web3jClient, encoder)
  }

  @Test
  fun `should decorate data with bridge logs and parent stateRootHash`() {
    val block1 = createBlock(number = 123UL)
    val block2 = createBlock(number = 124UL)
    val type2StateResponse = GetZkEVMStateMerkleProofResponse(
      zkStateMerkleProof = ArrayNode(null),
      zkParentStateRootHash = ByteArrayExt.random32(),
      zkEndStateRootHash = ByteArrayExt.random32(),
      zkStateManagerVersion = "2.0.0"
    )
    val generateTracesResponse = GenerateTracesResponse(
      tracesFileName = "123-114-conflated-traces.json",
      tracesEngineVersion = "1.0.0"
    )
    val request = BatchExecutionProofRequestV1(
      blocks = listOf(block1, block2),
      tracesResponse = generateTracesResponse,
      type2StateData = type2StateResponse
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

    val requestDto = requestDatDecorator.invoke(request).get()

    assertThat(requestDto.keccakParentStateRootHash).isEqualTo(stateRoot)
    assertThat(requestDto.zkParentStateRootHash).isEqualTo(type2StateResponse.zkParentStateRootHash.encodeHex())
    assertThat(requestDto.conflatedExecutionTracesFile).isEqualTo("123-114-conflated-traces.json")
    assertThat(requestDto.tracesEngineVersion).isEqualTo("1.0.0")
    assertThat(requestDto.type2StateManagerVersion).isEqualTo("2.0.0")
    assertThat(requestDto.zkStateMerkleProof).isEqualTo(type2StateResponse.zkStateMerkleProof)
    assertThat(requestDto.blocksData).hasSize(2)
    assertThat(requestDto.blocksData[0]).isEqualTo(
      RlpBridgeLogsData(
        rlp = "123".toByteArray().encodeHex(),
        bridgeLogs = listOf(CommonTestData.bridgeLogs[0])
      )
    )
    assertThat(requestDto.blocksData[1]).isEqualTo(
      RlpBridgeLogsData(
        rlp = "124".toByteArray().encodeHex(),
        bridgeLogs = listOf(CommonTestData.bridgeLogs[1])
      )
    )
  }
}
