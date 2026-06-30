package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.CHAIN_ID
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.ROLLUP_GUEST_PROGRAM_ID
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.blobWitness
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.blockIntervalProofIndex
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.fileBasedProverConfig
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.jsonMapper
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.rollupProofRequestV1
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.rollupProofResponseDto
import linea.domain.BlockIntervalProofIndex
import linea.fileio.FileReader
import linea.fileio.FileWriter
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.time.Instant

/**
 * Exercises [FileBasedRollupProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class FileBasedRollupProverClientTest {
  private lateinit var config: FileBasedProverConfig
  private lateinit var l2ExecutionProofTransport: L2ExecutionProofTransport
  private lateinit var client: FileBasedRollupProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tempDir: Path) {
    config = fileBasedProverConfig(tempDir)
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
      guestProgramId = ROLLUP_GUEST_PROGRAM_ID,
      chainId = CHAIN_ID,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = rollupProofRequestV1(
      blobs = listOf(blobWitness(1000501UL, 1000503UL)),
      l2Executions = listOf(blockIntervalProofIndex(1000501UL, 1000503UL)),
    )

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(RollupProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), FileBasedRollupProofRequestDto::class.java)
    val expectedDto = FileBasedRollupProofRequestDtoMapper(
      ROLLUP_GUEST_PROGRAM_ID,
      CHAIN_ID,
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
    val responseDto = rollupProofResponseDto(1000501L, 1000520L)
    val responseFile = saveResponseFile(RollupProofFileNameProvider.getFileName(proofIndex), responseDto)
    assertThat(responseFile).exists()

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(RollupProofResponseDtoMapper(responseDto))
  }

  private fun saveResponseFile(fileName: String, responseDto: RollupProofResponseDto): File {
    val responseFile = config.responsesDirectory.resolve(fileName).toFile()
    jsonMapper.writeValue(config.responsesDirectory.resolve(fileName).toFile(), responseDto)
    return responseFile
  }
}
