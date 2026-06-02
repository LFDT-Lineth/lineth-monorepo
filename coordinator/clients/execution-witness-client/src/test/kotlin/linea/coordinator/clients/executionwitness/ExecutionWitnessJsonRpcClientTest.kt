package linea.coordinator.clients.executionwitness

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.junit5.VertxExtension
import linea.domain.BlockParameter
import linea.executionwitness.ExecutionWitnessError
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.net.URL
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class ExecutionWitnessJsonRpcClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var client: ExecutionWitnessJsonRpcClient
  private lateinit var serverUri: URL

  private val sampleWitnessJson = """
    {
      "state": ["f902"],
      "keys": ["f844"],
      "codes": ["608060"],
      "headers": ["f902"]
    }
  """.trimIndent()

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(options().dynamicPort())
    wiremock.start()
    serverUri = URI("http://127.0.0.1:${wiremock.port()}").toURL()

    val metricsFacade = MicrometerMetricsFacade(SimpleMeterRegistry(), "linea")
    val rpcClient = VertxHttpJsonRpcClientFactory(vertx, metricsFacade).createWithRetries(
      serverUri,
      methodsToRetry = ExecutionWitnessJsonRpcClient.retryableMethods,
      retryConfig = RequestRetryConfig(
        maxRetries = 2u,
        timeout = 10.seconds,
        backoffDelay = 10.milliseconds,
        failuresWarningThreshold = 1u,
      ),
    )
    client = ExecutionWitnessJsonRpcClient(rpcClient)
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun `getExecutionWitness returns parsed witness for block number`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"method\":\"debug_executionWitness\""))
        .withRequestBody(containing("\"params\":[\"42\"]"))
        .willReturn(
          ok(
            JsonObject.of(
              "jsonrpc",
              "2.0",
              "id",
              1,
              "result",
              JsonObject(sampleWitnessJson),
            ).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.BlockNumber(42UL)).get()

    assertThat(result).isEqualTo(
      Ok(
        linea.executionwitness.ExecutionWitness(
          state = listOf("f902".decodeHex()),
          keys = listOf("f844".decodeHex()),
          codes = listOf("608060".decodeHex()),
          headers = listOf("f902".decodeHex()),
        ),
      ),
    )
    wiremock.verify(postRequestedFor(urlEqualTo("/")))
  }

  @Test
  fun `getExecutionWitness returns parsed witness for block hash`() {
    val hash = ByteArray(32) { 0xab.toByte() }
    val hashParam = hash.encodeHex(prefix = true)

    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"params\":[\"$hashParam\"]"))
        .willReturn(
          ok(
            JsonObject.of(
              "jsonrpc",
              "2.0",
              "id",
              1,
              "result",
              JsonObject(sampleWitnessJson),
            ).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.fromHash(hash)).get()

    assertThat(result).isInstanceOf(Ok::class.java)
    wiremock.verify(
      postRequestedFor(urlEqualTo("/"))
        .withRequestBody(containing(hashParam)),
    )
  }

  @Test
  fun `getExecutionWitness returns NULL_RESULT when result is null`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of(
              "jsonrpc",
              "2.0",
              "id",
              1,
              "result",
              null,
            ).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.Tag.LATEST).get()

    assertThat(result).isEqualTo(
      Err(
        linea.error.ErrorResponse(
          ExecutionWitnessError.NULL_RESULT,
          "debug_executionWitness returned null (witness unavailable for block)",
        ),
      ),
    )
  }

  @Test
  fun `getExecutionWitness returns RPC_ERROR on json-rpc error`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of(
              "jsonrpc",
              "2.0",
              "id",
              1,
              "error",
              JsonObject.of(
                "code",
                -32603,
                "message",
                "Internal error",
              ),
            ).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.Tag.LATEST).get()

    assertThat(result).isEqualTo(
      Err(
        linea.error.ErrorResponse(
          ExecutionWitnessError.RPC_ERROR,
          "-32603: Internal error",
        ),
      ),
    )
  }
}
