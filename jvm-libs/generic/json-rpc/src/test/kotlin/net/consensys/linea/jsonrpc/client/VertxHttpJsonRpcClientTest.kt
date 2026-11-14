package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.status
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import com.github.tomakehurst.wiremock.matching.EqualToJsonPattern
import com.github.tomakehurst.wiremock.matching.EqualToPattern
import com.github.tomakehurst.wiremock.matching.RequestPatternBuilder
import io.micrometer.core.instrument.Tag
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.core.http.HttpClient
import io.vertx.core.http.HttpClientOptions
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import linea.jsonrpc.TestingJsonRpcServer
import linea.kotlin.decodeHex
import lineth.vertx.vertxTestOptions
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcError
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import java.net.ServerSocket
import java.net.URI
import java.net.URL
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class VertxHttpJsonRpcClientTest {
  private lateinit var vertx: Vertx
  private lateinit var httpClient: HttpClient
  private lateinit var client: VertxHttpJsonRpcClient
  private lateinit var wiremock: WireMockServer
  private val path = "/api/v1?appKey=1234"
  private lateinit var meterRegistry: SimpleMeterRegistry
  private lateinit var metricsFacade: MetricsFacade
  private val clientOptions =
    HttpClientOptions()
      .setKeepAlive(true)
      .setProtocolVersion(HttpVersion.HTTP_1_1)
  private lateinit var endpoint: URL
  private lateinit var testingServer: TestingJsonRpcServer
  private lateinit var testingServerEndpoint: URL

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx(vertxTestOptions)
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    endpoint = URI(wiremock.baseUrl() + path).toURL()
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(registry = meterRegistry)
    testingServer = TestingJsonRpcServer(vertx = vertx, recordRequestsResponses = true)
    listOf("engine_getPayloadV1", "engine_newPayloadV1", "engine_forkchoiceUpdatedV1", "eth_blockNumber")
      .forEach { method -> testingServer.handle(method) { "OK" } }
    testingServerEndpoint = testingServer.httpEndpoint
    httpClient = vertx.createHttpClient(clientOptions, PoolOptions().setHttp1MaxSize(2))
    client = VertxHttpJsonRpcClient(
      httpClient,
      endpoint,
      metricsFacade,
    )
  }

  @AfterEach
  fun tearDown() {
    httpClient.close().get()
    testingServer.stopHttpServer().get()
    wiremock.stop()
    vertx.close().get()
  }

  @Test
  fun makesRequest_makesCorrectJsonRpcRequest() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", null))
    val params =
      listOf(
        "superUser",
        JsonObject()
          .put("name", "Alice")
          .put("email", "alice@wonderland.io")
          .put("address", "0xaabbccdd".decodeHex()),
      )
    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "addUser", params)).get()

    val expectedJsonBody =
      """{
        |"jsonrpc": "2.0",
        |"id": 1,
        |"method": "addUser",
        |"params": [
        | "superUser", {
        |   "name":"Alice",
        |   "email":"alice@wonderland.io",
        |   "address":"0xaabbccdd"
        |   }
        | ]
        |}"""
        .trimMargin()

    wiremock.verify(
      RequestPatternBuilder.newRequestPattern()
        .withPort(wiremock.port())
        .withUrl(path)
        .withHeader("content-type", EqualToPattern("application/json"))
        .withRequestBody(
          EqualToJsonPattern(
            expectedJsonBody, /*ignoreArrayOrder*/
            false, /*ignoreExtraElements*/
            false,
          ),
        ),
    )
  }

  @Test
  fun makesRequest_success_result_is_null() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", null))

    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "eth_blockNumber", emptyList())).get()
      .also { response ->
        assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse("1", null)))
      }
  }

  @Test
  fun makesRequest_success_result_is_number() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", 3))

    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumber", emptyList())).get()
      .also { response ->
        assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse("1", 3)))
      }
  }

  @Test
  fun makesRequest_success_result_is_string() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", "0x1234"))

    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumber", emptyList())).get()
      .also { response ->
        assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse("1", "0x1234")))
      }
  }

  @Test
  fun makesRequest_success_result_is_Object() {
    replyRequestWith(
      JsonObject()
        .put("jsonrpc", "2.0")
        .put("id", "1")
        .put("result", JsonObject().put("odd", 23).put("even", 10)),
    )

    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList()))
      .get()
      .also { response ->
        val expectedJsonNode = JsonObject("""{"odd":23,"even":10}""")
        assertThat(response)
          .isEqualTo(Ok(JsonRpcSuccessResponse("1", expectedJsonNode)))
      }

    client.makeRequest(
      request = JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList()),
      resultMapper = ::toPrimitiveOrJacksonJsonNode,
    )
      .get()
      .also { response ->
        val expectedJsonNode = objectMapper.readTree("""{"odd":23,"even":10}""")
        assertThat(response)
          .isEqualTo(Ok(JsonRpcSuccessResponse("1", expectedJsonNode)))
      }
  }

  @Test
  fun makesRequest_success_result_is_array() {
    replyRequestWith(
      statusCode = 200,
      """{
        |"jsonrpc": "2.0",
        |"id": "1",
        |"result": ["a", 2, "c", 4]
        |}
      """.trimMargin(),
    )

    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList()))
      .get()
      .also { response ->
        val expectedJsonNode = JsonArray("""["a", 2, "c", 4]""")
        assertThat(response)
          .isEqualTo(Ok(JsonRpcSuccessResponse("1", expectedJsonNode)))
      }

    client.makeRequest(
      request = JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList()),
      resultMapper = ::toPrimitiveOrJacksonJsonNode,
    )
      .get()
      .also { response ->
        val expectedJsonNode = objectMapper.readTree("""["a", 2, "c", 4]""")
        assertThat(response)
          .isEqualTo(Ok(JsonRpcSuccessResponse("1", expectedJsonNode)))
      }
  }

  @Test
  fun makesRequest_successWithMapper() {
    replyRequestWith(
      JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", "some_random_value"),
    )
    val resultMapper = { value: Any? -> (value as String).uppercase() }

    client
      .makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList()), resultMapper)
      .get()
      .also { response ->
        assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse("1", "SOME_RANDOM_VALUE")))
      }
  }

  @Test
  fun makesRequest_Error() {
    replyRequestWith(
      JsonObject()
        .put("jsonrpc", "2.0")
        .put("id", "1")
        .put(
          "error",
          JsonObject()
            .put("code", -32602)
            .put("message", "Invalid params")
            .put("data", JsonObject().put("k", "v")),
        ),
    )
    val response =
      client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList())).get()

    assertThat(response)
      .isEqualTo(
        Err(
          JsonRpcErrorResponse(
            "1",
            JsonRpcError(-32602, "Invalid params", mapOf("k" to "v")),
          ),
        ),
      )
  }

  @Test
  fun makesRequest_ParseErrorNoId() {
    replyRequestWith(
      JsonObject()
        .put("jsonrpc", "2.0")
        .put("error", JsonObject().put("code", -32602).put("message", "Parse Error")),
    )
    val response = client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList())).get()

    assertThat(response)
      .isEqualTo(Err(JsonRpcErrorResponse(null, JsonRpcError(-32602, "Parse Error"))))
  }

  @Test
  fun makesRequest_malFormattedJsonResponse() {
    replyRequestWith(
      JsonObject().put("jsonrpc", "2.0").put("id", "1").put("nonsense", "some_random_value"),
    )

    assertThatThrownBy {
      client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList())).get()
    }
      .isInstanceOf(ExecutionException::class.java)
      .hasMessage(
        "java.lang.IllegalArgumentException: Invalid JSON-RPC response without result or error",
      )
  }

  @Test
  fun makesRequest_connectionFailure() {
    val log: Logger = mock()
    // Use localhost with a guaranteed-closed port for an immediate "connection refused" failure,
    // rather than a non-routable hostname whose DNS resolution can take several seconds.
    val closedPort = ServerSocket(0).also { it.close() }.localPort
    val endpoint = URI("http://localhost:$closedPort/api/v1?appKey=1234").toURL()
    httpClient.close().get()
    httpClient = vertx.createHttpClient(clientOptions)
    client = VertxHttpJsonRpcClient(
      httpClient,
      endpoint,
      metricsFacade,
      log = log,
    )

    val request = JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList())
    assertThatThrownBy { client.makeRequest(request).get() }
      .isInstanceOf(ExecutionException::class.java)
      .hasMessageContaining("localhost")

    verify(log).log(
      eq(Level.DEBUG),
      eq("<--> {} {} failed with error={}"),
      eq(endpoint),
      eq(JsonObject.mapFrom(request).encode()),
      any<String>(),
      any<Throwable>(),
    )
  }

  @Test
  fun makesRequest_502_response() {
    replyRequestWith(500, "Internal server error\n 2nd line of response to be ignored")
    val log: Logger = mock()
    httpClient.close().get()
    httpClient = vertx.createHttpClient(clientOptions)
    client = VertxHttpJsonRpcClient(
      httpClient,
      endpoint,
      metricsFacade,
      log = log,
    )

    val request = JsonRpcRequestListParams("2.0", 1, "randomNumbers", emptyList())
    assertThatThrownBy { client.makeRequest(request).get() }
      .isInstanceOf(ExecutionException::class.java)
      .hasMessageContaining("HTTP errorCode=500, message=Server Error")

    verify(log).log(
      eq(Level.DEBUG),
      eq("<--> {} {} failed with error={}"),
      eq(endpoint),
      eq(JsonObject.mapFrom(request).encode()),
      any<String>(),
      any<Throwable>(),
    )
  }

  @Test
  fun makesRequest_measuresRequest() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", 3))

    val requestsFutures = (1..100).map {
      client.makeRequest(JsonRpcRequestListParams("2.0", 1, "randomNumber", emptyList()))
    }
    Future.all(requestsFutures).get()

    val timer =
      meterRegistry.timer(
        "jsonrpc.request",
        listOf(Tag.of("method", "randomNumber"), Tag.of("endpoint", "localhost")),
      )
    assertThat(timer).isNotNull

    await()
      .atMost(1, TimeUnit.MINUTES)
      .untilAsserted {
        assertThat(timer.count()).isEqualTo(requestsFutures.size.toLong())
      }
  }

  @Test
  fun makesRequest_addsHeadersFromProviderEvaluatedPerRequest() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", null))
    val invocations = AtomicInteger(0)
    client = clientWithHooks(
      requestHeadersProvider = { mapOf("Authorization" to "Bearer token-${invocations.incrementAndGet()}") },
    )

    repeat(3) {
      client.makeRequest(JsonRpcRequestListParams("2.0", it, "eth_blockNumber", emptyList())).get()
    }

    val authorizationHeaders =
      wiremock.findAll(postRequestedFor(urlEqualTo(path))).map { it.getHeader("Authorization") }
    assertThat(authorizationHeaders)
      .containsExactlyInAnyOrder("Bearer token-1", "Bearer token-2", "Bearer token-3")
    wiremock.verify(
      3,
      postRequestedFor(urlEqualTo(path)).withHeader("Content-Type", EqualToPattern("application/json")),
    )
  }

  @Test
  fun makesRequest_failsWithoutSendingWhenHeadersProviderThrows() {
    replyRequestWith(JsonObject().put("jsonrpc", "2.0").put("id", "1").put("result", null))
    client = clientWithHooks(requestHeadersProvider = { throw IllegalStateException("jwt secret unavailable") })

    assertThatThrownBy {
      client.makeRequest(JsonRpcRequestListParams("2.0", 1, "eth_blockNumber", emptyList())).get()
    }
      .isInstanceOf(ExecutionException::class.java)
      .cause()
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessage("jwt secret unavailable")
    wiremock.verify(0, postRequestedFor(urlEqualTo(path)))
  }

  @Test
  fun makesRequest_appliesPerMethodTimeoutAndKeepsClientUsable() {
    testingServer.delayResponses("engine_getPayloadV1", 2.seconds)
    testingServer.delayResponses("engine_newPayloadV1", 300.milliseconds)
    client = clientWithHooks(
      endpoint = testingServerEndpoint,
      requestTimeoutProvider = { method -> if (method == "engine_getPayloadV1") 200.milliseconds else null },
    )

    assertThatThrownBy {
      client.makeRequest(JsonRpcRequestListParams("2.0", 1, "engine_getPayloadV1", emptyList())).get()
    }
      .isInstanceOf(ExecutionException::class.java)
      .cause()
      .isInstanceOf(TimeoutException::class.java)
      .hasMessageContaining("method=engine_getPayloadV1")
      .hasMessageContaining("timed out after 200ms")

    client.makeRequest(JsonRpcRequestListParams("2.0", 2, "engine_newPayloadV1", emptyList())).get()
      .also { response -> assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse(2, "OK"))) }
  }

  /**
   * Vert.x cancels the idle timeout once the response headers arrive, so it never bounds the streaming of the body. The
   * idle timeout is generous because a cold request on a loaded CI runner can take over a second to receive headers.
   */
  @Test
  fun makesRequest_overallTimeoutTripsOnSlowlyStreamedResponseThatIdleTimeoutTolerates() {
    testingServer.streamResponses("eth_blockNumber", chunkInterval = 40.milliseconds)
    val idleTimeoutMillis = 10_000L
    val idleOnlyClient = clientWithHooks(endpoint = testingServerEndpoint, requestTimeout = idleTimeoutMillis)
    val overallDeadlineClient = clientWithHooks(
      endpoint = testingServerEndpoint,
      requestTimeout = idleTimeoutMillis,
      requestTimeoutProvider = { 500.milliseconds },
    )

    idleOnlyClient.makeRequest(JsonRpcRequestListParams("2.0", 1, "eth_blockNumber", emptyList())).get()
      .also { response -> assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse(1, "OK"))) }

    assertThatThrownBy {
      overallDeadlineClient.makeRequest(JsonRpcRequestListParams("2.0", 2, "eth_blockNumber", emptyList())).get()
    }
      .isInstanceOf(ExecutionException::class.java)
      .cause()
      .isInstanceOf(TimeoutException::class.java)
      .hasMessageContaining("method=eth_blockNumber")
  }

  @Test
  fun makesRequest_overallTimeoutCoversWaitingForPooledConnectionAndRequestIsNeverSent() {
    testingServer.delayResponses("engine_newPayloadV1", 1.seconds)
    httpClient.close().get()
    httpClient = vertx.createHttpClient(clientOptions, PoolOptions().setHttp1MaxSize(1))
    client = clientWithHooks(
      endpoint = testingServerEndpoint,
      requestTimeoutProvider = { method -> if (method == "engine_forkchoiceUpdatedV1") 200.milliseconds else null },
    )

    val occupyingRequest =
      client.makeRequest(JsonRpcRequestListParams("2.0", 1, "engine_newPayloadV1", emptyList()))
    assertThatThrownBy {
      client.makeRequest(JsonRpcRequestListParams("2.0", 2, "engine_forkchoiceUpdatedV1", emptyList())).get()
    }
      .isInstanceOf(ExecutionException::class.java)
      .cause()
      .isInstanceOf(TimeoutException::class.java)
      .hasMessageContaining("method=engine_forkchoiceUpdatedV1")
    occupyingRequest.get()

    client.makeRequest(JsonRpcRequestListParams("2.0", 3, "engine_newPayloadV1", emptyList())).get()
    assertThat(testingServer.recordedRequests().map { it.first.method })
      .containsExactly("engine_newPayloadV1", "engine_newPayloadV1")
  }

  @Test
  fun makesRequest_withoutTimeoutProviderWaitsForSlowResponse() {
    testingServer.delayResponses("engine_newPayloadV1", 500.milliseconds)
    client = clientWithHooks(endpoint = testingServerEndpoint)

    client.makeRequest(JsonRpcRequestListParams("2.0", 1, "engine_newPayloadV1", emptyList())).get()
      .also { response -> assertThat(response).isEqualTo(Ok(JsonRpcSuccessResponse(1, "OK"))) }
  }

  private fun clientWithHooks(
    endpoint: URL = this.endpoint,
    requestTimeout: Long? = null,
    requestHeadersProvider: () -> Map<String, String> = { emptyMap() },
    requestTimeoutProvider: (String) -> Duration? = { null },
  ): VertxHttpJsonRpcClient =
    VertxHttpJsonRpcClient(
      httpClient,
      endpoint,
      metricsFacade,
      requestTimeout = requestTimeout,
      requestHeadersProvider = requestHeadersProvider,
      requestTimeoutProvider = requestTimeoutProvider,
    )

  private fun replyRequestWith(jsonRpcResponse: JsonObject) {
    wiremock.stubFor(
      post(path)
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(jsonRpcResponse.toString()),
        ),
    )
  }

  private fun replyRequestWith(statusCode: Int, body: String?) {
    wiremock.stubFor(
      post(path)
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          status(statusCode)
            .withHeader("Content-type", "text/plain")
            .apply { if (body != null) withBody(body) },
        ),
    )
  }
}
