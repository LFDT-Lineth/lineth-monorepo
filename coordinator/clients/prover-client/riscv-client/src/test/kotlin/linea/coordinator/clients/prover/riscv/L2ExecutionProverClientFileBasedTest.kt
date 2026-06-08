package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.ChainConfig
import linea.clients.ExecutionWitness
import linea.clients.L2ExecutionProofRequestV1
import linea.coordinator.clients.prover.ExecutionProofFileNameProvider
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.ExecutionProofIndex
import linea.fileio.FileReader
import linea.fileio.FileWriter
import maru.core.ExecutionPayload
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.math.BigInteger
import java.nio.file.Path
import kotlin.random.Random
import kotlin.random.nextULong
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
    println("RequestFile = ${requestFile.toAbsolutePath()}")

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
    executionPayloads = listOf(
      ExecutionPayload(
        parentHash = Random.nextBytes(32),
        feeRecipient = Random.nextBytes(20),
        stateRoot = Random.nextBytes(32),
        receiptsRoot = Random.nextBytes(32),
        logsBloom = Random.nextBytes(256),
        prevRandao = Random.nextBytes(32),
        blockNumber = 1000501UL,
        gasLimit = Random.nextULong(),
        gasUsed = Random.nextULong(),
        timestamp = 1000UL,
        extraData = Random.nextBytes(32),
        baseFeePerGas = BigInteger.valueOf(Random.nextLong(0, Long.MAX_VALUE)),
        blockHash = Random.nextBytes(32),
        transactions = emptyList(),
      ),
      ExecutionPayload(
        parentHash = Random.nextBytes(32),
        feeRecipient = Random.nextBytes(20),
        stateRoot = Random.nextBytes(32),
        receiptsRoot = Random.nextBytes(32),
        logsBloom = Random.nextBytes(256),
        prevRandao = Random.nextBytes(32),
        blockNumber = 1000502UL,
        gasLimit = Random.nextULong(),
        gasUsed = Random.nextULong(),
        timestamp = 1000UL,
        extraData = Random.nextBytes(32),
        baseFeePerGas = BigInteger.valueOf(Random.nextLong(0, Long.MAX_VALUE)),
        blockHash = Random.nextBytes(32),
        transactions = emptyList(),
      ),
    ),
    executionWitnesses = listOf(
      ExecutionWitness(
        blockNumber = 1000501UL,
        state = emptyList(),
        keys = emptyList(),
        codes = emptyList(),
        headers = emptyList(),
      ),
      ExecutionWitness(
        blockNumber = 1000502UL,
        state = emptyList(),
        keys = emptyList(),
        codes = emptyList(),
        headers = emptyList(),
      ),
    ),
    forcedTransactions = emptyList(),
    chainConfig = ChainConfig(
      l2MessageServiceContract = ByteArray(20) { 1 },
      coinbase = ByteArray(20) { 2 },
      chainId = 1000UL,
    ),
    parentFtxRollingHash = ByteArray(32) { 1 },
    parentLastProcessedFtxNumber = 100UL,
  )

  private fun l2ResponseDto(): L2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
    proof = "0xabcd",
    proverVersion = "4.0.0-riscv",
    startBlockNumber = 1000500,
    endBlockNumber = 1000503,
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
    L2L1Messages = listOf("0xaa"),
    txFroms = listOf("0xbb"),
    filteredAddresses = emptyList(),
  )
}
