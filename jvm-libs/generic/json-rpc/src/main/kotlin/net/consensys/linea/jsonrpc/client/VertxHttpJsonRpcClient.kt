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

@Suppress("UNCHECKED_CAST")
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

    return httpClient.request(requestOptions).flatMap { httpClientRequest ->
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
      .onFailure { th -> logRequestFailure(json, th) }
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

  private fun logRequestFailure(
    requestBody: String,
    failureCause: Throwable,
  ) {
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
