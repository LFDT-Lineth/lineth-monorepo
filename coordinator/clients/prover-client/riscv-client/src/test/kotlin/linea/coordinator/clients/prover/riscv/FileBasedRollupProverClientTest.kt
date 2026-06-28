package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.BlobWitness
import linea.clients.RollupProofRequestV1
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.BlockIntervalProofIndex
import linea.fileio.FileReader
import linea.fileio.FileWriter
import linea.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Exercises [FileBasedRollupProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class FileBasedRollupProverClientTest {
  private val jsonMapper = JsonSerialization.proofResponseMapperV1
  private val guestProgramId = "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d"
  private val chainId = 59144L

  private lateinit var config: FileBasedProverConfig
  private lateinit var l2ExecutionProofTransport: L2ExecutionProofTransport
  private lateinit var client: FileBasedRollupProverClient

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
      FileBasedRollupProofRequestDto,
      RollupProofResponseDto,
      BlockIntervalProofIndex,
      >(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonMapper),
      fileReader = FileReader(vertx, jsonMapper, RollupProofResponseDto::class.java),
      requestFileNameProvider = RollupProofFileNameProvider,
      responseFileNameProvider = RollupProofFileNameProvider,
    )
    l2ExecutionProofTransport = FakeL2ExecutionProofTransport()
    client = FileBasedRollupProverClient(
      transport = transport,
      l2ExecutionProofTransport = l2ExecutionProofTransport,
      guestProgramId = guestProgramId,
      chainId = chainId,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = rollupRequest()

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(RollupProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), FileBasedRollupProofRequestDto::class.java)
    val expectedDto = FileBasedRollupProofRequestDtoMapper(
      guestProgramId,
      chainId,
      l2ExecutionProofTransport,
    ).invoke(request).get()
    assertThat(writtenDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the response file and maps it to the domain response`() {
    val proofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000520UL,
      hash = ByteArray(32) { 0x1a },
      startBlockTimestamp = Instant.fromEpochSeconds(1763000457),
    )
    val responseDto = rollupResponseDto()
    saveResponseFile(RollupProofFileNameProvider.getFileName(proofIndex), responseDto)

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(
      RollupProofResponseDtoMapper(responseDto),
    )
  }

  private fun saveResponseFile(fileName: String, responseDto: RollupProofResponseDto) {
    jsonMapper.writeValue(config.responsesDirectory.resolve(fileName).toFile(), responseDto)
  }

  private fun rollupRequest(): RollupProofRequestV1 = RollupProofRequestV1(
    blobs = listOf(
      BlobWitness(
        startBlockNumber = 1000501UL,
        endBlockNumber = 1000503UL,
        blobHash = Random.Default.nextBytes(32),
        blobKzgProof = Random.Default.nextBytes(48),
        blockRlps = listOf(
          Random.Default.nextBytes(16),
        ),
      ),
    ),
    parentShnarf = ByteArray(32) { 0x19 },
    endShnarf = ByteArray(32) { 0x20 },
    l2Executions = listOf(
      BlockIntervalProofIndex(
        startBlockNumber = 1000501UL,
        endBlockNumber = 1000503UL,
        hash = ByteArray(32) { 0x1e },
        startBlockTimestamp = Instant.fromEpochSeconds(1763000000),
      ),
    ),
  )

  private fun rollupResponseDto(): RollupProofResponseDto = RollupProofResponseDto(
    proverVersion = "4.0.0-riscv",
    proof = "0xabcd",
    startBlockNumber = 1000500,
    publicInputs = RollupProofPublicInputsDto(
      endBlockNumber = 1000520,
      endBlockTimestamp = 1763000457,
      l2L1BridgeTransactionTree = "0x10",
      parentL1L2BridgeRollingHash = "0x11",
      parentL1L2BridgeRollingHashMessageNumber = 12,
      endL1L2BridgeRollingHash = "0x13",
      endL1L2BridgeRollingHashMessageNumber = 14,
      dynamicChainConfigHash = "0xc0ffee",
      parentFtxRollingHash = "0x15",
      parentProcessedFtxNumber = 16,
      endFtxRollingHash = "0x16",
      endProcessedFtxNumber = 17,
      filteredAddressesHash = "0x18",
      parentShnarf = "0x19",
      endShnarf = "0x1a",
    ),
    l2L1Roots = listOf("0xaa"),
    filteredAddresses = emptyList(),
  )

  internal class FakeL2ExecutionProofTransport : L2ExecutionProofTransport {
    private val l2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
      startBlockNumber = 1000501L,
      proverVersion = "4.0.0-riscv",
      proof = Random.nextBytes(128).encodeHex(),
      publicInputs = L2ExecutionProofPublicInputsDto(
        parentBlockHash = "0x0a",
        endBlockHash = "0x0b",
        endBlockNumber = 1000503L,
        endBlockTimestamp = 1763000123L,
        l2L1MessagesHash = "0x01",
        parentL1L2BridgeRollingHash = "0x02",
        parentL1L2BridgeRollingHashMessageNumber = 3L,
        endL1L2BridgeRollingHash = "0x04",
        endL1L2BridgeRollingHashMessageNumber = 5L,
        dynamicChainConfigHash = "0xc0ffee",
        parentFtxRollingHash = "0x06",
        parentProcessedFtxNumber = 7L,
        endFtxRollingHash = "0x07",
        endProcessedFtxNumber = 8L,
        filteredAddressesHash = "0x09",
        txFromsHash = "0x0c",
      ),
      l2L1Messages = listOf(Random.nextBytes(32).encodeHex()),
      txFroms = listOf(Random.nextBytes(20).encodeHex()),
      filteredAddresses = listOf(Random.nextBytes(20).encodeHex()),
    )

    override fun isRequestAlreadySubmitted(proofIndex: BlockIntervalProofIndex): SafeFuture<Boolean> {
      return SafeFuture.completedFuture(true)
    }

    override fun submitRequest(
      proofIndex: BlockIntervalProofIndex,
      requestDto: L2ExecutionProofRequestDto,
    ): SafeFuture<Unit> {
      return SafeFuture.completedFuture(Unit)
    }

    override fun findResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<L2ExecutionProofResponseDto?> {
      return SafeFuture.completedFuture(
        l2ExecutionProofResponseDto,
      )
    }

    override fun awaitResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<L2ExecutionProofResponseDto> {
      return SafeFuture.completedFuture(
        l2ExecutionProofResponseDto,
      )
    }
  }
}
