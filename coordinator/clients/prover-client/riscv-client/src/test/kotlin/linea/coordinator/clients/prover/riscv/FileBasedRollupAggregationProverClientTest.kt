package linea.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.RollupAggregationProofRequestV1
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
 * Exercises [FileBasedRollupAggregationProverClient] end-to-end over the [FileBasedProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> JSON file;
 *  - reading a response: JSON file -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class FileBasedRollupAggregationProverClientTest {
  private val jsonMapper = JsonSerialization.proofResponseMapperV1
  private val guestProgramId = "0x8a5fdb137ddae03b9bad034500c0fcee76e1c61d70faca5f32bb7418d73392e1"

  private lateinit var config: FileBasedProverConfig
  private lateinit var rollupProofTransport: FileBasedRollupProofTransport
  private lateinit var client: FileBasedRollupAggregationProverClient

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
      guestProgramId = guestProgramId,
    )
  }

  @Test
  fun `createProofRequest writes the request DTO to a json file`() {
    val request = aggregationRequest()

    val proofIndex = client.createProofRequest(request).get()

    val requestFile = config.requestsDirectory.resolve(RollupAggregationProofFileNameProvider.getFileName(proofIndex))
    assertThat(requestFile).exists()

    val writtenDto = jsonMapper.readValue(requestFile.toFile(), FileBasedRollupAggregationProofRequestDto::class.java)
    val expectedDto = FileBasedRollupAggregationProofRequestDtoMapper(
      guestProgramId,
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
    val responseDto = aggregationResponseDto()
    saveResponseFile(RollupAggregationProofFileNameProvider.getFileName(proofIndex), responseDto)

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(RollupAggregationProofResponseDtoMapper(responseDto))
  }

  private fun saveResponseFile(fileName: String, responseDto: RollupAggregationProofResponseDto) {
    jsonMapper.writeValue(config.responsesDirectory.resolve(fileName).toFile(), responseDto)
  }

  private fun aggregationRequest(): RollupAggregationProofRequestV1 = RollupAggregationProofRequestV1(
    rollupProofs = listOf(
      BlockIntervalProofIndex(
        startBlockNumber = 1000501UL,
        endBlockNumber = 1000567UL,
        hash = ByteArray(32) { 0x1a },
        startBlockTimestamp = Instant.fromEpochSeconds(1763000000),
      ),
    ),
  )

  private fun aggregationResponseDto(): RollupAggregationProofResponseDto = RollupAggregationProofResponseDto(
    proverVersion = "4.0.0-riscv",
    startBlockNumber = 1000501,
    proof = "0xabcd",
    publicInputs = RollupProofPublicInputsDto(
      endBlockNumber = 1000567,
      endBlockTimestamp = 1763002301,
      l2L1BridgeTransactionTree = "0x10",
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
    l2L1Roots = listOf(Random.nextBytes(32).encodeHex()),
    filteredAddresses = listOf(Random.nextBytes(20).encodeHex()),
    l2MessagingBlocksOffsets = listOf(1, 20, 100),
  )

  internal class FakeRollupProofTransport : FileBasedRollupProofTransport {
    private val rollupProofResponseDto = RollupProofResponseDto(
      startBlockNumber = 1000501L,
      proverVersion = "4.0.0-riscv",
      proof = Random.nextBytes(128).encodeHex(),
      publicInputs = RollupProofPublicInputsDto(
        endBlockNumber = 1000503L,
        endBlockTimestamp = 1763000123L,
        l2L1BridgeTransactionTree = "0x01",
        parentL1L2BridgeRollingHash = "0x02",
        parentL1L2BridgeRollingHashMessageNumber = 3L,
        endL1L2BridgeRollingHash = "0x04",
        endL1L2BridgeRollingHashMessageNumber = 5L,
        dynamicChainConfigHash = "0xc0ffee",
        parentFtxRollingHash = "0x06",
        endFtxRollingHash = "0x07",
        lastProcessedFtxNumber = 8L,
        filteredAddressesHash = "0x09",
        parentShnarf = "0x0c",
        endShnarf = "0x0d",
      ),
      l2L1Roots = listOf(Random.nextBytes(32).encodeHex()),
      filteredAddresses = listOf(Random.nextBytes(20).encodeHex()),
    )

    override fun isRequestAlreadySubmitted(proofIndex: BlockIntervalProofIndex): SafeFuture<Boolean> {
      return SafeFuture.completedFuture(true)
    }

    override fun submitRequest(
      proofIndex: BlockIntervalProofIndex,
      requestDto: FileBasedRollupProofRequestDto,
    ): SafeFuture<Unit> {
      return SafeFuture.completedFuture(Unit)
    }

    override fun findResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<RollupProofResponseDto?> {
      return SafeFuture.completedFuture(
        rollupProofResponseDto,
      )
    }

    override fun awaitResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<RollupProofResponseDto> {
      return SafeFuture.completedFuture(
        rollupProofResponseDto,
      )
    }
  }
}
