package net.consensys.linea.jsonrpc

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Future
import io.vertx.core.json.Json
import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith

private fun <T> Future<T>.get() = this.toCompletionStage().toCompletableFuture().get()

@ExtendWith(VertxExtension::class)
class JsonRpcMessageProcessorTest {
  private lateinit var processor: JsonRpcMessageProcessor
  private lateinit var meterRegistry: SimpleMeterRegistry
  private lateinit var metricsFacade: MetricsFacade

  @BeforeEach
  fun setUp() {
    val fakeRequestHandlerAlwaysSuccess: JsonRpcRequestHandler =
      { _: User?, jsonRpcRequest: JsonRpcRequest, _: JsonObject ->
        Future.succeededFuture(Ok(JsonRpcSuccessResponse(jsonRpcRequest.id, JsonObject())))
      }
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(registry = meterRegistry)
    processor = JsonRpcMessageProcessor(fakeRequestHandlerAlwaysSuccess, metricsFacade)
  }

  @Test
  fun `handleMessage should catch exceptions and return internal error`(
    testContext: VertxTestContext,
  ) {
    val request = buildJsonRpcRequest(method = "eth_blockNumber")
    val processor = JsonRpcMessageProcessor(
      { _, _, _ -> throw RuntimeException("Something went wrong") },
      metricsFacade,
    )
    processor(null, request.toString())
      .onComplete(
        testContext.succeeding { response ->
          assertError(
            response,
            JsonObject.mapFrom(JsonRpcErrorCode.INTERNAL_ERROR.toErrorObject()),
            request,
          )
          testContext.completeNow()
        },
      )
  }

  @Test
  fun `handleMessage should return error when message can't be deserialized`(
    testContext: VertxTestContext,
  ) {
    val jsonStr = "{ bad json }"
    val future = processor(null, jsonStr)
    val expectedError = JsonObject.mapFrom(JsonRpcErrorCode.PARSE_ERROR.toErrorObject())
    future.onComplete(
      testContext.succeeding { response ->
        assertError(response, expectedError)
        testContext.completeNow()
      },
    )
  }

  @Test
  fun `handleMessage should return error when message contains invalid JSON-RPC request`(
    testContext: VertxTestContext,
  ) {
    val jsonStr = Json.encode(JsonArray().add(JsonObject()))
    val future = processor(null, jsonStr)
    future.onComplete(
      testContext.succeeding { response ->
        assertError(
          response,
          JsonObject.mapFrom(JsonRpcErrorCode.INVALID_REQUEST.toErrorObject()),
        )
        testContext.completeNow()
      },
    )
  }

  @Test
  fun `handleMessage bulk should return error when one message is invalid JSON-RPC request`(
    testContext: VertxTestContext,
  ) {
    val requests =
      listOf(
        buildJsonRpcRequest(id = 1, "eth_blockNumber"),
        JsonObject().put("invalid_key", "any value"),
        buildJsonRpcRequest(id = 2, "eth_getBlockByNumber", "latest"),
      )
    val jsonStr = Json.encode(JsonArray(requests))

    processor(null, jsonStr)
      .onComplete(
        testContext.succeeding { response ->
          assertError(
            response,
            JsonObject.mapFrom(JsonRpcErrorCode.INVALID_REQUEST.toErrorObject()),
          )
          testContext.completeNow()
        },
      )
  }

  @Test
  fun `handleMessage should execute single JSON-RPC request and return success response`(
    testContext: VertxTestContext,
  ) {
    val request = buildJsonRpcRequest(method = "eth_blockNumber")
    processor(null, request.toString())
      .onComplete(
        testContext.succeeding { response ->
          assertResult(response, JsonObject(), request)
          testContext.completeNow()
        },
      )
  }

