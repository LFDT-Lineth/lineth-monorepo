package net.consensys.linea.jsonrpc.client

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.contains
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.buffer.Buffer
import io.vertx.core.http.HttpClient
import io.vertx.core.http.HttpClientRequest
import io.vertx.core.http.HttpClientResponse
import io.vertx.core.http.HttpMethod
import io.vertx.core.http.RequestOptions
import net.consensys.linea.async.toCompletableFuture
import net.consensys.linea.async.toVertxFuture
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
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import kotlin.time.Duration

/**
 * @param requestTimeout Vert.x connect/idle timeout in milliseconds applied to every request. It is not an overall
 * deadline: a server that keeps streaming bytes slowly will not trip it.
 * @param requestHeadersProvider evaluated once per request; the returned headers are added to that request only.
 * Useful for credentials that must be regenerated per call, e.g. Engine API JWTs.
 * @param requestTimeoutProvider returns the overall deadline for a request of the given JSON-RPC method, covering
 * connection acquisition, sending and reading the full response. When it elapses the HTTP request is reset and the
 * returned future fails with [TimeoutException]. Returning null applies no overall deadline.
 */
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
  private val requestHeadersProvider: () -> Map<String, String> = { emptyMap() },
  private val requestTimeoutProvider: (method: String) -> Duration? = { null },
) : JsonRpcClient {
  private val baseRequestOptions = RequestOptions().apply {
    setMethod(HttpMethod.POST)
    setAbsoluteURI(endpoint)
    requestTimeout?.let { setTimeout(it) }
  }

  private fun buildRequestOptions(): RequestOptions {
    val headers = requestHeadersProvider()
    if (headers.isEmpty()) return baseRequestOptions
    return RequestOptions(baseRequestOptions).apply {
      headers.forEach { (name, value) -> putHeader(name, value) }
    }
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
    val (requestOptions, overallTimeout) =
      runCatching { buildRequestOptions() to requestTimeoutProvider(request.method) }
        .getOrElse { error ->
          return Future.failedFuture<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>(error)
            .onFailure { th -> logRequestFailure(json, th) }
        }
    val inFlightRequest = InFlightHttpRequest()

    return httpClient.request(requestOptions).flatMap { httpClientRequest ->
      inFlightRequest.attach(httpClientRequest)?.let { cancellationCause ->
        return@flatMap Future.failedFuture(cancellationCause)
      }
      httpClientRequest.putHeader("Content-Type", "application/json")
      logRequest(json)

      val requestFuture =
        httpClientRequest.send(json).flatMap { response: HttpClientResponse ->
          if (isSuccessStatusCode(response.statusCode())) {
            handleResponse(json, response, resultMapper)
          } else {
            response.body().flatMap { bodyBuffer ->
              logResponse(
                isError = true,
                response = response,
                requestBody = json,
                responseBody = bodyBuffer.toString().lines().firstOrNull() ?: "",
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

      metricsFacade.createTimer(
        category = metricsCategory,
        name = "request",
        description = "Time of Upstream API JsonRpc Requests",
        tags = listOf(
          Tag("endpoint", endpoint.host),
          Tag("method", request.method),
        ),
      )
        .captureTime(requestFuture.toCompletableFuture())
        .toVertxFuture()
    }
      .withOverallTimeout(request.method, overallTimeout, inFlightRequest)
      .onFailure { th -> logRequestFailure(json, th) }
  }

  private fun <T> Future<T>.withOverallTimeout(
    method: String,
    timeout: Duration?,
    inFlightRequest: InFlightHttpRequest,
  ): Future<T> {
    if (timeout == null) return this
    val responseFuture = this
    return responseFuture
      .timeout(timeout.inWholeMilliseconds, TimeUnit.MILLISECONDS)
      .recover { error ->
        if (error is TimeoutException && !responseFuture.isComplete) {
          val timeoutError = TimeoutException("JSON-RPC request method=$method to $endpoint timed out after $timeout")
          inFlightRequest.cancel(timeoutError)
          Future.failedFuture(timeoutError)
        } else {
          Future.failedFuture(error)
        }
      }
  }

  /**
   * Tracks the HTTP request backing a JSON-RPC call so an overall timeout can reset it, including when the timeout
   * elapses before a connection has been acquired from the pool.
   */
  private class InFlightHttpRequest {
    private var request: HttpClientRequest? = null
    private var cancellationCause: Throwable? = null

    @Synchronized
    fun attach(httpClientRequest: HttpClientRequest): Throwable? {
      cancellationCause?.let {
        httpClientRequest.reset(0, it)
        return it
      }
      request = httpClientRequest
      return null
    }

    @Synchronized
    fun cancel(cause: Throwable) {
      cancellationCause = cause
      request?.reset(0, cause)
    }
  }

  private fun handleResponse(
    requestBody: String,
    httpResponse: HttpClientResponse,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    var isError = false
    var responseBody = ""
    return httpResponse
      .body()
      .flatMap { bodyBuffer: Buffer ->
        responseBody = bodyBuffer.toString()
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
          Future.succeededFuture<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>(response)
        } catch (e: Throwable) {
          isError = true
          when (e) {
            is IllegalArgumentException -> Future.failedFuture(e)
            else -> Future.failedFuture(
              IllegalArgumentException(
                "Error parsing JSON-RPC response: message=${e.message}",
                e,
              ),
            )
          }
        }
      }
      .andThen { asyncResult ->
        logResponse(isError, httpResponse, requestBody, responseBody, asyncResult.cause())
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
