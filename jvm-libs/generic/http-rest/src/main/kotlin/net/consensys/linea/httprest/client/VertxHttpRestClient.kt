package net.consensys.linea.httprest.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpResponse
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class VertxHttpRestClient(
  private val webClientOptions: WebClientOptions,
  private val vertx: Vertx,
) : HttpRestClient {
  private var webClient = WebClient.create(vertx, webClientOptions)
  private val log: Logger = LogManager.getLogger(this.javaClass)

  override fun get(
    path: String,
    params: List<Pair<String, String>>,
    resultMapper: (Any?) -> Any?,
  ): SafeFuture<Result<Any?, ErrorResponse<RestErrorType>>> {
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
          log.debug("Received response with status code: ${response.statusCode()}")
          Future.succeededFuture(Ok(resultMapper(response)))
        } else {
          log.error(
            "Something went wrong: " +
              "endpoint=${URI(
                "http",
                null,
                webClientOptions.defaultHost,
                webClientOptions.defaultPort,
                path,
                null,
                null,
              ).toURL()}, " +
              "statusMessage=${response.statusMessage()}",
          )
          val errorType = RestErrorType.fromStatusCode(response.statusCode())
          Future.succeededFuture(Err(ErrorResponse(errorType, response.statusMessage())))
        }
      }
      .onFailure { err ->
        Err(ErrorResponse(RestErrorType.UNKNOWN, err.message ?: "Unknown error"))
      }
      .toSafeFuture()
  }

  override fun post(
    path: String,
    buffer: Buffer,
    resultMapper: (Any?) -> Any?,
  ): SafeFuture<Result<Any?, ErrorResponse<RestErrorType>>> {
    return webClient
      .post(webClientOptions.defaultPort, webClientOptions.defaultHost, path)
      .putHeader("Content-Type", "application/json")
      .sendBuffer(buffer)
      .flatMap { httpResponse: HttpResponse<Buffer> ->
        if (isSuccessStatusCode(httpResponse.statusCode())) {
          log.debug("Received response with status code: ${httpResponse.statusCode()}")
          Future.succeededFuture(Ok(resultMapper(httpResponse)))
        } else {
          log.error(
            "Something went wrong: " +
              "endpoint=${URI(
                "http",
                null,
                webClientOptions.defaultHost,
                webClientOptions.defaultPort,
                path,
                null,
                null,
              ).toURL()}, " +
              "statusMessage=${httpResponse.statusMessage()}",
          )
          val errorType = RestErrorType.fromStatusCode(httpResponse.statusCode())
          Future.succeededFuture(Err(ErrorResponse(errorType, httpResponse.statusMessage())))
        }
      }
      .onFailure { err ->
        Err(ErrorResponse(RestErrorType.UNKNOWN, err.message ?: "Unknown error"))
      }
      .toSafeFuture()
  }

  private fun isSuccessStatusCode(statusCode: Int): Boolean {
    return statusCode >= 200 && statusCode < 300
  }
}
