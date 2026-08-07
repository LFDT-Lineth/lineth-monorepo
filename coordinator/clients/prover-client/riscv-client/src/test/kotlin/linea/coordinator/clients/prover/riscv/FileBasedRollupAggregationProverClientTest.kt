package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.ROLLUP_AGGREGATION_GUEST_PROGRAM_ID
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.fileBasedProverConfig
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.jsonMapper
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.rollupAggregationProofRequestV1
import linea.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.rollupAggregationProofResponseDto
import linea.fileio.FileReader
import linea.fileio.FileWriter
import lineth.domain.BlockIntervalProofIndex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.time.Instant

/**
 * Exercises [FileBasedRollupAggregationProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class FileBasedRollupAggregationProverClientTest {
  private lateinit var config: FileBasedProverConfig
  private lateinit var rollupProofTransport: FileBasedRollupProofTransport
  private lateinit var client: FileBasedRollupAggregationProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tempDir: Path) {
    config = fileBasedProverConfig(tempDir)
    val transport = FileBasedProverProofTransport<
      FileBasedRollupAggregationProofRequestDto,
      RollupAggregationProofResponseDto,
      BlockIntervalProofIndex,
      >(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonMapper),
      fileReader = FileReader(vertx, jsonMapper, RollupAggregationProofResponseDto::class.java),
      requestFileNameProvider = RollupAggregationProofFileNameProvider,
      responseFileNameProvider = RollupAggregationProofFileNameProvider,
    )
    rollupProofTransport = FakeRollupProofTransport()
    client = FileBasedRollupAggregationProverClient(
      transport = transport,
      rollupProofTransport = rollupProofTransport,
      guestProgramId = ROLLUP_AGGREGATION_GUEST_PROGRAM_ID,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = rollupAggregationProofRequestV1()

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(RollupAggregationProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), FileBasedRollupAggregationProofRequestDto::class.java)
    val expectedDto = FileBasedRollupAggregationProofRequestDtoMapper(
      ROLLUP_AGGREGATION_GUEST_PROGRAM_ID,
      rollupProofTransport,
    ).invoke(request).get()
    assertThat(writtenDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the response file and maps it to the domain response`() {
    val proofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000567UL,
      hash = ByteArray(32) { 0x1a },
      startBlockTimestamp = Instant.fromEpochSeconds(1763000000),
    )
    val responseDto = rollupAggregationProofResponseDto(1000501L, 1000567L)
    val responseFile = saveResponseFile(RollupAggregationProofFileNameProvider.getFileName(proofIndex), responseDto)
    assertThat(responseFile).exists()

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(RollupAggregationProofResponseDtoMapper(responseDto))
  }

  private fun saveResponseFile(fileName: String, responseDto: RollupAggregationProofResponseDto): File {
    val responseFile = config.responsesDirectory.resolve(fileName).toFile()
    jsonMapper.writeValue(config.responsesDirectory.resolve(fileName).toFile(), responseDto)
    return responseFile
  }
}
