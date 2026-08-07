package lineth.coordinator.clients.prover.riscv

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.CHAIN_ID
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.COINBASE
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.L2_EXECUTION_GUEST_PROGRAM_ID
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.L2_MESSAGE_SERVICE_ADDRESS
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.jsonMapper
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.l2ExecutionProofRequestV1
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.l2ExecutionProofResponseDto
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.proverJobResponseBody
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.restClient
import lineth.domain.BlockIntervalProofIndex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Exercises [L2ExecutionProverClient] end-to-end over the [RestfulProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> POST body (`proof_request`);
 *  - reading a response: GET job body -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class RestfulL2ExecutionProverClientTest {
  private val proofType = "l2-execution"
  private val jobsPathPattern = "/v1/jobs/$CHAIN_ID/$proofType/.*"

  private lateinit var wiremock: WireMockServer
  private lateinit var client: L2ExecutionProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    val transport = RestfulProverProofTransport<
      L2ExecutionProofRequestDto,
      L2ExecutionProofResponseDto,
      BlockIntervalProofIndex,
      >(
      restClient = restClient(vertx, wiremock),
      vertx = vertx,
      chainId = CHAIN_ID,
      proofType = proofType,
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = L2ExecutionProofResponseDto::class.java,
      pollingInterval = 50.milliseconds,
      pollingTimeout = 2.seconds,
    )
    client = L2ExecutionProverClient(
      transport = transport,
      guestProgramId = L2_EXECUTION_GUEST_PROGRAM_ID,
      l2MessageServiceAddress = L2_MESSAGE_SERVICE_ADDRESS,
      coinbase = COINBASE,
    )
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun `createProofRequest posts the request DTO to the prover service`() {
    // no existing job -> isRequestAlreadySubmitted == false
    wiremock.stubFor(WireMock.get(WireMock.urlPathMatching(jobsPathPattern)).willReturn(WireMock.notFound()))
    wiremock.stubFor(WireMock.post(WireMock.urlPathMatching(jobsPathPattern)).willReturn(WireMock.ok()))

    val request = l2ExecutionProofRequestV1()
    client.createProofRequest(request).get()

    val postedRequests = wiremock.findAll(WireMock.postRequestedFor(WireMock.urlPathMatching(jobsPathPattern)))
    assertThat(postedRequests).hasSize(1)

    val body = jsonMapper.readTree(postedRequests.first().bodyAsString)
    val postedDto = jsonMapper.treeToValue(body.get("proof_request"), L2ExecutionProofRequestDto::class.java)
    val expectedDto = L2ExecutionProofRequestDtoMapper(
      L2_EXECUTION_GUEST_PROGRAM_ID,
      L2_MESSAGE_SERVICE_ADDRESS,
      COINBASE,
    ).invoke(request).get()
    assertThat(postedDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the job response and maps it to the domain response`() {
    val proofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000503UL,
      hash = ByteArray(32) { 0x1a },
      startBlockTimestamp = Instant.fromEpochSeconds(1763000123),
    )
    val responseDto = l2ExecutionProofResponseDto(1000501L, 1000503L)
    wiremock.stubFor(
      WireMock.get(
        WireMock.urlEqualTo("/v1/jobs/$CHAIN_ID/$proofType/1000501/1000503"),
      ).willReturn(
        WireMock.okJson(
          proverJobResponseBody(
            proofType = proofType,
            startBlock = responseDto.startBlockNumber,
            endBlock = responseDto.publicInputs.endBlockNumber,
            proofResponse = responseDto,
          ),
        ),
      ),
    )

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(L2ExecutionProofResponseDtoMapper(responseDto))
  }
}
