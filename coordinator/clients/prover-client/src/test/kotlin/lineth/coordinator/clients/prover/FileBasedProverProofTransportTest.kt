package lineth.coordinator.clients.prover

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.ProverFileNameProvider
import linea.domain.BlockIntervalProofIndex
import lineth.coordinator.clients.prover.RiscvProverClientTestFixtures.fileBasedProverConfig
import lineth.coordinator.clients.prover.RiscvProverClientTestFixtures.jsonMapper
import lineth.fileio.FileReader
import lineth.fileio.FileWriter
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class FileBasedProverProofTransportTest {
  private lateinit var config: FileBasedProverConfig
  private lateinit var transport:
    FileBasedProverProofTransport<ProofRequestDto, ProofResponseDto, BlockIntervalProofIndex>

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tempDir: Path) {
    config = fileBasedProverConfig(tempDir)
    transport = createTransport(vertx, enableRequestFilesCleanup = true)
  }

  @Test
  fun `submitRequest writes the request json file`() {
    val proofIndex = proofIndex(100u, 199u)
    val requestDto = ProofRequestDto(value = "request-100")

    transport.submitRequest(proofIndex, requestDto).get()

    val requestFile = config.requestsDirectory.resolve(requestFileNameProvider.getFileName(proofIndex)).toFile()
    assertThat(requestFile).exists()
    assertThat(jsonMapper.readValue(requestFile, ProofRequestDto::class.java)).isEqualTo(requestDto)
  }

  @Test
  fun `isRequestAlreadySubmitted returns true when the request file exists`() {
    val proofIndex = proofIndex(100u, 199u)
    transport.submitRequest(proofIndex, ProofRequestDto(value = "request-100")).get()

    assertThat(transport.isRequestAlreadySubmitted(proofIndex).get()).isTrue()
  }

  @Test
  fun `findResponse returns the response when the response file exists`() {
    val proofIndex = proofIndex(100u, 199u)
    val responseDto = ProofResponseDto(value = "response-100")
    saveResponseFile(proofIndex, responseDto)

    assertThat(transport.findResponse(proofIndex).get()).isEqualTo(responseDto)
  }

  @Test
  fun `isResponseAlreadyExisted returns true when the response file exists`() {
    val proofIndex = proofIndex(100u, 199u)
    saveResponseFile(proofIndex, ProofResponseDto(value = "response-100"))

    assertThat(transport.isResponseAlreadyExisted(proofIndex).get()).isTrue()
  }

  @Test
  fun `awaitResponse returns the response when the response file exists`() {
    val proofIndex = proofIndex(100u, 199u)
    val responseDto = ProofResponseDto(value = "response-100")
    saveResponseFile(proofIndex, responseDto)

    assertThat(transport.awaitResponse(proofIndex).get()).isEqualTo(responseDto)
  }

  @Test
  fun `removeRequests deletes request files at or above the threshold`() {
    val lower = proofIndex(100u, 199u)
    val higher = proofIndex(200u, 299u)
    transport.submitRequest(lower, ProofRequestDto(value = "request-100")).get()
    transport.submitRequest(higher, ProofRequestDto(value = "request-200")).get()

    transport.removeRequests(200).get()

    assertThat(config.requestsDirectory.resolve(requestFileNameProvider.getFileName(lower))).exists()
    assertThat(config.requestsDirectory.resolve(requestFileNameProvider.getFileName(higher))).doesNotExist()
  }

  private fun createTransport(
    vertx: Vertx,
    enableRequestFilesCleanup: Boolean,
  ): FileBasedProverProofTransport<ProofRequestDto, ProofResponseDto, BlockIntervalProofIndex> {
    return FileBasedProverProofTransport(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonMapper),
      fileReader = FileReader(vertx, jsonMapper, ProofResponseDto::class.java),
      requestFileNameProvider = requestFileNameProvider,
      responseFileNameProvider = responseFileNameProvider,
      enableRequestFilesCleanup = enableRequestFilesCleanup,
    )
  }

  private fun saveResponseFile(proofIndex: BlockIntervalProofIndex, responseDto: ProofResponseDto) {
    jsonMapper.writeValue(
      config.responsesDirectory.resolve(responseFileNameProvider.getFileName(proofIndex)).toFile(),
      responseDto,
    )
  }

  private fun proofIndex(startBlock: ULong, endBlock: ULong): BlockIntervalProofIndex = BlockIntervalProofIndex(
    startBlockNumber = startBlock,
    endBlockNumber = endBlock,
    startBlockTimestamp = Instant.fromEpochSeconds(0),
    hash = ByteArray(32) { 0x1a },
  )

  private data class ProofRequestDto(
    val value: String,
  )

  private data class ProofResponseDto(
    val value: String,
  )

  private val requestFileNameProvider = object : ProverFileNameProvider<BlockIntervalProofIndex> {
    override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
      return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-request.json"
    }
  }

  private val responseFileNameProvider = object : ProverFileNameProvider<BlockIntervalProofIndex> {
    override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
      return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-response.json"
    }
  }
}
