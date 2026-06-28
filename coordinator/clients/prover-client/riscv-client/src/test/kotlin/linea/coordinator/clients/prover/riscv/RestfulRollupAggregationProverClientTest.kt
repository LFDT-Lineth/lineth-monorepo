package linea.coordinator.clients.prover.riscv

import com.fasterxml.jackson.databind.JsonNode
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.ext.web.client.WebClientOptions
import io.vertx.junit5.VertxExtension
import linea.clients.RollupAggregationProofRequestV1
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.BlockIntervalProofIndex
import linea.kotlin.encodeHex
import net.consensys.linea.httprest.client.VertxHttpRestClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Exercises [FileBasedRollupAggregationProverClient] end-to-end over the [RestfulProverProofTransport]:
 *  - writing a domain request: request -> request DTO -> POST body (`proof_request`);
 *  - reading a response: GET job body -> response DTO -> domain response.
 */
@ExtendWith(VertxExtension::class)
class RestfulRollupAggregationProverClientTest {
  private val jsonMapper = JsonSerialization.proofResponseMapperV1
  private val guestProgramId = "0x8a5fdb137ddae03b9bad034500c0fcee76e1c61d70faca5f32bb7418d73392e1"
  private val proverVersion = "4.0.0-riscv"
  private val proofType = "rollup-aggregation"
  private val chainId = 59144L
  private val jobsPathPattern = "/v1/jobs/$chainId/$proofType/.*"

  private lateinit var wiremock: WireMockServer
  private lateinit var client: RestfulRollupAggregationProverClient

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    val webClientOptions = WebClientOptions()
      .setProtocolVersion(HttpVersion.HTTP_1_1)
      .setDefaultHost("localhost")
      .setDefaultPort(wiremock.port())
    val restClient = VertxHttpRestClient(webClientOptions, PoolOptions(), vertx)
    val transport = RestfulProverProofTransport<
      RestfulRollupAggregationProofRequestDto,
      RollupAggregationProofResponseDto,
      BlockIntervalProofIndex,
      >(
      restClient = restClient,
      vertx = vertx,
      chainId = chainId,
      proofType = proofType,
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = RollupAggregationProofResponseDto::class.java,
      pollingInterval = 50.milliseconds,
      pollingTimeout = 2.seconds,
    )
    client = RestfulRollupAggregationProverClient(
      transport = transport,
      guestProgramId = guestProgramId,
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

    val request = aggregationRequest()
    client.createProofRequest(request).get()

    val postedRequests = wiremock.findAll(WireMock.postRequestedFor(WireMock.urlPathMatching(jobsPathPattern)))
    assertThat(postedRequests).hasSize(1)

    val body = jsonMapper.readTree(postedRequests.first().bodyAsString)
    val postedDto = jsonMapper.treeToValue(
      body.get("proof_request"),
      RestfulRollupAggregationProofRequestDto::class.java,
    )
    val expectedDto = RestfulRollupAggregationProofRequestDtoMapper(guestProgramId).invoke(request).get()
    assertThat(postedDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the job response and maps it to the domain response`() {
    val responseDto = aggregationResponseDto()
    val proofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000567UL,
      hash = ByteArray(32) { 0x1a },
      startBlockTimestamp = Instant.fromEpochSeconds(1763000000),
    )
    wiremock.stubFor(
      WireMock.get(
        WireMock.urlEqualTo("/v1/jobs/$chainId/$proofType/${proofIndex.startBlockNumber}/${proofIndex.endBlockNumber}"),
      ).willReturn(WireMock.okJson(jobResponseBody(status = "proved", proofResponse = responseDto))),
    )

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(RollupAggregationProofResponseDtoMapper(responseDto))
  }

  private fun jobResponseBody(status: String, proofResponse: RollupAggregationProofResponseDto): String {
    val job = jsonMapper.createObjectNode().apply {
      put("proof_type", proofType)
      put("start_block", proofResponse.startBlockNumber)
      put("end_block", proofResponse.publicInputs.endBlockNumber)
      put("status", status)
      put("tier", "large")
      put("attempt", 1)
      set<JsonNode>("proof_response", jsonMapper.valueToTree(proofResponse))
    }
    return jsonMapper.writeValueAsString(job)
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
    proverVersion = proverVersion,
    proof = "0xabcd",
    startBlockNumber = 1000501,
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
      parentProcessedFtxNumber = 16,
      endFtxRollingHash = "0x16",
      endProcessedFtxNumber = 17,
      filteredAddressesHash = "0x18",
      parentShnarf = "0x19",
      endShnarf = "0x1a",
    ),
    l2L1Roots = listOf(Random.nextBytes(32).encodeHex()),
    filteredAddresses = listOf(Random.nextBytes(20).encodeHex()),
    l2MessagingBlocksOffsets = listOf(1, 20, 100),
  )
}
