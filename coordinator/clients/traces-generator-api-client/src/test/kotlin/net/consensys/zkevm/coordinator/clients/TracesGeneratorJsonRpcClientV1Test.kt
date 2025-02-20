package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
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
import linea.domain.BlockNumberAndHash
import linea.kotlin.ByteArrayExt
import linea.kotlin.encodeHex
import net.consensys.linea.async.get
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.TracingModuleV1
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.net.URL
import java.util.concurrent.ExecutionException
import kotlin.collections.set
import kotlin.random.Random
import kotlin.random.nextUInt
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class TracesGeneratorJsonRpcClientV1Test {
  private lateinit var wiremock: WireMockServer
  private lateinit var tracesGeneratorClient: TracesGeneratorJsonRpcClientV1
  private lateinit var meterRegistry: SimpleMeterRegistry
  private val blockL1Size = 101U
  private val tracesCountersValid: Map<String, Long> =
    TracingModuleV1.entries
      .fold(mutableMapOf()) { acc: MutableMap<String, Long>,
        evmModule: TracingModuleV1 ->
        acc[evmModule.name] = Random.nextUInt(0u, UInt.MAX_VALUE).toLong()
        acc
      }
      .also {
        // add edge case of max UInt
        it[TracingModuleV1.EXT.name] = UInt.MAX_VALUE.toLong()
      }
  private lateinit var fakeTracesServerUri: URL
  private lateinit var vertxHttpJsonRpcClient: JsonRpcClient
  private val expectedTracesApiVersion = "2.3.4"
  private val rawExecutionTracesVersion = "9.8.7"

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(options().dynamicPort())
    wiremock.start()

    fakeTracesServerUri = URI("http://127.0.0.1:" + wiremock.port()).toURL()
    meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    val rpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, metricsFacade)
    vertxHttpJsonRpcClient = rpcClientFactory.createWithRetries(
      fakeTracesServerUri,
      methodsToRetry = TracesGeneratorJsonRpcClientV1.retryableMethods,
      retryConfig = RequestRetryConfig(
        maxRetries = 2u,
        timeout = 10.seconds,
        backoffDelay = 10.milliseconds,
        failuresWarningThreshold = 1u
      )
    )

    tracesGeneratorClient = TracesGeneratorJsonRpcClientV1(
      vertxHttpJsonRpcClient,
      TracesGeneratorJsonRpcClientV1.Config(
        rawExecutionTracesVersion = rawExecutionTracesVersion,
        expectedTracesApiVersion = expectedTracesApiVersion
      )
    )
  }

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    wiremock.stop()
    vertxStopFuture.get()
  }

  private fun successTracesCountersResponse(tracesEngineVersion: String = "0.0.1"): JsonObject {
    return JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      "1",
      "result",
      mapOf(
        "blockL1Size" to blockL1Size.toString(),
        "tracesEngineVersion" to tracesEngineVersion,
        "tracesCounters" to tracesCountersValid
      )
    )
  }

  private fun jsonRpcErrorResponse(errorMessage: String): JsonObject {
    return JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      "1",
      "error",
      mapOf("code" to "1", "message" to errorMessage)
    )
  }

  @Test
  fun getTracesCounters_allEvmModulesOk() {
    val tracesEngineVersion = "0.0.1"
    val response = successTracesCountersResponse(tracesEngineVersion)

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString())
        )
    )

    val blockIdAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = tracesGeneratorClient.rollupGetTracesCounters(blockIdAndHash)
    resultFuture.get()

    assertThat(resultFuture)
      .isCompletedWithValue(
        Ok(
          GetTracesCountersResponse(
            TracesCountersV1(
              tracesCountersValid
                .mapKeys { TracingModuleV1.valueOf(it.key) }
                .mapValues { it.value.toUInt() }
            ),
            tracesEngineVersion
          )
        )
      )

    val expectedJsonRequest = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "method",
      "rollup_getBlockTracesCountersV1",
      "params",
      mapOf(
        "block" to mapOf(
          "blockNumber" to "1",
          "blockHash" to blockIdAndHash.hash.encodeHex()
        ),
        "rawExecutionTracesVersion" to rawExecutionTracesVersion,
        "expectedTracesApiVersion" to expectedTracesApiVersion
      )
    )
    wiremock.verify(
      postRequestedFor(urlEqualTo("/"))
        .withHeader("Content-Type", equalTo("application/json"))
        .withRequestBody(equalToJson(expectedJsonRequest.toString(), false, true))
    )
  }

  @Test
  fun `getTracesCounters when response misses EVM module returns error`() {
    val tracesCountersMissingModule =
      tracesCountersValid.toMutableMap().apply { this.remove(TracingModuleV1.WCP.name) }

    val tracesEngineVersion = "0.0.1"
    val response =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        "1",
        "result",
        mapOf(
          "blockL1Size" to blockL1Size.toString(),
          "tracesEngineVersion" to tracesEngineVersion,
          "tracesCounters" to tracesCountersMissingModule
        )
      )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString())
        )
    )

    val blockIdAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = tracesGeneratorClient.rollupGetTracesCounters(blockIdAndHash)
    val exception = assertThrows<ExecutionException> { resultFuture.get() }
    assertThat(exception.message).contains("missing modules: WCP")
  }

  @Test
  fun `getTracesCounters when response has unrecognized evm module returns error`() {
    val tracesCountersMissingModule =
      tracesCountersValid.toMutableMap().apply { this["NEW_EVM_MODULE"] = 100 }

    val tracesEngineVersion = "0.0.1"
    val response =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        "1",
        "result",
        mapOf(
          "blockL1Size" to blockL1Size.toString(),
          "tracesEngineVersion" to tracesEngineVersion,
          "tracesCounters" to tracesCountersMissingModule
        )
      )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "application/json").withBody(response.toString())
        )
    )

    val blockIdAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = tracesGeneratorClient.rollupGetTracesCounters(blockIdAndHash)
    val exception = assertThrows<ExecutionException> { resultFuture.get() }
    assertThat(exception.message).contains("unsupported modules: NEW_EVM_MODULE")
  }

  @Test
  fun generateConflatedTracesToFile() {
    val startBlockNumber = 50L
    val endBlockNumber = 100L

    val tracesEngineVersion = "0.0.1"
    val conflatedTracesFileName =
      "$startBlockNumber-$endBlockNumber.conflated.v$tracesEngineVersion.json.gz"

    val response =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        "1",
        "result",
        mapOf(
          "tracesEngineVersion" to tracesEngineVersion,
          "conflatedTracesFileName" to conflatedTracesFileName
        )
      )

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(response.toString().toByteArray())
        )
    )

    val blocks = listOf(
      BlockNumberAndHash(1U, ByteArrayExt.random32()),
      BlockNumberAndHash(2U, ByteArrayExt.random32()),
      BlockNumberAndHash(3U, ByteArrayExt.random32())
    )

    val resultFuture =
      tracesGeneratorClient.rollupGenerateConflatedTracesToFile(blocks)
    resultFuture.get()

    assertThat(resultFuture)
      .isCompletedWithValue(
        Ok(GenerateTracesResponse(conflatedTracesFileName, tracesEngineVersion))
      )

    val expectedJsonRequest = JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "method",
      "rollup_generateConflatedTracesToFileV1",
      "params",
      JsonObject.of(
        "blocks",
        blocks.map {
          JsonObject.of(
            "blockNumber",
            it.number.toString(),
            "blockHash",
            it.hash.encodeHex()
          )
        },
        "rawExecutionTracesVersion",
        rawExecutionTracesVersion,
        "expectedTracesApiVersion",
        expectedTracesApiVersion
      )
    )
    wiremock.verify(
      postRequestedFor(urlEqualTo("/"))
        .withHeader("Content-Type", equalTo("application/json"))
        .withRequestBody(equalToJson(expectedJsonRequest.toString(), false, true))
    )
  }

  @Test
  fun error_getTracesCounter() {
    val errorMessage = "BLOCK_MISSING_IN_CHAIN: Block 1 doesn't exist in the chain"
    val response = jsonRpcErrorResponse(errorMessage)

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(response.toString().toByteArray())
        )
    )

    val blockIdAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = tracesGeneratorClient.rollupGetTracesCounters(blockIdAndHash)
    resultFuture.get()

    assertThat(resultFuture)
      .isCompletedWithValue(
        Err(ErrorResponse(TracesServiceErrorType.BLOCK_MISSING_IN_CHAIN, errorMessage))
      )
  }

  @Test
  fun error_generateConflatedTracesToFile() {
    val errorMessage = "BLOCK_RANGE_TOO_LARGE: Block range between 50 and 100 is too large"
    val response = jsonRpcErrorResponse(errorMessage)

    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(response.toString().toByteArray())
        )
    )

    val blocks = listOf(
      BlockNumberAndHash(1U, ByteArrayExt.random32()),
      BlockNumberAndHash(2U, ByteArrayExt.random32()),
      BlockNumberAndHash(3U, ByteArrayExt.random32())
    )
    val resultFuture =
      tracesGeneratorClient.rollupGenerateConflatedTracesToFile(blocks)
    resultFuture.get()

    assertThat(resultFuture)
      .isCompletedWithValue(
        Err(ErrorResponse(TracesServiceErrorType.BLOCK_RANGE_TOO_LARGE, errorMessage))
      )
  }

  @Test
  fun error_generateConflatedTracesToFile_retriesRequest() {
    val tracesEngineVersion = "0.0.1"
    val errorMessage = "BLOCK_MISSING_IN_CHAIN: Block 1 doesn't exist in the chain"
    val jsonRpcErrorResponse = jsonRpcErrorResponse(errorMessage)
    wiremock.stubFor(
      post("/")
        .inScenario("retry")
        .whenScenarioStateIs(STARTED)
        .willSetStateTo("first failure")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          aResponse()
            .withStatus(500)
            .withBody("Internal Server Error")
        )
    )
    wiremock.stubFor(
      post("/")
        .inScenario("retry")
        .whenScenarioStateIs("first failure")
        .willSetStateTo("second failure")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          aResponse()
            .withStatus(200)
            .withBody(jsonRpcErrorResponse.toString())
        )
    )
    wiremock.stubFor(
      post("/")
        .inScenario("retry")
        .whenScenarioStateIs("second failure")
        .willSetStateTo("success")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(
              JsonObject.of(
                "jsonrpc",
                "2.0",
                "id",
                "1",
                "result",
                mapOf(
                  "tracesEngineVersion" to tracesEngineVersion,
                  "conflatedTracesFileName" to "conflated-traces-1-3.json"
                )
              ).toString()
            )
        )
    )

    tracesGeneratorClient = TracesGeneratorJsonRpcClientV1(
      vertxHttpJsonRpcClient,
      TracesGeneratorJsonRpcClientV1.Config(
        rawExecutionTracesVersion = rawExecutionTracesVersion,
        expectedTracesApiVersion = expectedTracesApiVersion
      )
    )

    val blockIdAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = tracesGeneratorClient.rollupGenerateConflatedTracesToFile(listOf(blockIdAndHash))

    assertThat(resultFuture.get()).isInstanceOf(Ok::class.java)
  }
}
