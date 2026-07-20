package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.COINBASE
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.L2_EXECUTION_GUEST_PROGRAM_ID
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.L2_MESSAGE_SERVICE_ADDRESS
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.fileBasedProverConfig
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.jsonMapper
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.l2ExecutionProofRequestV1
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.l2ExecutionProofResponseDto
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
 * Exercises [L2ExecutionProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class FileBasedL2ExecutionProverClientTest {
  private lateinit var config: FileBasedProverConfig
  private lateinit var client: L2ExecutionProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tempDir: Path) {
    config = fileBasedProverConfig(tempDir)
    val transport = FileBasedProverProofTransport<
      L2ExecutionProofRequestDto,
      L2ExecutionProofResponseDto,
      BlockIntervalProofIndex,
      >(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonMapper),
      fileReader = FileReader(vertx, jsonMapper, L2ExecutionProofResponseDto::class.java),
      requestFileNameProvider = L2ExecutionProofFileNameProvider,
      responseFileNameProvider = L2ExecutionProofFileNameProvider,
    )
    client = L2ExecutionProverClient(
      transport = transport,
      guestProgramId = L2_EXECUTION_GUEST_PROGRAM_ID,
      l2MessageServiceAddress = L2_MESSAGE_SERVICE_ADDRESS,
      coinbase = COINBASE,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = l2ExecutionProofRequestV1()

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(L2ExecutionProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), L2ExecutionProofRequestDto::class.java)
    val expectedDto = L2ExecutionProofRequestDtoMapper(
      L2_EXECUTION_GUEST_PROGRAM_ID,
      L2_MESSAGE_SERVICE_ADDRESS,
      COINBASE,
    ).invoke(request).get()
    assertThat(writtenDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the response file and maps it to the domain response`() {
    val proofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000503UL,
      hash = ByteArray(32) { 0x1e },
      startBlockTimestamp = Instant.fromEpochSeconds(1763000123),
    )
    val responseDto = l2ExecutionProofResponseDto(1000501L, 1000503L)
    val responseFile = saveResponseFile(L2ExecutionProofFileNameProvider.getFileName(proofIndex), responseDto)
    assertThat(responseFile).exists()

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(L2ExecutionProofResponseDtoMapper(responseDto))
  }

  private fun saveResponseFile(fileName: String, responseDto: L2ExecutionProofResponseDto): File {
    val responseFile = config.responsesDirectory.resolve(fileName).toFile()
    jsonMapper.writeValue(responseFile, responseDto)
    return responseFile
  }
}
