package net.consensys.linea.httprest.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.core.http.PoolOptions
import io.vertx.ext.web.client.HttpResponse
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import linea.error.ErrorResponse
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class VertxHttpRestClient(
  private val webClientOptions: WebClientOptions,
  poolOptions: PoolOptions = PoolOptions(),
  vertx: Vertx,
  private val log: Logger = LogManager.getLogger(VertxHttpRestClient::class.java),
  private val requestResponseLogLevel: Level = Level.TRACE,
  private val failuresLogLevel: Level = Level.DEBUG,
  private val maskRequestBody: Boolean = requestResponseLogLevel != Level.TRACE,
) : HttpRestClient {
  private var webClient = WebClient.create(vertx, webClientOptions, poolOptions)

  override fun get(
    path: String,
    params: List<Pair<String, String>>,
    resultMapper: (Any?) -> Any?,
  ): SafeFuture<Result<Any?, ErrorResponse<RestErrorType>>> {
    val method = "GET"
    logRequest(method, path, params)

    return webClient
      .get(webClientOptions.defaultPort, webClientOptions.defaultHost, path)
      .apply {
        for (param in params) {
          addQueryParam(param.first, param.second)
        }
      }
      .send()
      .flatMap { response: HttpResponse<Buffer> ->
        if (isSuccessStatusCode(response.statusCode())) {
          logResponse(isError = false, method = method, path = path, requestBody = params, response = response)
          Future.succeededFuture(Ok(resultMapper(response)))
        } else {
          logResponse(isError = true, method = method, path = path, requestBody = params, response = response)
          val errorType = RestErrorType.fromStatusCode(response.statusCode())
          Future.succeededFuture(Err(ErrorResponse(errorType, response.statusMessage())))
        }
      }
      .recover { err ->
        logRequestFailure(method, path, params, err)
        Future.succeededFuture(Err(ErrorResponse(RestErrorType.UNKNOWN, err.message ?: "Unknown error")))
      }
      .toSafeFuture()
  }

  override fun post(
    path: String,
    buffer: Buffer,
    resultMapper: (Any?) -> Any?,
  ): SafeFuture<Result<Any?, ErrorResponse<RestErrorType>>> {
    val method = "POST"
    logRequest(method, path, buffer)

    return webClient
      .post(webClientOptions.defaultPort, webClientOptions.defaultHost, path)
      .putHeader("Content-Type", "application/json")
      .sendBuffer(buffer)
      .flatMap { httpResponse: HttpResponse<Buffer> ->
        if (isSuccessStatusCode(httpResponse.statusCode())) {
          logResponse(isError = false, method = method, path = path, requestBody = buffer, response = httpResponse)
          Future.succeededFuture(Ok(resultMapper(httpResponse)))
        } else {
          logResponse(isError = true, method = method, path = path, requestBody = buffer, response = httpResponse)
          val errorType = RestErrorType.fromStatusCode(httpResponse.statusCode())
          Future.succeededFuture(Err(ErrorResponse(errorType, httpResponse.statusMessage())))
        }
      }
      .recover { err ->
        logRequestFailure(method, path, buffer, err)
        Future.succeededFuture(Err(ErrorResponse(RestErrorType.UNKNOWN, err.message ?: "Unknown error")))
      }
      .toSafeFuture()
  }

  private fun buildEndpointUri(path: String): URI {
    return URI(
      if (webClientOptions.isSsl) "https" else "http",
      null,
      webClientOptions.defaultHost,
      webClientOptions.defaultPort,
      path,
      null,
      null,
    )
  }

  private fun logRequest(method: String, path: String, body: Any?, level: Level = requestResponseLogLevel) {
    if (!log.isEnabled(level)) return
    val renderedBody = body.toString().let {
      if (maskRequestBody) {
        "Hidden Body: size=${it.length} (toString() characters)"
      } else {
        it
      }
    }

    log.log(level, "--> {} {} {}", method, buildEndpointUri(path), renderedBody)
  }

  private fun logResponse(
    isError: Boolean,
    method: String,
    path: String,
    requestBody: Any?,
    response: HttpResponse<Buffer>,
  ) {
    val logLevel = if (isError) failuresLogLevel else requestResponseLogLevel
    if (isError && !log.isEnabled(requestResponseLogLevel)) {
      // in case of error, log the request that originated the error
      // to help replicate and debug later
      logRequest(method, path, requestBody, logLevel)
    }

    if (!log.isEnabled(logLevel)) return
    log.log(
      logLevel,
      "<-- {} {} {} {}",
      method,
      buildEndpointUri(path),
      response.statusCode(),
      response.statusMessage(),
    )
  }

  private fun logRequestFailure(method: String, path: String, requestBody: Any?, failureCause: Throwable) {
    if (!log.isEnabled(requestResponseLogLevel)) {
      // request/response tracing wasn't active, log the request that originated the error
      // to help replicate and debug later
      logRequest(method, path, requestBody, failuresLogLevel)
    }

    if (!log.isEnabled(failuresLogLevel)) return
    log.log(
      failuresLogLevel,
      "<--> {} {} failed with error={}",
      method,
      buildEndpointUri(path),
      failureCause.message,
      failureCause,
    )
  }

  private fun isSuccessStatusCode(statusCode: Int): Boolean {
    return statusCode >= 200 && statusCode < 300
  }
}
