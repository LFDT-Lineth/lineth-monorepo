package net.consensys.linea.jsonrpc.client

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.contains
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.Promise
import io.vertx.core.buffer.Buffer
import io.vertx.core.http.HttpClient
import io.vertx.core.http.HttpClientResponse
import io.vertx.core.http.HttpMethod
import io.vertx.core.http.RequestOptions
import net.consensys.linea.async.toCompletableFuture
import net.consensys.linea.jsonrpc.JsonRpcError
import net.consensys.linea.jsonrpc.JsonRpcErrorException
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestData
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.metrics.MetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.net.URL

class VertxHttpJsonRpcClient(
  private val httpClient: HttpClient,
  private val endpoint: URL,
  private val metricsFacade: MetricsFacade,
  private val requestParamsObjectMapper: ObjectMapper = objectMapper,
  private val responseObjectMapper: ObjectMapper = objectMapper,
  private val log: Logger = LogManager.getLogger(VertxHttpJsonRpcClient::class.java),
  private val requestResponseLogLevel: Level = Level.TRACE,
  private val requestTimeout: Long? = null,
  private val failuresLogLevel: Level = Level.DEBUG,
  private val metricsCategory: MetricsCategory = object : MetricsCategory {
    override val name: String = "jsonrpc"
  },
) : JsonRpcClient {
  private val requestOptions = RequestOptions().apply {
    setMethod(HttpMethod.POST)
    setAbsoluteURI(endpoint)
    requestTimeout?.let { setTimeout(it) }
  }

  private fun serializeRequest(request: JsonRpcRequest): String {
    return requestEnvelopeObjectMapper.writeValueAsString(
      JsonRpcRequestData(
        jsonrpc = request.jsonrpc,
        id = request.id,
        method = request.method,
        params = requestParamsObjectMapper.valueToTree(request.params),
      ),
    )
  }

  override fun makeRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val json = serializeRequest(request)

    // Bridge via a context-free Promise so the returned future is observable from any thread
    // without requiring Vertx context dispatch (context.execute). In Vertx 5, futures that carry
    // an event-loop context dispatch their completion listeners via context.execute(), which is
    // unreliable when the observer is on a non-event-loop thread (e.g. a test thread or
    // AsyncRetryer worker). Promise.promise() produces a context-free PromiseImpl whose
    // completeInternal always uses signalComplete — a direct call with no scheduler hop.
    val bridge = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()

    httpClient.request(requestOptions).flatMap { httpClientRequest ->
      httpClientRequest.putHeader("Content-Type", "application/json")
      logRequest(json)

      httpClientRequest.send(json).flatMap { response: HttpClientResponse ->
        if (isSuccessStatusCode(response.statusCode())) {
          handleResponse(json, response, resultMapper)
        } else {
          // Don't chain on response.body() here: its future carries the connection's event-loop
          // context, which may differ from the request future's context when using a connection
          // pool. That mismatch triggers context.execute() (path 2) on Linux/epoll, which can
          // be delayed past the caller's timeout. Discard the body via resume() instead so the
          // connection can be reused, and return a context-free failed future immediately.
          response.resume()
          logResponse(
            isError = true,
            response = response,
            requestBody = json,
            responseBody = "",
          )
          Future.failedFuture(
            JsonRpcErrorException(
              message =
              "HTTP errorCode=${response.statusCode()}, message=${response.statusMessage()}",
              httpStatusCode = response.statusCode(),
            ),
          )
        }
      }
    }
      .onComplete { ar ->
        if (ar.failed()) logRequestFailure(json, ar.cause())
        bridge.handle(ar)
      }

    metricsFacade.createTimer(
      category = metricsCategory,
      name = "request",
      description = "Time of Upstream API JsonRpc Requests",
      tags = listOf(
        Tag("endpoint", endpoint.host),
        Tag("method", request.method),
      ),
    ).captureTime(bridge.future().toCompletableFuture())

    return bridge.future()
  }

  private fun handleResponse(
    requestBody: String,
    httpResponse: HttpClientResponse,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    return httpResponse
      .body()
      .flatMap { bodyBuffer: Buffer ->
        val responseBody = bodyBuffer.toString()
        var isError = false
        try {
          val jsonResponse = responseObjectMapper.readTree(responseBody)
          val responseId = responseObjectMapper.convertValue(jsonResponse.get("id"), Any::class.java)
          val response =
            when {
              jsonResponse.contains("result") -> {
                Ok(
                  JsonRpcSuccessResponse(
                    responseId,
                    resultMapper(jsonResponse.get("result").toPrimitiveOrJsonNode()),
                  ),
                )
              }

              jsonResponse.contains("error") -> {
                isError = true
                val errorResponse = JsonRpcErrorResponse(
                  responseId,
                  responseObjectMapper.treeToValue(jsonResponse["error"], JsonRpcError::class.java),
                )
                Err(errorResponse)
              }

              else -> throw IllegalArgumentException("Invalid JSON-RPC response without result or error")
            }
          logResponse(isError, httpResponse, requestBody, responseBody, null)
          Future.succeededFuture<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>(response)
        } catch (e: Throwable) {
          isError = true
          val cause = when (e) {
            is IllegalArgumentException -> e
            else -> IllegalArgumentException("Error parsing JSON-RPC response: message=${e.message}", e)
          }
          logResponse(isError, httpResponse, requestBody, responseBody, cause)
          Future.failedFuture(cause)
        }
      }
  }

  private fun logRequest(jsonBody: String, level: Level = requestResponseLogLevel) {
    log.log(level, "--> {} {}", endpoint, jsonBody)
  }

  private fun logResponse(
    isError: Boolean,
    response: HttpClientResponse,
    requestBody: String,
    responseBody: String,
    failureCause: Throwable? = null,
  ) {
    val logLevel = if (isError) failuresLogLevel else requestResponseLogLevel
    if (isError && log.level != requestResponseLogLevel) {
      // in case of error, log the request that originated the error
      // to help replicate and debug later
      logRequest(requestBody, logLevel)
    }

    log.log(
      logLevel,
      "<-- {} {} {} {}",
      endpoint,
      response.statusCode(),
      responseBody,
      failureCause?.message ?: "",
    )
  }

  private fun logRequestFailure(requestBody: String, failureCause: Throwable) {
    log.log(
      failuresLogLevel,
      "<--> {} {} failed with error={}",
      endpoint,
      requestBody,
      failureCause.message,
      failureCause,
    )
  }

  private fun isSuccessStatusCode(statusCode: Int): Boolean {
    return statusCode >= 200 && statusCode < 300
  }

  companion object {
    private val requestEnvelopeObjectMapper: ObjectMapper = jacksonObjectMapper()
  }
}
