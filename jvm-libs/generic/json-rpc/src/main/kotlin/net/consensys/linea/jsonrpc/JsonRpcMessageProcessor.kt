package net.consensys.linea.jsonrpc

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.registerKotlinModule
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.map
import com.github.michaelbull.result.merge
import com.github.michaelbull.result.recover
import com.github.michaelbull.result.unwrap
import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.Counter
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Timer
import io.vertx.core.AsyncResult
import io.vertx.core.CompositeFuture
import io.vertx.core.Future
import io.vertx.core.Promise
import io.vertx.core.json.DecodeException
import io.vertx.core.json.Json
import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import io.vertx.core.json.jackson.DatabindCodec
import io.vertx.core.json.jackson.VertxModule
import io.vertx.ext.auth.User
import net.consensys.linea.metrics.micrometer.DynamicTagTimerCapture
import net.consensys.linea.metrics.micrometer.SimpleTimerCapture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

typealias JsonRpcMessageHandler = (user: User?, messageJsonStr: String) -> Future<String>

typealias JsonRpcRequestParser =
  (json: Any) -> Result<Pair<JsonRpcRequest, JsonObject>, JsonRpcErrorResponse>

typealias JsonRpcRequestHandler =
  (user: User?, jsonRpcRequest: JsonRpcRequest, requestJson: JsonObject) -> Future<
    Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>

fun Result<*, *>.isSuccess(): Boolean = this is Ok

private data class RequestContext(
  val id: Any,
  val method: String,
  val result: Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>
)

/**
 * Class for handling RPC Messages (which can contain 1 or more RPC request). It parses the raw
 * String message into JsonArray, then each entry parsed to JsonRpcRequest class
 */