  @Test
  fun `handleMessage should execute bulk JSON-RPC requests and return success response`(
    testContext: VertxTestContext,
  ) {
    val requests =
      listOf(
        buildJsonRpcRequest(id = 1, "read_value"),
        buildJsonRpcRequest(id = 2, "update_value", "latest"),
      )
    val jsonStr = Json.encode(JsonArray(requests))

    processor(null, jsonStr)
      .onComplete(
        testContext.succeeding { response ->
          val responses = JsonArray(response)
          assertResult(responses.getJsonObject(0).toString(), JsonObject(), requests[0])
          assertResult(responses.getJsonObject(1).toString(), JsonObject(), requests[1])
          testContext.completeNow()
        },
      )
  }

  private val fakeRequestHandlerWithSomeFailures: JsonRpcRequestHandler =
    { _: User?, jsonRpcRequest: JsonRpcRequest, _: JsonObject ->
      when (jsonRpcRequest.id as Int) {
        -12 ->
          Future.succeededFuture(
            Err(
              JsonRpcErrorResponse(
                jsonRpcRequest.id,
                JsonRpcError.invalidMethodParameter("Required argument missing"),
              ),
            ),
          )

        -100 -> Future.failedFuture(Exception("An internal bug"))
        else ->
          Future.succeededFuture(Ok(JsonRpcSuccessResponse(jsonRpcRequest.id, JsonObject())))
      }
    }

  @Test
  fun `handleMessage should return error when any of the requests fail`(
    testContext: VertxTestContext,
  ) {
    val requests =
      listOf(
        buildJsonRpcRequest(id = 1, "read_value"),
        buildJsonRpcRequest(id = -12, "update_value", "latest"),
        buildJsonRpcRequest(id = 3, "update_value", "latest"),
        buildJsonRpcRequest(id = -100, "update_value", "latest"),
        buildJsonRpcRequest(id = 4, "update_value", "latest"),
        buildJsonRpcRequest(id = 5, "read_value"),
        buildJsonRpcRequest(id = 6, "update_value", "latest"),
      )

    val jsonStr = Json.encode(JsonArray(requests))

    processor = JsonRpcMessageProcessor(fakeRequestHandlerWithSomeFailures, metricsFacade)

    processor(null, jsonStr)
      .onComplete(
        testContext.succeeding { response ->
          val responses = JsonArray(response)
          assertResult(responses.getJsonObject(0).toString(), JsonObject(), requests[0])
          assertError(
            responses.getJsonObject(1).toString(),
            JsonObject.mapFrom(
              JsonRpcError.invalidMethodParameter("Required argument missing"),
            ),
            requests[1],
          )
          assertResult(responses.getJsonObject(2).toString(), JsonObject(), requests[2])
          assertError(
            responses.getJsonObject(3).toString(),
            JsonObject.mapFrom(JsonRpcError.internalError()),
            requests[3],
          )
          testContext.completeNow()
        },
      )
  }

