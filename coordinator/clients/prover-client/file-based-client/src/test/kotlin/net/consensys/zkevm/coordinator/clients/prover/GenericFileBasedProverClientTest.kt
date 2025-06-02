package net.consensys.zkevm.coordinator.clients.prover

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockInterval
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Files
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class GenericFileBasedProverClientTest {
  data class ProofRequest(override val startBlockNumber: ULong, override val endBlockNumber: ULong) : BlockInterval
  data class ProofResponse(val startBlockNumber: ULong, val endBlockNumber: ULong)
  data class ProofRequestDto(val blockNumberStart: ULong, val blockNumberEnd: ULong) {
    companion object {
      fun fromDomain(request: ProofRequest): ProofRequestDto {
        return ProofRequestDto(request.startBlockNumber, request.endBlockNumber)
      }
    }
  }

  // Repeated Dto class for illustration purposes
  data class ProofResponseDto(val blockNumberStart: ULong, val blockNumberEnd: ULong) {
    companion object {
      fun toDomain(request: ProofResponseDto): ProofResponse {
        return ProofResponse(request.blockNumberStart, request.blockNumberEnd)
      }
    }
  }

  private val requestFileNameProvider = ProverFileNameProvider("proof-request.json")
  private val responseFileNameProvider = ProverFileNameProvider("proof-response.json")

  private lateinit var proverClient: GenericFileBasedProverClient<
    ProofRequest,
    ProofResponse,
    ProofRequestDto,
    ProofResponseDto,
    >
  private lateinit var proverConfig: FileBasedProverConfig

  private fun createProverClient(
    config: FileBasedProverConfig,
    vertx: Vertx,
  ): GenericFileBasedProverClient<
    ProofRequest,
    ProofResponse,
    ProofRequestDto,
    ProofResponseDto,
    > {
    return GenericFileBasedProverClient(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, JsonSerialization.proofResponseMapperV1),
      fileReader = FileReader(
        vertx,
        JsonSerialization.proofResponseMapperV1,
        ProofResponseDto::class.java,
      ),
      requestFileNameProvider = requestFileNameProvider,
      responseFileNameProvider = responseFileNameProvider,
      requestMapper = { SafeFuture.completedFuture(ProofRequestDto.fromDomain(it)) },
      proofTypeLabel = "batch",
      responseMapper = ProofResponseDto::toDomain,
    )
  }

  @BeforeEach
  fun beforeEach(
    vertx: Vertx,
    @TempDir tempDir: Path,
  ) {
    proverConfig = FileBasedProverConfig(
      requestsDirectory = tempDir.resolve("requests"),
      responsesDirectory = tempDir.resolve("responses"),
      inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
      inprogressRequestWritingSuffix = "coordinator_writing_inprogress",
      pollingInterval = 100.milliseconds,
      pollingTimeout = 2.seconds,
    )

    proverClient = createProverClient(proverConfig, vertx)
  }

  private fun responseFilePath(proofIndex: ProofIndex): Path {
    return proverConfig.responsesDirectory.resolve(responseFileNameProvider.getFileName(proofIndex))
  }

  private fun requestFilePath(proofIndex: ProofIndex): Path {
    return proverConfig.requestsDirectory.resolve(requestFileNameProvider.getFileName(proofIndex))
  }

  private fun saveToFile(file: Path, content: Any) {
    val writeInProgessFile = file.resolveSibling(file.fileName.toString() + ".coordinator_writing_inprogress")
    JsonSerialization.proofResponseMapperV1.writeValue(writeInProgessFile.toFile(), content)
    Files.move(writeInProgessFile, file)
  }

  private fun <T> readFromFile(file: Path, valueType: Class<T>): T {
    return JsonSerialization.proofResponseMapperV1.readValue(file.toFile(), valueType)
  }

  @Test
  fun `when it cannot create request and response directories shall fail`(
    vertx: Vertx,
  ) {
    val dirWithoutWritePermissions = Path.of("/invalid/path")
    val invalidConfig = proverConfig.copy(
      requestsDirectory = dirWithoutWritePermissions.resolve("requests"),
      responsesDirectory = dirWithoutWritePermissions.resolve("responses"),
    )
    assertThrows<Exception> {
      createProverClient(invalidConfig, vertx)
    }
  }

  @Test
  fun `when request does not exist shall write it and wait for response`() {
    val proofIndex = ProofIndex(startBlockNumber = 1u, endBlockNumber = 20u)
    val responseFuture = proverClient.requestProof(proofIndex.toRequest())
    // assert it will write the request file
    val expectedRequestFile = requestFilePath(proofIndex)
    val expectedResponseFile = responseFilePath(proofIndex)

    // assert it will wait for the response file
    assertThat(responseFuture.isDone).isFalse()
    assertThat(responseFuture.isCancelled).isFalse()
    assertThat(responseFuture.isCompletedExceptionally).isFalse()
    assertThat(responseFuture.isCompletedNormally).isFalse()
    assertRequestWaitingCountIs(1)

    // write response
    saveToFile(expectedResponseFile, ProofResponseDto(blockNumberStart = 1u, blockNumberEnd = 20u))

    val response = responseFuture.get()
    assertThat(expectedRequestFile).exists()
    assertThat(response).isEqualTo(ProofResponse(startBlockNumber = 1u, endBlockNumber = 20u))
    assertThat(proverClient.get()).isEqualTo(0)
  }

  @Test
  fun `when response already exists, should reuse it`() {
    val proofIndex = ProofIndex(startBlockNumber = 2u, endBlockNumber = 22u)
    // write response
    saveToFile(responseFilePath(proofIndex), ProofResponseDto(blockNumberStart = 2u, blockNumberEnd = 22u))

    val response = proverClient.requestProof(proofIndex.toRequest()).get()
    assertThat(proverClient.get()).isEqualTo(0)
    assertThat(response).isEqualTo(ProofResponse(startBlockNumber = 2u, endBlockNumber = 22u))
    assertThat(requestFilePath(proofIndex)).doesNotExist()
  }

  @Test
  fun `when request already exists shall skip writing it and wait for response`() {
    // this is to prevent when coordinator is restarted
    // and the request is already in the requests directory and create a duplicated one

    val proofIndex = ProofIndex(startBlockNumber = 3u, endBlockNumber = 33u)

    // write request
    // Write with a different block number content to check that it was not overwritten
    saveToFile(requestFilePath(proofIndex), ProofRequestDto(blockNumberStart = 3u, blockNumberEnd = 33333u))
    val responseFuture = proverClient.requestProof(proofIndex.toRequest())
    assertRequestWaitingCountIs(1)

    // write response
    saveToFile(responseFilePath(proofIndex), ProofResponseDto(blockNumberStart = 3u, blockNumberEnd = 33u))

    responseFuture.get()

    val proofRequest = readFromFile(requestFilePath(proofIndex), ProofRequestDto::class.java)

    // assert original request was not overwritten
    assertThat(proofRequest).isEqualTo(ProofRequestDto(blockNumberStart = 3u, blockNumberEnd = 33333u))
    assertThat(proverClient.get()).isEqualTo(0)
  }

  @Test
  fun `when request is prooving inprogress shall skip writing it and wait for response`() {
    // this is to prevent when coordinator is restarted
    // and the request is already in the requests directory being proved by the prover,
    // we must not create a duplicated one
    val proofIndex = ProofIndex(startBlockNumber = 4u, endBlockNumber = 44u)
    // example of PROD file name
    // 8930088-8930101-etv0.2.0-stv2.2.0-getZkProof.json.inprogress.prover-aggregation-97695c877-vgsfg
    val requestProvingInprogressFilePath = proverConfig.requestsDirectory.resolve(
      requestFileNameProvider.getFileName(proofIndex) +
        "some-midle-str.inprogress.prover-aggregation-97695c877-vgsfg",
    )
    // write request with a different block number content to check tha it was not overwritten
    saveToFile(requestProvingInprogressFilePath, ProofRequestDto(blockNumberStart = 4u, blockNumberEnd = 44444u))
    val responseFuture = proverClient.requestProof(proofIndex.toRequest())
    assertRequestWaitingCountIs(1)

    // write response
    saveToFile(responseFilePath(proofIndex), ProofResponseDto(blockNumberStart = 4u, blockNumberEnd = 44u))

    responseFuture.get()

    // assert that the request file was not written again
    assertThat(requestFilePath(proofIndex)).doesNotExist()
    assertThat(proverClient.get()).isEqualTo(0)
  }

  private fun assertRequestWaitingCountIs(expectedCount: Int) {
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(proverClient.get()).isEqualTo(expectedCount.toLong())
      }
  }

  private fun ProofIndex.toRequest(): ProofRequest = ProofRequest(startBlockNumber, endBlockNumber)
}
