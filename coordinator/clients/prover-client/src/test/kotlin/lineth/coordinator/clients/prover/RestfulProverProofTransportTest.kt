package lineth.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import com.github.michaelbull.result.Ok
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.junit5.VertxExtension
import linea.domain.BlockIntervalProofIndex
import linea.error.ErrorResponse
import net.consensys.linea.httprest.client.HttpRestClient
import net.consensys.linea.httprest.client.RestErrorType
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class RestfulProverProofTransportTest {

  @Test
  fun `submitRequest posts the request dto under proof_request`() {
    val vertx = Vertx.vertx()
    val wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    try {
      val transport = createTransport<SubmitRequestDto, Any>(
        vertx = vertx,
        wiremock = wiremock,
        proofType = "execution",
        responseDtoClass = Any::class.java,
      )
      wiremock.stubFor(WireMock.post(WireMock.urlEqualTo("/api/v1/jobs/1/execution/100/199")).willReturn(WireMock.ok()))

      transport.submitRequest(proofIndex(), SubmitRequestDto(value = "hello")).get()

      val posted = wiremock.findAll(WireMock.postRequestedFor(WireMock.urlPathMatching(jobsPathPattern("execution"))))
      assertThat(posted).hasSize(1)
      val body = OBJECT_MAPPER.readTree(posted.first().bodyAsString)
      assertThat(body.get("proof_request").get("value")).isNotNull
      assertThat(body.get("proof_request").get("value").asText()).isEqualTo("hello")
    } finally {
      wiremock.stop()
      vertx.close()
    }
  }

  @Test
  fun `isRequestAlreadySubmitted returns true when the job is claimed`() {
    val vertx = Vertx.vertx()
    val wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    try {
      val transport = createTransport<Any, Any>(
        vertx = vertx,
        wiremock = wiremock,
        proofType = "execution",
        responseDtoClass = Any::class.java,
      )
      wiremock.stubFor(
        WireMock.get(WireMock.urlEqualTo("/api/v1/jobs/1/execution/100/199"))
          .willReturn(
            WireMock.okJson(
              RiscvProverClientTestFixtures.proverJobResponseBody(
                proofType = "execution",
                startBlock = 100,
                endBlock = 199,
                status = "queued",
                proofResponse = mapOf("ignored" to true),
              ),
            ),
          ),
      )

      assertThat(transport.isRequestAlreadySubmitted(proofIndex()).get()).isTrue()
    } finally {
      wiremock.stop()
      vertx.close()
    }
  }

  @Test
  fun `request and response lookup use the expected job status`() {
    val vertx = Vertx.vertx()
    val wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    try {
      val transport = createTransport<Any, ProofResponseDto>(
        vertx = vertx,
        wiremock = wiremock,
        proofType = "execution",
        responseDtoClass = ProofResponseDto::class.java,
      )
      val proofIndex = proofIndex()
      val responseDto = ProofResponseDto(value = "response")

      wiremock.stubFor(
        WireMock.get(WireMock.urlEqualTo("/api/v1/jobs/1/execution/100/199"))
          .willReturn(
            WireMock.okJson(
              RiscvProverClientTestFixtures.proverJobResponseBody(
                proofType = "execution",
                startBlock = 100,
                endBlock = 199,
                status = "queued",
                proofResponse = responseDto,
              ),
            ),
          ),
      )
      wiremock.stubFor(
        WireMock.get(WireMock.urlEqualTo("/api/v1/jobs/1/execution/100/199?includeResponse=true"))
          .willReturn(
            WireMock.okJson(
              RiscvProverClientTestFixtures.proverJobResponseBody(
                proofType = "execution",
                startBlock = 100,
                endBlock = 199,
                status = "proved",
                proofResponse = responseDto,
              ),
            ),
          ),
      )

      assertThat(transport.isRequestAlreadySubmitted(proofIndex).get()).isTrue()
      assertThat(transport.isResponseAlreadyExisted(proofIndex).get()).isFalse()
      assertThat(transport.findResponse(proofIndex).get()).isEqualTo(responseDto)
      assertThat(transport.awaitResponse(proofIndex).get()).isEqualTo(responseDto)
    } finally {
      wiremock.stop()
      vertx.close()
    }
  }

  @Test
  fun `removeRequests omits null criteria fields from the request body`() {
    val restClient = RecordingHttpRestClient()
    val transport = RestfulProverProofTransport<Any, Any, BlockIntervalProofIndex>(
      restClient = restClient,
      vertx = Vertx.vertx(),
      chainId = 1,
      proofType = "execution",
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = Any::class.java,
      pollingInterval = 1.seconds,
      pollingTimeout = 1.seconds,
      objectMapper = ObjectMapper(),
    )

    transport.removeRequests(null).get()

    val body = OBJECT_MAPPER.readTree(checkNotNull(restClient.lastPostBody))
    val criteria = checkNotNull(body.get("criteria"))
    assertThat(criteria.has("start_block_gte")).isFalse()
    assertThat(criteria.get("proof_type").asText()).isEqualTo("execution")
  }

  @Test
  fun `removeRequests includes startBlockGte when it is set`() {
    val restClient = RecordingHttpRestClient()
    val transport = RestfulProverProofTransport<Any, Any, BlockIntervalProofIndex>(
      restClient = restClient,
      vertx = Vertx.vertx(),
      chainId = 1,
      proofType = "execution",
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = Any::class.java,
      pollingInterval = 1.seconds,
      pollingTimeout = 1.seconds,
      objectMapper = ObjectMapper(),
    )

    transport.removeRequests(42).get()

    val body = OBJECT_MAPPER.readTree(checkNotNull(restClient.lastPostBody))
    val criteria = checkNotNull(body.get("criteria"))
    assertThat(criteria.get("start_block_gte").asLong()).isEqualTo(42L)
    assertThat(criteria.get("proof_type").asText()).isEqualTo("execution")
  }

  private fun jobsPathPattern(proofType: String): String = "/api/v1/jobs/1/$proofType/.*"

  private fun proofIndex(): BlockIntervalProofIndex = BlockIntervalProofIndex(
    startBlockNumber = 100u,
    endBlockNumber = 199u,
    hash = ByteArray(32) { 0x1a },
    startBlockTimestamp = Instant.fromEpochSeconds(0),
  )

  private fun <RequestDto : Any, ResponseDto> createTransport(
    vertx: Vertx,
    wiremock: WireMockServer,
    proofType: String,
    responseDtoClass: Class<ResponseDto>,
  ): RestfulProverProofTransport<RequestDto, ResponseDto, BlockIntervalProofIndex> {
    return RestfulProverProofTransport(
      restClient = RiscvProverClientTestFixtures.restClient(vertx, wiremock),
      vertx = vertx,
      chainId = 1,
      proofType = proofType,
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = responseDtoClass,
      pollingInterval = 50.milliseconds,
      pollingTimeout = 2.seconds,
      objectMapper = RiscvProverClientTestFixtures.jsonMapper,
    )
  }

  private data class SubmitRequestDto(
    val value: String,
  )

  private data class ProofResponseDto(
    val value: String,
  )

  private class RecordingHttpRestClient : HttpRestClient {
    var lastPostPath: String? = null
      private set
    var lastPostBody: String? = null
      private set

    override fun get(
      path: String,
      params: List<Pair<String, String>>,
      resultMapper: (Any?) -> Any?,
    ): SafeFuture<com.github.michaelbull.result.Result<Any?, ErrorResponse<RestErrorType>>> {
      return SafeFuture.failedFuture(UnsupportedOperationException("GET is not used in this test"))
    }

    override fun post(
      path: String,
      buffer: Buffer,
      resultMapper: (Any?) -> Any?,
    ): SafeFuture<com.github.michaelbull.result.Result<Any?, ErrorResponse<RestErrorType>>> {
      lastPostPath = path
      lastPostBody = buffer.toString()
      return SafeFuture.completedFuture(Ok(null))
    }
  }

  private companion object {
    val OBJECT_MAPPER: ObjectMapper = ObjectMapper()
  }
}