class JsonRpcMessageProcessor(
  private val requestsHandler: JsonRpcRequestHandler,
  private val meterRegistry: MeterRegistry,
  private val requestParser: JsonRpcRequestParser = Companion::parseRequest,
  private val log: Logger = LogManager.getLogger(JsonRpcMessageProcessor::class.java),
  private val responseResultObjectMapper: ObjectMapper = jacksonObjectMapper().registerModules(VertxModule()),
  private val rpcEnvelopeObjectMapper: ObjectMapper = jacksonObjectMapper()
) : JsonRpcMessageHandler {
  init {
    DatabindCodec.mapper().registerKotlinModule()
  }

  private val counterBuilder = Counter.builder("jsonrpc.counter")
  override fun invoke(user: User?, messageJsonStr: String): Future<String> =
    handleMessage(user, messageJsonStr)

  private fun handleMessage(user: User?, requestJsonStr: String): Future<String> {
    val wholeRequestTimer =
      Timer.builder("jsonrpc.processing.whole")
        .description(
          "Processing of JSON-RPC message: Deserialization + Business Logic + Serialization"
        )
    val timerSample = Timer.start(Clock.SYSTEM)
    val json: Any =
      when (val result = decodeMessage(requestJsonStr)) {
        is Ok -> result.value
        is Err -> {
          return Future.succeededFuture(Json.encode(result.error))
        }
      }
    log.trace(json)
    val isBulkRequest: Boolean = json is JsonArray
    val jsonArray = if (isBulkRequest) json as JsonArray else JsonArray().add(json)
    val parsingResults: List<Result<Pair<JsonRpcRequest, JsonObject>, JsonRpcErrorResponse>> =
      jsonArray.map(::measureRequestParsing)

    // all or nothing: if any of the requests has a parsing error, return before execution
    parsingResults.forEach {
      when (it) {
        is Err -> return Future.succeededFuture(Json.encode(it.error))
        is Ok -> Unit
      }
    }

    var allSuccessful = true
    val executionFutures: List<Future<RequestContext>> =
      parsingResults.map { result ->
        // all success results at this state
        val (rpc: JsonRpcRequest, jsonObj: JsonObject) = result.unwrap()
        handleRequest(user, rpc, jsonObj)
          .map { RequestContext(rpc.id, rpc.method, it) }
          .recover { error: Throwable ->
            log.error(
              "Failed processing JSON-RPC request. error: {}",
              // NullPointerException have null message, at least log the class name
              error.message ?: error::class.java,
              error
            )
            Future.succeededFuture(
              RequestContext(
                rpc.id,
                rpc.method,
                Err(JsonRpcErrorResponse.internalError(rpc.id, null))
              )
            )
          }
      }

    executionFutures.forEach { resultFuture ->
      resultFuture.onComplete { ar ->
        if (ar.failed() || !ar.result().result.isSuccess()) {
          allSuccessful = false
        }
      }
    }

    val serializedResponses =
      executionFutures.map { future -> future.map(this::encodeAndMeasureResponse) }

    return Future.all(serializedResponses)
      .transform { ar: AsyncResult<CompositeFuture> ->
        val methodTag =
          if (isBulkRequest) {
            "bulk_request"
          } else {
            parsingResults.first()
              .unwrap().first.method
          }
        wholeRequestTimer.tag("method", methodTag)

        val responses = ar.result().list<String>()
        val finalResponseJsonStr =
          if (responses.size == 1) {
            responses.first()
          } else {
            SimpleTimerCapture<String>(
              meterRegistry,
              "jsonrpc.serialization.response.bulk"
            )
              .setDescription("Time of bulk json response serialization")
              .captureTime { responses.joinToString(",", "[", "]") }
          }

        timerSample.stop(wholeRequestTimer.register(meterRegistry))
        logResponse(allSuccessful, finalResponseJsonStr, requestJsonStr)
        Future.succeededFuture(finalResponseJsonStr)
      }
  }

  private fun measureRequestParsing(
    json: Any
  ): Result<Pair<JsonRpcRequest, JsonObject>, JsonRpcErrorResponse> {
    return DynamicTagTimerCapture<Result<Pair<JsonRpcRequest, JsonObject>, JsonRpcErrorResponse>>(
      meterRegistry,
      "jsonrpc.serialization.request"
    )
      .setTagKey("method")
      .setDescription("json-rpc method parsing")
      .setTagValueExtractor { parsingResult: Result<Pair<JsonRpcRequest, JsonObject>, JsonRpcErrorResponse> ->
        parsingResult.map { it.first.method }.recover { "METHOD_PARSE_ERROR" }.value
      }
      .setTagValueExtractorOnError { "METHOD_PARSE_ERROR" }
      .captureTime { requestParser(json) }
  }

  private fun encodeAndMeasureResponse(requestContext: RequestContext): String {
    return SimpleTimerCapture<String>(meterRegistry, "jsonrpc.serialization.response")
      .setDescription("Time of json response serialization")
      .setTag("method", requestContext.method)
      .captureTime {
        val result = requestContext.result.map { successResponse ->
          val resultJsonNode = responseResultObjectMapper.valueToTree<JsonNode>(successResponse.result)
          successResponse.copy(result = resultJsonNode)
        }
        rpcEnvelopeObjectMapper.writeValueAsString(result.merge())
      }
  }

  private fun handleRequest(
    user: User?,
    jsonRpcRequest: JsonRpcRequest,
    requestJson: JsonObject
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    return SimpleTimerCapture<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>(
      meterRegistry,
      "jsonrpc.processing.logic"
    )
      .setTag("method", jsonRpcRequest.method)
      .setDescription("Processing of a particular JRPC method's logic without SerDes")
      .captureTime(callRequestHandlerAndCatchError(user, jsonRpcRequest, requestJson))
      .onComplete { result: AsyncResult<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> ->
        val success = (result.succeeded() && result.result() is Ok)
        counterBuilder.tag("success", success.toString())
        counterBuilder.tag("method", jsonRpcRequest.method)
        counterBuilder.register(meterRegistry).increment()
      }
  }

  private fun callRequestHandlerAndCatchError(
    user: User?,
    jsonRpcRequest: JsonRpcRequest,
    requestJson: JsonObject
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val promise = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()

    try {
      requestsHandler(user, jsonRpcRequest, requestJson)
        .onSuccess(promise::complete)
        .onFailure(promise::fail)
    } catch (e: Exception) {
      promise.fail(e)
    }

    return promise.future()
  }

  private fun logResponse(isSuccessResponse: Boolean, response: String, request: String) {
    // if is success => log response in trace mode
    // if is failure =>
    //   if TRACE is disabled => log request and response in DEBUG mode,
    //   if TRACE is enabled => log response in DEBUG (skip request logging, it was already logged)
    if (isSuccessResponse) {
      log.trace(response)
    } else {
      if (!log.isTraceEnabled) {
        log.debug(request)
      }
      log.debug(response)
    }
  }

  companion object {
    // init {
    //   DatabindCodec.mapper().enable(SerializationFeature.INDENT_OUTPUT)
    // }
    fun parseRequest(json: Any): Result<Pair<JsonRpcRequest, JsonObject>, JsonRpcErrorResponse> {
      try {
        json as JsonObject
        val request: JsonRpcRequest = when {
          json.getValue("method") !is String -> return Err(JsonRpcErrorResponse.invalidRequest())
          json.getValue("params") is JsonObject -> json.mapTo(JsonRpcRequestMapParams::class.java)
          json.getValue("params") is JsonArray -> json.mapTo(JsonRpcRequestListParams::class.java)
          else -> return Err(JsonRpcErrorResponse.invalidRequest())
        }
        if (!request.isValid) {
          return Err(JsonRpcErrorResponse.invalidRequest())
        }
        return Ok(Pair(request, json))
      } catch (e: ClassCastException) {
        return Err(JsonRpcErrorResponse.invalidRequest())
      } catch (e: DecodeException) {
        return Err(JsonRpcErrorResponse.invalidRequest())
      }
    }

    fun decodeMessage(msg: String): Result<Any, JsonRpcErrorResponse> {
      return try {
        when (val jsonObject = Json.decodeValue(msg)) {
          is JsonArray -> {
            if (jsonObject.isEmpty) {
              Err(JsonRpcErrorResponse.invalidRequest())
            } else {
              Ok(jsonObject)
            }
          }

          is JsonObject -> Ok(jsonObject)
          else -> Err(JsonRpcErrorResponse.parseError())
        }
      } catch (e: DecodeException) {
        Err(JsonRpcErrorResponse.parseError())
      }
    }
  }
}
