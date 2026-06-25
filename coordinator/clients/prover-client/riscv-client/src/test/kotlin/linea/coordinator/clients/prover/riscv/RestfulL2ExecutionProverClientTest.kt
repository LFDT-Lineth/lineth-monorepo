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
import linea.clients.ChainConfig
import linea.clients.ExecutionPayload
import linea.clients.ExecutionRequests
import linea.clients.ExecutionWitness
import linea.clients.L2ExecutionProofRequestV1
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.ExecutionProofIndex
import net.consensys.linea.httprest.client.VertxHttpRestClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.math.BigInteger
import kotlin.random.Random
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
  private val jsonMapper = JsonSerialization.proofResponseMapperV1
  private val guestProgramId = "0x17d2e0660946012c80c5fe6bbecc2076a6f6f5aa58606efe66a14426d2ffe46f"
  private val proverVersion = "4.0.0-riscv"
  private val proofType = "l2-execution"
  private val chainId = 59144L
  private val jobsPathPattern = "/v1/jobs/$chainId/$proofType/.*"
  private val chainConfig = ChainConfigDto(
    l2MessageServiceAddress = "0x508ca82df566dcd1b0019d2dedf7e3d6f7ad6dde",
    coinbase = "0x0000000000000000000000000000000000000000",
    chainId = chainId,
    forkName = "Amsterdam",
  )

  private lateinit var wiremock: WireMockServer
  private lateinit var client: L2ExecutionProverClient

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
      L2ExecutionProofRequestDto,
      L2ExecutionProofResponseDto,
      ExecutionProofIndex,
      >(
      restClient = restClient,
      vertx = vertx,
      chainId = chainId,
      proofType = proofType,
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = L2ExecutionProofResponseDto::class.java,
      pollingInterval = 50.milliseconds,
      pollingTimeout = 2.seconds,
    )
    client = L2ExecutionProverClient(
      transport = transport,
      guestProgramId = guestProgramId,
      chainConfig = chainConfig,
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

    val request = l2Request()
    client.createProofRequest(request).get()

    val postedRequests = wiremock.findAll(WireMock.postRequestedFor(WireMock.urlPathMatching(jobsPathPattern)))
    assertThat(postedRequests).hasSize(1)

    val body = jsonMapper.readTree(postedRequests.first().bodyAsString)
    val postedDto = jsonMapper.treeToValue(body.get("proof_request"), L2ExecutionProofRequestDto::class.java)
    val expectedDto = L2ExecutionProofRequestDtoMapper(guestProgramId, chainConfig).invoke(request).get()
    assertThat(postedDto).isEqualTo(expectedDto)
  }

  @Test
  fun `findProofResponse reads the job response and maps it to the domain response`() {
    val responseDto = l2ResponseDto()
    val proofIndex = ExecutionProofIndex(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000503UL,
      startBlockTimestamp = Instant.fromEpochSeconds(1763000123),
    )
    wiremock.stubFor(
      WireMock.get(WireMock.urlEqualTo("/v1/jobs/$chainId/$proofType/1000501/1000503")).willReturn(
        WireMock.okJson(jobResponseBody(status = "proved", proofResponse = responseDto)),
      ),
    )

    val response = client.findProofResponse(proofIndex).get()

    assertThat(response).isEqualTo(L2ExecutionProofResponseDtoMapper(proofIndex, responseDto))
  }

  private fun jobResponseBody(status: String, proofResponse: L2ExecutionProofResponseDto): String {
    val job = jsonMapper.createObjectNode().apply {
      put("proof_type", proofType)
      put("start_block", proofResponse.startBlockNumber)
      put("end_block", proofResponse.publicInputs.endBlockNumber)
      put("status", status)
      put("tier", "small")
      put("attempt", 1)
      set<JsonNode>("proof_response", jsonMapper.valueToTree(proofResponse))
    }
    return jsonMapper.writeValueAsString(job)
  }

  private fun l2Request(): L2ExecutionProofRequestV1 = L2ExecutionProofRequestV1(
    executionPayloads = listOf(
      executionPayload(blockNumber = 1000501UL),
      executionPayload(blockNumber = 1000503UL),
    ),
    executionWitnesses = listOf(
      ExecutionWitness(
        blockNumber = 1000501UL,
        state = emptyList(),
        codes = emptyList(),
        headers = emptyList(),
      ),
      ExecutionWitness(
        blockNumber = 1000503UL,
        state = emptyList(),
        codes = emptyList(),
        headers = emptyList(),
      ),
    ),
    executionRequests = listOf(
      ExecutionRequests(
        blockNumber = 1000501UL,
        executionRequests = emptyList(),
      ),
      ExecutionRequests(
        blockNumber = 1000503UL,
        executionRequests = emptyList(),
      ),
    ),
    forcedTransactions = emptyList(),
    chainConfig = ChainConfig(
      l2MessageServiceContract = ByteArray(20) { 1 },
      coinbase = ByteArray(20) { 2 },
      chainId = 1000UL,
    ),
    parentFtxRollingHash = ByteArray(32) { 1 },
    parentLastProcessedFtxNumber = 100UL,
  )

  private fun executionPayload(blockNumber: ULong): ExecutionPayload = ExecutionPayload(
    parentHash = Random.nextBytes(32),
    feeRecipient = Random.nextBytes(20),
    stateRoot = Random.nextBytes(32),
    receiptsRoot = Random.nextBytes(32),
    logsBloom = Random.nextBytes(256),
    prevRandao = Random.nextBytes(32),
    blockNumber = blockNumber,
    gasLimit = Random.nextLong(0, Long.MAX_VALUE).toULong(),
    gasUsed = Random.nextLong(0, Long.MAX_VALUE).toULong(),
    timestamp = 1000UL,
    extraData = Random.nextBytes(32),
    baseFeePerGas = BigInteger.valueOf(Random.nextLong(0, Long.MAX_VALUE)),
    blockHash = Random.nextBytes(32),
    transactions = emptyList(),
    withdrawals = emptyList(),
    blobGasUsed = 0UL,
    excessBlobGas = 0UL,
    blockAccessList = ByteArray(0),
  )

  private fun l2ResponseDto(): L2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
    proverVersion = proverVersion,
    startBlockNumber = 1000501,
    proof = "0xabcd",
    publicInputs = L2ExecutionProofPublicInputsDto(
      parentBlockHash = "0x0a",
      endBlockHash = "0x0b",
      endBlockNumber = 1000503,
      endBlockTimestamp = 1763000123,
      l2L1MessagesHash = "0x01",
      parentL1L2BridgeRollingHash = "0x02",
      parentL1L2BridgeRollingHashMessageNumber = 3,
      endL1L2BridgeRollingHash = "0x04",
      endL1L2BridgeRollingHashMessageNumber = 5,
      dynamicChainConfigHash = "0xc0ffee",
      parentFtxRollingHash = "0x06",
      endFtxRollingHash = "0x07",
      lastProcessedFtxNumber = 8,
      filteredAddressesHash = "0x09",
      txFromsHash = "0x0c",
    ),
    l2L1Messages = listOf("0xaa"),
    txFroms = listOf("0xbb"),
    filteredAddresses = emptyList(),
  )
}
