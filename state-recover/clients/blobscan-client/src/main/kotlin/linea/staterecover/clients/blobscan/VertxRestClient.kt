package linea.staterecover.clients.blobscan

import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import io.vertx.ext.web.client.WebClient
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

// TODO: move to a common module
data class RestResponse<T>(
  val statusCode: Int,
  val body: T?
)

interface RestClient<Response> {
  fun get(path: String): SafeFuture<RestResponse<Response>>
  // add remaining verbs as we need them
}

class VertxRestClient<Response>(
  private val vertx: Vertx,
  private val webClient: WebClient,
  private val responseParser: (Buffer) -> Response,
  private val retryableErrorCodes: Set<Int> = DEFAULT_RETRY_HTTP_CODES,
  private val requestRetryConfig: RequestRetryConfig,
  private val asyncRetryer: AsyncRetryer<HttpResponse<Buffer>> = AsyncRetryer.retryer(
    backoffDelay = requestRetryConfig.backoffDelay,
    maxRetries = requestRetryConfig.maxRetries?.toInt(),
    timeout = requestRetryConfig.timeout,
    vertx = vertx
  ),
  private val requestHeaders: Map<String, String> = mapOf("Accept" to "application/json"),
  private val log: Logger = LogManager.getLogger(VertxRestClient::class.java),
  private val requestResponseLogLevel: Level = Level.TRACE,
  private val failuresLogLevel: Level = Level.DEBUG,
  private val responseLogMaxSize: UInt? = null
) : RestClient<Response> {
  private fun makeRequestWithRetry(
    request: HttpRequest<Buffer>
  ): SafeFuture<HttpResponse<Buffer>> {
    return asyncRetryer
      .retry(
        stopRetriesPredicate = { response: HttpResponse<Buffer> ->
          response.statusCode() !in retryableErrorCodes
        }
      ) {
        logRequest(request)
        request.send().toSafeFuture()
          .thenPeek { response -> logResponse(request, response) }
          .whenException { error -> logResponse(request = request, failureCause = error) }
      }
  }

  override fun get(path: String): SafeFuture<RestResponse<Response>> {
    return makeRequestWithRetry(
      webClient
        .get(path)
        .apply { requestHeaders.forEach(::putHeader) }
    )
      .thenApply { response ->
        val parsedResponse = response.body()?.let(responseParser)
        RestResponse(response.statusCode(), parsedResponse)
      }
  }

  private fun logRequest(request: HttpRequest<Buffer>, level: Level = requestResponseLogLevel) {
    log.log(level, "--> {} {}", request.method(), request.uri())
  }

  private fun logResponse(
    request: HttpRequest<Buffer>,
    response: HttpResponse<Buffer>? = null,
    failureCause: Throwable? = null
  ) {
    val isError = response?.statusCode()?.let(::isNotSuccessStatusCode) ?: true
    val logLevel = if (isError) failuresLogLevel else requestResponseLogLevel
    if (isError && log.level != requestResponseLogLevel) {
      // in case of error, log the request that originated the error
      // to help replicate and debug later
      logRequest(request, logLevel)
    }

    val responseToLog = response?.bodyAsString()?.let { bodyStr ->
      if (responseLogMaxSize != null) {
        bodyStr.take(responseLogMaxSize.toInt()) + "..." + "(contentLength=${response.getHeader("Content-Length")})"
      } else {
        bodyStr
      }
    }

    log.log(
      logLevel,
      "<-- {} {} {} {} {}",
      request.method(),
      request.uri(),
      response?.statusCode(),
      responseToLog,
      failureCause?.message ?: ""
    )
  }

  private fun isNotSuccessStatusCode(statusCode: Int): Boolean {
    return statusCode !in 200..299
  }

  companion object {
    val DEFAULT_RETRY_HTTP_CODES = setOf(429, 500, 503, 504)
  }
}