  @Test
  fun `handleMessage should issue metrics correctly`() {
    val request1 = buildJsonRpcRequest(id = 1, "read_value")
    val request2 = buildJsonRpcRequest(id = 1, "update_value", "latest")

    val request3 = buildJsonRpcRequest(id = -12, "read_value")
    val request4 = buildJsonRpcRequest(id = -12, "update_value", "latest")

    val bulkRequests1 =
      listOf(
        buildJsonRpcRequest(id = 1, "read_value"),
        buildJsonRpcRequest(id = 2, "update_value", "latest"),
      )
    val bulkRequests2 =
      listOf(
        buildJsonRpcRequest(id = 1, "read_value"),
        buildJsonRpcRequest(id = -12, "update_value", "latest"),
        buildJsonRpcRequest(id = 3, "update_value", "latest"),
        buildJsonRpcRequest(id = -100, "update_value", "latest"),
        buildJsonRpcRequest(id = 4, "update_value", "latest"),
        buildJsonRpcRequest(id = 5, "read_value"),
        buildJsonRpcRequest(id = 6, "update_value", "latest"),
      )
    val singleAsBulk = listOf(buildJsonRpcRequest(id = 10, "read_value"))

    processor = JsonRpcMessageProcessor(fakeRequestHandlerWithSomeFailures, metricsFacade)
    processor(null, Json.encode(request1)).get()
    processor(null, Json.encode(request2)).get()
    processor(null, Json.encode(request3)).get()
    processor(null, Json.encode(request4)).get()
    processor(null, Json.encode(JsonArray(bulkRequests1))).get()
    processor(null, Json.encode(JsonArray(bulkRequests2))).get()
    processor(null, Json.encode(JsonArray(singleAsBulk))).get()

    // println(meterRegistry.metersAsString)
    // assert correct metrics:
    assertThat(meterRegistry.timer("jsonrpc.processing.whole", "method", "read_value").count())
      .isEqualTo(2)
    assertThat(meterRegistry.timer("jsonrpc.processing.whole", "method", "update_value").count())
      .isEqualTo(2)
    assertThat(meterRegistry.timer("jsonrpc.processing.whole", "method", "bulk_request").count())
      .isEqualTo(3)

    assertThat(
      meterRegistry
        .counter("jsonrpc.counter", "method", "read_value", "success", "true")
        .count(),
    )
      .isEqualTo(5.0)
    assertThat(
      meterRegistry
        .counter("jsonrpc.counter", "method", "read_value", "success", "false")
        .count(),
    )
      .isEqualTo(1.0)

    assertThat(
      meterRegistry
        .counter("jsonrpc.counter", "method", "update_value", "success", "true")
        .count(),
    )
      .isEqualTo(5.0)
    assertThat(
      meterRegistry
        .counter("jsonrpc.counter", "method", "update_value", "success", "false")
        .count(),
    )
      .isEqualTo(3.0)

    assertThat(meterRegistry.timer("jsonrpc.processing.logic", "method", "read_value").count())
      .isEqualTo(6)

    assertThat(meterRegistry.timer("jsonrpc.processing.logic", "method", "update_value").count())
      .isEqualTo(8)

    assertThat(meterRegistry.timer("jsonrpc.serialization.request", "method", "read_value").count())
      .isEqualTo(6)
    assertThat(
      meterRegistry.timer("jsonrpc.serialization.request", "method", "update_value").count(),
    )
      .isEqualTo(8)
    assertThat(
      meterRegistry.timer("jsonrpc.serialization.response", "method", "read_value").count(),
    )
      .isEqualTo(6)
    assertThat(
      meterRegistry.timer("jsonrpc.serialization.response", "method", "update_value").count(),
    )
      .isEqualTo(8)

    assertThat(meterRegistry.timer("jsonrpc.serialization.response.bulk").count()).isEqualTo(2)
  }

  private fun buildJsonRpcRequest(
    id: Int = 1,
    method: String = "eth_blockNumber",
    vararg params: Any,
  ): JsonObject {
    return JsonObject()
      .put("jsonrpc", "2.0")
      .put("id", id)
      .put("method", method)
      .put("params", JsonArray.of(*params))
  }

  private fun assertResult(
    responseStr: String,
    expectedResult: JsonObject,
    originalRequest: JsonObject,
  ) {
    val response = JsonObject(responseStr)
    assertThat(response.getValue("id")).isEqualTo(originalRequest.getValue("id"))
    assertThat(response.getValue("result")).isEqualTo(expectedResult)
    assertThat(response.getValue("error")).isNull()
  }

  private fun assertError(
    responseStr: String,
    expectedError: Any,
    originalRequest: JsonObject? = null,
  ) {
    val response = JsonObject(responseStr)
    originalRequest?.let {
      assertThat(response.getValue("id")).isEqualTo(originalRequest.getValue("id"))
    }
    assertThat(response.getValue("error")).isEqualTo(expectedError)
    assertThat(response.getValue("result")).isNull()
  }
}
