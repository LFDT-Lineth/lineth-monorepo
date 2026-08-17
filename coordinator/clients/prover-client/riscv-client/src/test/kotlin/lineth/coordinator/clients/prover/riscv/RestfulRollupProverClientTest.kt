package lineth.coordinator.clients.prover.riscv

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockIntervalProofIndex
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.CHAIN_ID
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.ROLLUP_GUEST_PROGRAM_ID
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.blockIntervalProofIndex
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.jsonMapper
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.proverJobResponseBody
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.restClient
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.rollupProofRequestV1
import lineth.coordinator.clients.prover.riscv.RiscVProverClientTestFixtures.rollupProofResponseDto
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Exercises [RestfulRollupProverClient] end-to-end over the [RestfulProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> POST body (`proof_request`);
 *  - reading a response: GET job body -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class RestfulRollupProverClientTest {
  private val proofType = "rollup"
  private val jobsPathPattern = "/v1/jobs/$CHAIN_ID/$proofType/.*"

  private lateinit var wiremock: WireMockServer
  private lateinit var client: RestfulRollupProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    val transport = RestfulProverProofTransport<
      RestfulRollupProofRequestDto,
      RollupProofResponseDto,
      BlockIntervalProofIndex,
      >(
      restClient = restClient(vertx, wiremock),
      vertx = vertx,
      chainId = CHAIN_ID,
      proofType = proofType,
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = RollupProofResponseDto::class.java,
      pollingInterval = 50.milliseconds,
      pollingTimeout = 2.seconds,
    )
    client = RestfulRollupProverClient(
      transport = transport,
      guestProgramId = ROLLUP_GUEST_PROGRAM_ID,
      chainId = CHAIN_ID,
    )
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun `createProofRequest posts the request DTO to the prover service`() {
    wiremock.stubFor(WireMock.get(WireMock.urlPathMatching(jobsPathPattern)).willReturn(WireMock.notFound()))
    wiremock.stubFor(WireMock.post(WireMock.urlPathMatching(jobsPathPattern)).willReturn(WireMock.ok()))

    val request = rollupProofRequestV1(
      l2Executions = listOf(
        blockIntervalProofIndex(1000501UL, 1000510UL),
        blockIntervalProofIndex(1000511UL, 1000520UL, startBlockTimestamp = Instant.fromEpochSeconds(1763002000)),
      ),
    )
    client.createProofRequest(request).get()

    val postedRequests = wiremock.findAll(WireMock.postRequestedFor(WireMock.urlPathMatching(jobsPathPattern)))
    assertThat(postedRequests).hasSize(1)

    val body = jsonMapper.readTree(postedRequests.first().bodyAsString)
    val postedDto = jsonMapper.treeToValue(body.get("proof_request"), RestfulRollupProofRequestDto::class.java)
    val expectedDto = RestfulRollupProofRequestDtoMapper(ROLLUP_GUEST_PROGRAM_ID, CHAIN_ID).invoke(request).get()
    assertThat(postedDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the job response and maps it to the domain response`() {
    val proofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000520UL,
      hash = ByteArray(32) { 0x1a },
      startBlockTimestamp = Instant.fromEpochSeconds(1763000457),
    )
    val responseDto = rollupProofResponseDto(1000501L, 1000520L)
    wiremock.stubFor(
      WireMock.get(WireMock.urlEqualTo("/v1/jobs/$CHAIN_ID/$proofType/1000501/1000520")).willReturn(
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

    assertThat(response).isEqualTo(RollupProofResponseDtoMapper(responseDto))
  }
}
