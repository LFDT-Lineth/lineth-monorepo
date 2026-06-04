package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.RollupAggregationProofRequestV1
import linea.clients.RollupProofPublicInputs
import linea.coordinator.clients.prover.AggregationProofFileNameProvider
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.crypto.Sha256HashFunction
import linea.domain.AggregationProofIndex
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
 * Exercises [RollupAggregationProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class RollupAggregationProverClientFileBasedTest {
  private val jsonMapper = JsonSerialization.proofResponseMapperV1
  private val proverVersion = "4.0.0-riscv"
  private val proofIndexProvider =
    RollupAggregationProverClient.createProofIndexProviderFn(Sha256HashFunction())

  private lateinit var config: FileBasedProverConfig
  private lateinit var client: RollupAggregationProverClient

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
      RollupAggregationProofRequestDto,
      RollupAggregationProofResponseDto,
      AggregationProofIndex,
      >(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonMapper),
      fileReader = FileReader(vertx, jsonMapper, RollupAggregationProofResponseDto::class.java),
      requestFileNameProvider = AggregationProofFileNameProvider,
      responseFileNameProvider = AggregationProofFileNameProvider,
    )
    client = RollupAggregationProverClient(
      transport = transport,
      proverVersion = proverVersion,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = aggregationRequest()

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(AggregationProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), RollupAggregationProofRequestDto::class.java)
    val expectedDto = RollupAggregationProofRequestDtoMapper(proverVersion).invoke(request).get()
    assertThat(writtenDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the response file and maps it to the domain response`() {
    val proofIndex = proofIndexProvider(aggregationRequest())
    val responseDto = aggregationResponseDto()
    saveResponseFile(AggregationProofFileNameProvider.getFileName(proofIndex), responseDto)

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(
      RollupAggregationProofResponseDtoMapper(proofIndex, responseDto),
    )
  }

  private fun saveResponseFile(fileName: String, responseDto: RollupAggregationProofResponseDto) {
    jsonMapper.writeValue(config.responsesDirectory.resolve(fileName).toFile(), responseDto)
  }

  private fun aggregationRequest(): RollupAggregationProofRequestV1 = RollupAggregationProofRequestV1(
    startBlockNumber = 1000501UL,
    startBlockTimestamp = Instant.fromEpochSeconds(1763000000),
    publicInputs = rollupPublicInputs(),
    rollupProofs = emptyList(),
  )

  private fun rollupPublicInputs(): RollupProofPublicInputs = RollupProofPublicInputs(
    endBlockNumber = 1000567UL,
    endBlockTimestamp = Instant.fromEpochSeconds(1763002301),
    L2L1BridgeTransactionTree = ByteArray(32) { 0x10 },
    parentL1L2BridgeRollingHash = ByteArray(32) { 0x11 },
    parentL1L2BridgeRollingHashMessageNumber = 12UL,
    endL1L2BridgeRollingHash = ByteArray(32) { 0x13 },
    endL1L2BridgeRollingHashMessageNumber = 14UL,
    dynamicChainConfigHash = ByteArray(32) { 0x0c },
    parentFtxRollingHash = ByteArray(32) { 0x15 },
    endFtxRollingHash = ByteArray(32) { 0x16 },
    lastProcessedFtxNumber = 17UL,
    filteredAddressesHash = ByteArray(32) { 0x18 },
    parentShnarf = ByteArray(32) { 0x19 },
    endShnarf = ByteArray(32) { 0x1a },
  )

  private fun aggregationResponseDto(): RollupAggregationProofResponseDto = RollupAggregationProofResponseDto(
    proof = "0xabcd",
    publicInputs = RollupProofPublicInputsDto(
      endBlockNumber = 1000567,
      endBlockTimestamp = 1763002301,
      L2L1BridgeTransactionTree = "0x10",
      parentL1L2BridgeRollingHash = "0x11",
      parentL1L2BridgeRollingHashMessageNumber = 12,
      endL1L2BridgeRollingHash = "0x13",
      endL1L2BridgeRollingHashMessageNumber = 14,
      dynamicChainConfigHash = "0xc0ffee",
      parentFtxRollingHash = "0x15",
      endFtxRollingHash = "0x16",
      lastProcessedFtxNumber = 17,
      filteredAddressesHash = "0x18",
      parentShnarf = "0x19",
      endShnarf = "0x1a",
    ),
  )
}
