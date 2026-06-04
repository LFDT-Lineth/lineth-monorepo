package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.L2ExecutionProofRequestV1
import linea.coordinator.clients.prover.ExecutionProofFileNameProvider
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.ExecutionProofIndex
import linea.domain.createBlock
import linea.fileio.FileReader
import linea.fileio.FileWriter
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Exercises [L2ExecutionProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class L2ExecutionProverClientFileBasedTest {
  private val jsonMapper = JsonSerialization.proofResponseMapperV1
  private val proverVersion = "4.0.0-riscv"
  private val chainConfig = ChainConfigDto(
    l2MessageServiceContract = "0x508ca82df566dcd1b0019d2dedf7e3d6f7ad6dde",
    coinbase = "0x0000000000000000000000000000000000000000",
    chainId = 59144,
  )

  private lateinit var config: FileBasedProverConfig
  private lateinit var client: L2ExecutionProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tempDir: Path) {
    config = FileBasedProverConfig(
      requestsDirectory = tempDir.resolve("requests"),
      responsesDirectory = tempDir.resolve("responses"),
      inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
      inprogressRequestWritingSuffix = "coordinator_writing_inprogress",
      pollingInterval = 100.milliseconds,
      pollingTimeout = 2.seconds,
    )
    val transport = FileBasedProverProofTransport<
      L2ExecutionProofRequestDto,
      L2ExecutionProofResponseDto,
      ExecutionProofIndex,
      >(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonMapper),
      fileReader = FileReader(vertx, jsonMapper, L2ExecutionProofResponseDto::class.java),
      requestFileNameProvider = ExecutionProofFileNameProvider,
      responseFileNameProvider = ExecutionProofFileNameProvider,
    )
    client = L2ExecutionProverClient(
      transport = transport,
      proverVersion = proverVersion,
      chainConfig = chainConfig,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = l2Request()

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(ExecutionProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), L2ExecutionProofRequestDto::class.java)
    val expectedDto = L2ExecutionProofRequestDtoMapper(proverVersion, chainConfig).invoke(request).get()
    assertThat(writtenDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the response file and maps it to the domain response`() {
    val proofIndex = ExecutionProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000503UL,
      startBlockTimestamp = Instant.fromEpochSeconds(1763000123),
    )
    val responseDto = l2ResponseDto()
    saveResponseFile(ExecutionProofFileNameProvider.getFileName(proofIndex), responseDto)

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(
      L2ExecutionProofResponseDtoMapper(proofIndex, responseDto),
    )
  }

  private fun saveResponseFile(fileName: String, responseDto: L2ExecutionProofResponseDto) {
    jsonMapper.writeValue(config.responsesDirectory.resolve(fileName).toFile(), responseDto)
  }

  private fun l2Request(): L2ExecutionProofRequestV1 = L2ExecutionProofRequestV1(
    blocks = listOf(createBlock(number = 1000501UL), createBlock(number = 1000503UL)),
    forcedTransactions = emptyList(),
    l2L1MessagesHash = ByteArray(32) { 1 },
    parentL1L2BridgeRollingHash = ByteArray(32) { 2 },
    parentL1L2BridgeRollingHashMessageNumber = 0UL,
    endL1L2BridgeRollingHash = ByteArray(32) { 3 },
    endL1L2BridgeRollingHashMessageNumber = 0UL,
    dynamicChainConfigHash = ByteArray(32) { 4 },
    parentFtxRollingHash = ByteArray(32) { 5 },
    endFtxRollingHash = ByteArray(32) { 6 },
    lastProcessedFtxNumber = 0UL,
    filteredAddressesHash = ByteArray(32) { 7 },
    txFromsHash = ByteArray(32) { 8 },
  )

  private fun l2ResponseDto(): L2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
    proof = "0xabcd",
    publicInputs = L2ExecutionProofPublicInputsDto(
      parentBlockHash = "0x0a",
      endBlockHash = "0x0b",
      endBlockNumber = 1000503,
      endBlockTimestamp = 1763000123,
      L2L1MessagesHash = "0x01",
      parentL1L2BridgeRollingHash = "0x02",
      parentL1L2BridgeRollingHashMessageNumber = 3,
      endL1L2BridgeRollingHash = "0x04",
      endL1L2BridgeRollingHashMessageNumber = 5,
      dynamicChainConfigHash = "0xc0ffee",
      parentFtxRollingHash = "0x06",
      endFtxRollingHash = "0x07",
      lastProcessedFtxNumber = 8,
      filteredAddressesHash = "0x09",
      txFromsHash = "0x0c",
    ),
    L2L1MsgList = listOf("0xaa"),
    froms = listOf("0xbb"),
    addrs = emptyList(),
  )
}
