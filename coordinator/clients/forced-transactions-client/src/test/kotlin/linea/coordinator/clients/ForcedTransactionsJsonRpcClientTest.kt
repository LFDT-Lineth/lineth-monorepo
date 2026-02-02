package linea.coordinator.clients

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.aResponse
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.equalTo
import com.github.tomakehurst.wiremock.client.WireMock.equalToJson
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import com.github.tomakehurst.wiremock.stubbing.Scenario.STARTED
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.junit5.VertxExtension
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.forcedtx.ForcedTransactionRequest
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.net.URL
import java.util.concurrent.ExecutionException
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class ForcedTransactionsJsonRpcClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var client: ForcedTransactionsJsonRpcClient
  private lateinit var meterRegistry: SimpleMeterRegistry
  private lateinit var fakeServerUri: URL

  companion object {
    // Fixed seed for reproducible test data
    private const val TEST_SEED = 12345L

    /**
     * Creates fake RLP-encoded transaction bytes for testing.
     * Uses a seeded random generator for reproducible tests.
     */
    fun fakeRlpTransaction(seed: Long = TEST_SEED, index: Int = 0): ByteArray {
      val random = Random(seed + index)
      val prefix = byteArrayOf(0x02)
      val payload = random.nextBytes(112)
      return prefix + payload
    }
  }

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(options().dynamicPort())
    wiremock.start()

    fakeServerUri = URI("http://127.0.0.1:" + wiremock.port()).toURL()
    meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    val rpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, metricsFacade)

    val vertxHttpJsonRpcClient = rpcClientFactory.createWithRetries(
      fakeServerUri,
      methodsToRetry = ForcedTransactionsJsonRpcClient.retryableMethods,
      retryConfig = RequestRetryConfig(
        maxRetries = 2u,
        timeout = 10.seconds,
        backoffDelay = 10.milliseconds,
        failuresWarningThreshold = 1u,
      ),
    )

    client = ForcedTransactionsJsonRpcClient(vertxHttpJsonRpcClient)
  }

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    wiremock.stop()
    vertxStopFuture.get()
  }

  @Test
  fun lineaSendForcedRawTransaction_singleTransaction_success() {
    val txRlp = fakeRlpTransaction()
    val expectedHash = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      listOf(
        mapOf(
          "forcedTransactionNumber" to 6,
          "hash" to expectedHash,
        ),
      ),
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    val request = ForcedTransactionRequest(
      ftxNumber = 6uL,
      deadlineBlockNumber = 4046uL,
      ftxRlp = txRlp,
    )

    val resultFuture = client.lineaSendForcedRawTransaction(listOf(request))
    val result = resultFuture.get()

    assertThat(result).hasSize(1)
    assertThat(result[0].ftxNumber).isEqualTo(6uL)
    assertThat(result[0].ftxHash).isEqualTo(expectedHash.decodeHex())
    assertThat(result[0].ftxError).isNull()

    // Verify request format
    val expectedJsonRequest = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "method",
      "linea_sendForcedRawTransaction",
      "params",
      listOf(
        listOf(
          mapOf(
            "forcedTransactionNumber" to 6,
            "transaction" to txRlp.encodeHex(),
            "deadlineBlockNumber" to "4046",
          ),
        ),
      ),
    )

    wiremock.verify(
      postRequestedFor(urlEqualTo("/"))
        .withHeader("Content-Type", equalTo("application/json"))
        .withRequestBody(equalToJson(expectedJsonRequest.toString(), false, true)),
    )
  }

  @Test
  fun lineaSendForcedRawTransaction_multipleTransactions_success() {
    val txRlp1 = fakeRlpTransaction(index = 0)
    val txRlp2 = fakeRlpTransaction(index = 1)
    val hash1 = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
    val hash2 = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      listOf(
        mapOf(
          "forcedTransactionNumber" to 6,
          "hash" to hash1,
        ),
        mapOf(
          "forcedTransactionNumber" to 7,
          "hash" to hash2,
        ),
      ),
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    val requests = listOf(
      ForcedTransactionRequest(ftxNumber = 6uL, deadlineBlockNumber = 4046uL, ftxRlp = txRlp1),
      ForcedTransactionRequest(ftxNumber = 7uL, deadlineBlockNumber = 4050uL, ftxRlp = txRlp2),
    )

    val result = client.lineaSendForcedRawTransaction(requests).get()

    assertThat(result).hasSize(2)
    assertThat(result[0].ftxNumber).isEqualTo(6uL)
    assertThat(result[0].ftxHash).isEqualTo(hash1.decodeHex())
    assertThat(result[1].ftxNumber).isEqualTo(7uL)
    assertThat(result[1].ftxHash).isEqualTo(hash2.decodeHex())
  }

  @Test
  fun lineaSendForcedRawTransaction_partialFailure_returnsErrorsPerTransaction() {
    val txRlp1 = fakeRlpTransaction(index = 0)
    val txRlp2 = fakeRlpTransaction(index = 1)
    val hash1 = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      listOf(
        mapOf(
          "forcedTransactionNumber" to 6,
          "hash" to hash1,
        ),
        mapOf(
          "forcedTransactionNumber" to 7,
          "error" to "Invalid nonce",
        ),
      ),
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    val requests = listOf(
      ForcedTransactionRequest(ftxNumber = 6uL, deadlineBlockNumber = 4046uL, ftxRlp = txRlp1),
      ForcedTransactionRequest(ftxNumber = 7uL, deadlineBlockNumber = 4050uL, ftxRlp = txRlp2),
    )

    val result = client.lineaSendForcedRawTransaction(requests).get()

    assertThat(result).hasSize(2)
    assertThat(result[0].ftxNumber).isEqualTo(6uL)
    assertThat(result[0].ftxHash).isEqualTo(hash1.decodeHex())
    assertThat(result[0].ftxError).isNull()
    assertThat(result[1].ftxNumber).isEqualTo(7uL)
    assertThat(result[1].ftxHash).isNull()
    assertThat(result[1].ftxError).isEqualTo("Invalid nonce")
  }

  @Test
  fun lineaFindForcedTransactionStatus_found_returnsStatus() {
    val expectedHash = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
    val expectedFrom = "0x6221a9c005f6e47eb398fd867784cacfdcfff4e7"

    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      mapOf(
        "forcedTransactionNumber" to 6,
        "blockNumber" to "0xeff35f",
        "blockTimestamp" to 1234567890,
        "from" to expectedFrom,
        "inclusionResult" to "Included",
        "transactionHash" to expectedHash,
      ),
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    val result = client.lineaFindForcedTransactionStatus(6uL).get()

    assertThat(result).isNotNull
    assertThat(result!!.ftxNumber).isEqualTo(6uL)
    assertThat(result.blockNumber).isEqualTo(0xeff35fuL)
    assertThat(result.blockTimestamp.epochSeconds).isEqualTo(1234567890L)
    assertThat(result.inclusionResult).isEqualTo(ForcedTransactionInclusionResult.Included)
    assertThat(result.ftxHash).isEqualTo(expectedHash.decodeHex())
    assertThat(result.from).isEqualTo(expectedFrom.decodeHex())

    // Verify request format
    val expectedJsonRequest = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "method",
      "linea_getForcedTransactionInclusionStatus",
      "params",
      listOf(6),
    )

    wiremock.verify(
      postRequestedFor(urlEqualTo("/"))
        .withHeader("Content-Type", equalTo("application/json"))
        .withRequestBody(equalToJson(expectedJsonRequest.toString(), false, true)),
    )
  }

  @Test
  fun lineaFindForcedTransactionStatus_notFound_returnsNull() {
    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      null,
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    val result = client.lineaFindForcedTransactionStatus(999uL).get()

    assertThat(result).isNull()
  }

  @Test
  fun lineaFindForcedTransactionStatus_badNonce_returnsStatus() {
    val expectedHash = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
    val expectedFrom = "0x6221a9c005f6e47eb398fd867784cacfdcfff4e7"

    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      mapOf(
        "forcedTransactionNumber" to 6,
        "blockNumber" to "0xeff35f",
        "blockTimestamp" to 1234567890,
        "from" to expectedFrom,
        "inclusionResult" to "BadNonce",
        "transactionHash" to expectedHash,
      ),
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    val result = client.lineaFindForcedTransactionStatus(6uL).get()

    assertThat(result).isNotNull
    assertThat(result!!.inclusionResult).isEqualTo(ForcedTransactionInclusionResult.BadNonce)
  }

  @Test
  fun lineaFindForcedTransactionStatus_allInclusionResults() {
    val expectedHash = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
    val expectedFrom = "0x6221a9c005f6e47eb398fd867784cacfdcfff4e7"

    for (inclusionResult in ForcedTransactionInclusionResult.entries) {
      wiremock.resetAll()

      val response = JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        1,
        "result",
        mapOf(
          "forcedTransactionNumber" to 6,
          "blockNumber" to "0xeff35f",
          "blockTimestamp" to 1234567890,
          "from" to expectedFrom,
          "inclusionResult" to inclusionResult.name,
          "transactionHash" to expectedHash,
        ),
      )

      wiremock.stubFor(
        post("/")
          .withHeader("Content-Type", containing("application/json"))
          .willReturn(
            ok().withHeader("Content-type", "application/json").withBody(response.toString()),
          ),
      )

      val result = client.lineaFindForcedTransactionStatus(6uL).get()

      assertThat(result).isNotNull
      assertThat(result!!.inclusionResult)
        .withFailMessage("Expected $inclusionResult but got ${result.inclusionResult}")
        .isEqualTo(inclusionResult)
    }
  }

  @Test
  fun lineaSendForcedRawTransaction_retriesOnFailure() {
    val txRlp = fakeRlpTransaction()
    val expectedHash = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

    val successResponse = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      listOf(
        mapOf(
          "forcedTransactionNumber" to 6,
          "hash" to expectedHash,
        ),
      ),
    )

    // First call fails with 500
    wiremock.stubFor(
      post("/")
        .inScenario("retry")
        .whenScenarioStateIs(STARTED)
        .willSetStateTo("first failure")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          aResponse()
            .withStatus(500)
            .withBody("Internal Server Error"),
        ),
    )

    // Second call succeeds
    wiremock.stubFor(
      post("/")
        .inScenario("retry")
        .whenScenarioStateIs("first failure")
        .willSetStateTo("success")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(successResponse.toString()),
        ),
    )

    val request = ForcedTransactionRequest(
      ftxNumber = 6uL,
      deadlineBlockNumber = 4046uL,
      ftxRlp = txRlp,
    )

    val result = client.lineaSendForcedRawTransaction(listOf(request)).get()

    assertThat(result).hasSize(1)
    assertThat(result[0].ftxNumber).isEqualTo(6uL)
    assertThat(result[0].ftxHash).isEqualTo(expectedHash.decodeHex())
  }

  @Test
  fun lineaSendForcedRawTransaction_jsonRpcError_throwsException() {
    val txRlp = fakeRlpTransaction()

    val errorResponse = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "error",
      mapOf(
        "code" to -32603,
        "message" to "Internal error",
      ),
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(errorResponse.toString()),
        ),
    )

    val request = ForcedTransactionRequest(
      ftxNumber = 6uL,
      deadlineBlockNumber = 4046uL,
      ftxRlp = txRlp,
    )

    val exception = assertThrows<ExecutionException> {
      client.lineaSendForcedRawTransaction(listOf(request)).get()
    }

    assertThat(exception.cause).isInstanceOf(RuntimeException::class.java)
    assertThat(exception.cause!!.message).contains("JSON-RPC error")
    assertThat(exception.cause!!.message).contains("-32603")
  }

  @Test
  fun lineaGetForcedTransactionStatus_notFound_throwsException() {
    val response = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      null,
    )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString()),
        ),
    )

    // Test lineaGetForcedTransactionStatus (not lineaFindForcedTransactionStatus)
    // which should throw when result is null
    val exception = assertThrows<ExecutionException> {
      client.lineaGetForcedTransactionStatus(999uL).get()
    }

    assertThat(exception.cause).isInstanceOf(IllegalStateException::class.java)
    assertThat(exception.cause!!.message).contains("Forced transaction not found")
  }
}
