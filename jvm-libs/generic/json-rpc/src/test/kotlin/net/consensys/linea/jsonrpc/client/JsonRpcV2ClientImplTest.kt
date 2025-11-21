package net.consensys.linea.jsonrpc.client

import build.linea.s11n.jackson.BigIntegerToHexSerializer
import build.linea.s11n.jackson.ByteArrayToHexSerializer
import build.linea.s11n.jackson.JIntegerToHexSerializer
import build.linea.s11n.jackson.ULongToHexSerializer
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.module.SimpleModule
import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.getOr
import com.github.michaelbull.result.orElseThrow
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.status
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import com.github.tomakehurst.wiremock.stubbing.Scenario.STARTED
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.junit5.VertxExtension
import linea.kotlin.decodeHex
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.javacrumbs.jsonunit.assertj.JsonAssertions.assertThatJson
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.net.ConnectException
import java.net.URI
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.ExecutionException
import java.util.function.Predicate
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class JsonRpcV2ClientImplTest {
  private lateinit var vertx: Vertx
  private lateinit var factory: VertxHttpJsonRpcClientFactory
  private lateinit var client: JsonRpcV2Client
  private lateinit var wiremock: WireMockServer
  private val path = "/api/v1?appKey=1234"
  private lateinit var meterRegistry: SimpleMeterRegistry
  private val defaultRetryConfig = retryConfig(maxRetries = 2u, timeout = 20.seconds, backoffDelay = 5.milliseconds)

  private val defaultObjectMapper = jacksonObjectMapper()
  private val objectMapperBytesAsHex = jacksonObjectMapper()
    .registerModules(
      SimpleModule().apply {
        this.addSerializer(ByteArray::class.java, ByteArrayToHexSerializer)
        this.addSerializer(ULong::class.java, ULongToHexSerializer)
      },
    )
  private val jsonRpcResultOk = """{"jsonrpc": "2.0", "id": 1, "result": "OK"}"""

  private fun retryConfig(
    maxRetries: UInt = 2u,
    timeout: Duration = 8.seconds, // bellow 2s we may have flacky tests when running whole test suite in parallel
    backoffDelay: Duration = 5.milliseconds,
  ) = RequestRetryConfig(
    maxRetries = maxRetries,
    timeout = timeout,
    backoffDelay = backoffDelay,
  )

  private fun createClientAndSetupWireMockServer(
    responseObjectMapper: ObjectMapper = defaultObjectMapper,
    requestObjectMapper: ObjectMapper = defaultObjectMapper,
    retryConfig: RequestRetryConfig = defaultRetryConfig,
    shallRetryRequestsClientBasePredicate: Predicate<Result<Any?, Throwable>> = Predicate { false },
  ): JsonRpcV2Client {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()

    val uris = listOf(URI(wiremock.baseUrl() + path))

    return createClient(
      uris = uris,
      responseObjectMapper = responseObjectMapper,
      requestObjectMapper = requestObjectMapper,
      retryConfig = retryConfig,
      shallRetryRequestsClientBasePredicate = shallRetryRequestsClientBasePredicate,
    )
  }

  private fun createClient(
    uris: List<URI>,
    responseObjectMapper: ObjectMapper = defaultObjectMapper,
    requestObjectMapper: ObjectMapper = defaultObjectMapper,
    retryConfig: RequestRetryConfig = defaultRetryConfig,
    shallRetryRequestsClientBasePredicate: Predicate<Result<Any?, Throwable>> = Predicate { false },
  ): JsonRpcV2Client {
    return factory.createJsonRpcV2Client(
      endpoints = uris,
      retryConfig = retryConfig,
      requestObjectMapper = requestObjectMapper,
      responseObjectMapper = responseObjectMapper,
      shallRetryRequestsClientBasePredicate = shallRetryRequestsClientBasePredicate,
    )
  }

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    this.vertx = vertx
    this.meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    this.factory = VertxHttpJsonRpcClientFactory(vertx, metricsFacade)
    this.client = createClientAndSetupWireMockServer()
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  private fun WireMockServer.jsonRequest(requestIndex: Int = 0): String {
    return this.serveEvents.serveEvents[requestIndex].request.bodyAsString
  }

  @Test
  fun `when request is a list of params shall serialize to json array`() {
    replyRequestWith(200, jsonRpcResultOk)

    client.makeRequest(
      method = "someMethod",
      params = listOf("superUser", "Alice"),
      resultMapper = { it },
    ).get()

    assertThatJson(wiremock.jsonRequest()).isEqualTo(
      """
      {
        "jsonrpc": "2.0",
        "id":"${'$'}{json-unit.any-number}",
        "method": "someMethod",
        "params": ["superUser", "Alice"]
      }
      """,
    )
  }

  @Test
  fun `when request is a map of params shall serialize to json object`() {
    replyRequestWith(200, jsonRpcResultOk)

    client.makeRequest(
      method = "someMethod",
      params = mapOf("superUser" to "Alice"),
      resultMapper = { it },
    ).get()

    assertThatJson(wiremock.jsonRequest()).isEqualTo(
      """
      {
        "jsonrpc": "2.0",
        "id":"${'$'}{json-unit.any-number}",
        "method": "someMethod",
        "params": {"superUser":"Alice"}
      }
      """,
    )
  }

  private data class User(
    val name: String,
    val email: String,
    val address: ByteArray,
    val value: ULong,
  )

  @Test
  fun `when request is Pojo object shall serialize to json object`() {
    replyRequestWith(200, jsonRpcResultOk)

    client.makeRequest(
      method = "someMethod",
      params = User(name = "John", email = "email@example.com", address = "0x01ffbb".decodeHex(), value = 987UL),
      resultMapper = { it },
    ).get()
    // 0x01ffbb -> "Af+7" in Base64, jackon's default encoding for ByteArray
    assertThatJson(wiremock.jsonRequest()).isEqualTo(
      """
      {
        "jsonrpc": "2.0",
        "id":"${'$'}{json-unit.any-number}",
        "method": "someMethod",
        "params": {"name":"John", "email":"email@example.com", "address":"Af+7", "value":987}
      }
      """,
    )
  }

  @Test
  fun `request params shall use defined objectMapper and not affect json-rpc envelope`() {
    val obj = User(name = "John", email = "email@example.com", address = "0x01ffbb".decodeHex(), value = 987UL)

    createClientAndSetupWireMockServer(requestObjectMapper = defaultObjectMapper).also { client ->
      replyRequestWith(200, jsonRpcResultOk)
      client.makeRequest(
        method = "someMethod",
        params = obj,
        resultMapper = { it },
      ).get()

      assertThatJson(wiremock.jsonRequest()).isEqualTo(
        """
      {
        "jsonrpc": "2.0",
        "id":"${'$'}{json-unit.any-number}",
        "method": "someMethod",
        "params": {"name":"John", "email":"email@example.com", "address":"Af+7", "value":987}
      }
      """,
      )
      wiremock.stop()
    }

    val objMapperWithNumbersAsHex = jacksonObjectMapper()
      .registerModules(
        SimpleModule().apply {
          this.addSerializer(ByteArray::class.java, ByteArrayToHexSerializer)
          this.addSerializer(ULong::class.java, ULongToHexSerializer)
          this.addSerializer(Integer::class.java, JIntegerToHexSerializer)
          this.addSerializer(BigInteger::class.java, BigIntegerToHexSerializer)
        },
      )

    createClientAndSetupWireMockServer(requestObjectMapper = objMapperWithNumbersAsHex).also { client ->
      replyRequestWith(200, jsonRpcResultOk)
      client.makeRequest(
        method = "someMethod",
        params = obj,
        resultMapper = { it },
      ).get()

      assertThatJson(wiremock.jsonRequest()).isEqualTo(
        """
      {
        "jsonrpc": "2.0",
        "id":"${'$'}{json-unit.any-number}",
        "method": "someMethod",
        "params": {"name":"John", "email":"email@example.com", "address":"0x01ffbb", "value": "0x3db"}
      }
      """,
      )
    }
  }

  @Test
  fun `when multiple requests are made, each request shall have an unique id`() {
    replyRequestWith(200, jsonRpcResultOk)
    val numberOfRequests = 20
    val requestsPromises = IntRange(1, numberOfRequests).map { index ->
      client.makeRequest(
        method = "someMethod",
        params = listOf(index),
        resultMapper = { it },
      )
    }
    SafeFuture.collectAll(requestsPromises.stream()).get()
    assertThat(wiremock.serveEvents.serveEvents.size).isEqualTo(numberOfRequests)

    val ids = wiremock.serveEvents.serveEvents.fold(mutableSetOf<String>()) { acc, event ->
      val index = JsonObject(event.request.bodyAsString).getString("id")
      acc.add(index)
      acc
    }
    assertThat(ids.size).isEqualTo(numberOfRequests)
  }

  @Test
  fun `when result is null resolves with null`() {
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": null}""")

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        assertThat(response).isNull()
      }
  }

  @Test
  fun `when result is string resolves with String`() {
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": "hello :)"}""")

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        assertThat(response).isEqualTo("hello :)")
      }
  }

  @Test
  fun `when result is Int Number resolves with Int`() {
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": 42}""")

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        assertThat(response).isEqualTo(42)
      }
  }

  @Test
  fun `when result is Long Number resolves with Long`() {
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": ${Long.MAX_VALUE}}""")

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        assertThat(response).isEqualTo(Long.MAX_VALUE)
      }
  }

  @Test
  fun `when result is Floating point Number resolves with Double`() {
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": 3.14}""")

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        assertThat(response).isEqualTo(3.14)
      }
  }

  @Test
  fun `when result is an Object, returns JsonNode`() {
    replyRequestWith(
      200,
      """{"jsonrpc": "2.0", "id": 1, "result": {"name": "Alice", "age": 23}}""",
    )

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        val expectedObj: ObjectNode =
          objectMapperBytesAsHex.readTree("""{"name": "Alice", "age": 23}""") as ObjectNode
        assertThat(response).isEqualTo(expectedObj)
      }
  }

  @Test
  fun `when result is an Array, return JsonNode`() {
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": [1, 2, 3]}""")

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = { it },
    )
      .get()
      .also { response ->
        val expectedArray: ArrayNode = objectMapperBytesAsHex.readTree("[1, 2, 3]") as ArrayNode
        assertThat(response).isEqualTo(expectedArray)
      }
  }

  @Test
  fun `when result transforms result shall return it`() {
    data class SimpleUser(val name: String, val age: Int)
    replyRequestWith(200, """{"jsonrpc": "2.0", "id": 1, "result": {"name": "Alice", "age": 23}}""")

    val expectedUser = SimpleUser("Alice", 23)
    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = {
        it as JsonNode
        SimpleUser(it.get("name").asText(), it.get("age").asInt())
      },
    )
      .get()
      .also { response ->
        assertThat(response).isEqualTo(expectedUser)
      }

    client.makeRequest(
      method = "someMethod",
      params = emptyList<Any>(),
      resultMapper = {
        it as JsonNode
        defaultObjectMapper.treeToValue(it, SimpleUser::class.java)
      },
    )
      .get()
      .also { response ->
        assertThat(response).isEqualTo(expectedUser)
      }
  }

  @Test
  fun `when it gets a json-prc error rejects with JsonRpcErrorException cause with error code and message`() {
    replyRequestWith(
      200,
      """{
        "jsonrpc": "2.0",
        "id": 1,
        "error": {
          "code": -32602,
          "message": "Invalid params",
          "data": {
            "field1": {"key1": "value1", "key2": 20, "key3": [1, 2, 3], "key4": null}
           }
          }
        }
      """.trimMargin(),
    )

    assertThat(
      client.makeRequest(
        method = "someMethod",
        params = emptyList<Any>(),
        resultMapper = { it },
      ),
    ).failsWithin(10.seconds.toJavaDuration())
      .withThrowableThat()
      .isInstanceOfSatisfying(ExecutionException::class.java) {
        assertThat(it.cause).isInstanceOf(JsonRpcErrorResponseException::class.java)
        val cause = it.cause as JsonRpcErrorResponseException
        assertThat(cause.rpcErrorCode).isEqualTo(-32602)
        assertThat(cause.rpcErrorMessage).isEqualTo("Invalid params")
        val expectedData = mapOf(
          "field1" to mapOf(
            "key1" to "value1",
            "key2" to 20,
            "key3" to listOf(1, 2, 3),
            "key4" to null,
          ),
        )
        assertThat(cause.rpcErrorData).isEqualTo(expectedData)
      }
  }

  @Test
  fun `when it gets an error propagates to shallRetryRequestPredicate and retries while is true`() {
    createClientAndSetupWireMockServer(
      retryConfig = retryConfig(maxRetries = 10u),
    ).also { client ->
      val responses = listOf(
        500 to "Internal Error",
        200 to "Invalid Json",
        200 to """{"jsonrpc": "2.0", "id": 1, "error": {"code": -32602, "message": "Invalid params"}}""",
        200 to """{"jsonrpc": "2.0", "id": 1, "result": null }""",
        200 to """{"jsonrpc": "2.0", "id": 1, "result": "some result" }""",
        200 to """{"jsonrpc": "2.0", "id": 1, "result": "expected result" }""",
      )
      replyRequestsWith(responses = responses)
      val retryPredicateCalls = mutableListOf<Result<String?, Throwable>>()

      client.makeRequest(
        method = "someMethod",
        params = emptyList<Any>(),
        shallRetryRequestPredicate = {
          retryPredicateCalls.add(it)
          it != Ok("EXPECTED RESULT")
        },
        resultMapper = {
          it as String?
          it?.uppercase()
        },
      ).get()

      assertThat(wiremock.serveEvents.serveEvents).hasSize(responses.size)
      assertThat(retryPredicateCalls).hasSize(responses.size)
      assertThatThrownBy { retryPredicateCalls[0].orElseThrow() }
        .isInstanceOfSatisfying(Exception::class.java) {
          assertThat(it.message).contains("HTTP errorCode=500, message=Server Error")
        }
      assertThatThrownBy { retryPredicateCalls[1].orElseThrow() }
        .isInstanceOfSatisfying(IllegalArgumentException::class.java) {
          assertThat(it.message).contains("Error parsing JSON-RPC response")
        }
      assertThatThrownBy { retryPredicateCalls[2].orElseThrow() }
        .isInstanceOfSatisfying(JsonRpcErrorResponseException::class.java) {
          assertThat(it.rpcErrorCode).isEqualTo(-32602)
          assertThat(it.rpcErrorMessage).isEqualTo("Invalid params")
        }
      assertThat(retryPredicateCalls[3]).isEqualTo(Ok(value = null))
      assertThat(retryPredicateCalls[4]).isEqualTo(Ok(value = "SOME RESULT"))
      assertThat(retryPredicateCalls[5]).isEqualTo(Ok(value = "EXPECTED RESULT"))
    }
  }

  @Test
  fun `when it has connection error propagates to shallRetryRequestPredicate and retries while is true`() {
    createClientAndSetupWireMockServer(
      retryConfig = retryConfig(maxRetries = 10u),
    ).also { client ->
      // stop the server to simulate connection error
      wiremock.stop()

      val retryPredicateCalls = CopyOnWriteArrayList<Result<String?, Throwable>>()

      val reqFuture = client.makeRequest(
        method = "someMethod",
        params = emptyList<Any>(),
        shallRetryRequestPredicate = {
          retryPredicateCalls.add(it)
          retryPredicateCalls.size < 2
        },
        resultMapper = { it as String? },
      )

      assertThatThrownBy { reqFuture.get() }
        .isInstanceOfSatisfying(ExecutionException::class.java) {
          assertThat(it.cause).isInstanceOfSatisfying(ConnectException::class.java) {
            assertThat(it.message).contains("Connection refused: localhost/127.0.0.1:")
          }
        }

      assertThat(retryPredicateCalls.size).isEqualTo(2)
      assertThatThrownBy { retryPredicateCalls[0].orElseThrow() }
        .isInstanceOfSatisfying(ConnectException::class.java) {
          assertThat(it.message).contains("Connection refused: localhost/127.0.0.1:")
        }
    }
  }

  @Test
  fun `when it has connection error propagates to shallRetryRequestPredicate and retries until retry config elapses`() {
    createClient(
      uris = listOf(URI.create("http://127.0.0.1:19472")),
      retryConfig = retryConfig(maxRetries = 2u, timeout = 8.seconds, backoffDelay = 5.milliseconds),
    ).also { client ->
      val retryPredicateCalls = mutableListOf<Result<String?, Throwable>>()

      val reqFuture = client.makeRequest(
        method = "someMethod",
        params = emptyList<Any>(),
        shallRetryRequestPredicate = {
          retryPredicateCalls.add(it)
          true // keep retrying
        },
        resultMapper = { it as String? },
      )

      assertThatThrownBy { reqFuture.get() }
        .isInstanceOfSatisfying(ExecutionException::class.java) {
          assertThat(it.cause).isInstanceOfSatisfying(ConnectException::class.java) {
            assertThat(it.message).contains("Connection refused: /127.0.0.1:19472")
          }
        }
      assertThat(retryPredicateCalls).hasSizeBetween(1, 3)
    }
  }

  @Test
  fun `when shared predicate is defined shall retry when any of them returns true`() {
    val baseRetryPredicateCalls = mutableListOf<Result<Any?, Throwable>>()
    val baseRetryPredicate = Predicate<Result<Any?, Throwable>> {
      baseRetryPredicateCalls.add(it)
      it as Ok
      (it.value as String).startsWith("retry_a")
    }
    createClientAndSetupWireMockServer(
      retryConfig = RequestRetryConfig(
        maxRetries = 10u,
        timeout = 5.minutes,
        backoffDelay = 1.milliseconds,
      ),
      shallRetryRequestsClientBasePredicate = baseRetryPredicate,
    ).also { client ->
      replyRequestsWith(
        listOf(
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "retry_a_0" }""",
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "retry_a_1" }""",
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "retry_a_2" }""",
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "retry_b_3" }""",
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "retry_b_4" }""",
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "retry_b_5" }""",
          200 to """{"jsonrpc": "2.0", "id": 1, "result": "some_result" }""",
        ),
      )
      val retryPredicateCalls = mutableListOf<Result<String, Throwable>>()
      val reqFuture = client.makeRequest(
        method = "someMethod",
        params = emptyList<Any>(),
        shallRetryRequestPredicate = {
          retryPredicateCalls.add(it)
          it.getOr("").startsWith("retry_b")
        },
        resultMapper = { it as String },
      )

      assertThat(reqFuture.get()).isEqualTo("some_result")
      assertThat(baseRetryPredicateCalls).hasSize(7)
      // this extra assertion is not necessary for correctness.
      // however, if it breaks raises awareness that 2nd predicate may be evaluated without need
      assertThat(retryPredicateCalls.map { it.getOr(-1) }).isEqualTo(
        listOf(
          "retry_b_3",
          "retry_b_4",
          "retry_b_5",
          "some_result",
        ),
      )
    }
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

  private fun replyRequestsWith(responses: List<Pair<Int, String?>>) {
    val (firstResponseStatus, firstResponseBody) = responses.first()
    wiremock.stubFor(
      post(path)
        .withHeader("Content-Type", containing("application/json"))
        .inScenario("retry")
        .whenScenarioStateIs(STARTED)
        .willReturn(
          status(firstResponseStatus)
            .withHeader("Content-type", "text/plain")
            .apply { if (firstResponseBody != null) withBody(firstResponseBody) },
        )
        .willSetStateTo("req_0"),
    )

    responses
      .drop(1)
      .forEachIndexed { index, (statusCode, body) ->
        wiremock.stubFor(
          post(path)
            .withHeader("Content-Type", containing("application/json"))
            .inScenario("retry")
            .whenScenarioStateIs("req_$index")
            .willReturn(
              status(statusCode)
                .withHeader("Content-type", "text/plain")
                .apply { if (body != null) withBody(body) },
            )
            .willSetStateTo("req_${index + 1}"),
        )
      }
  }
}
