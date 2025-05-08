package linea.http.vertx

import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import linea.domain.RetryConfig
import net.consensys.linea.async.AsyncRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture

class VertxRequestRetrier(
  private val vertx: Vertx,
  private val requestSender: VertxHttpRequestSender,
  private val requestRetryConfig: RetryConfig,
  private val retryableErrorCodes: Set<Int> = DEFAULT_RETRY_HTTP_CODES,
  private val asyncRetryer: AsyncRetryer<HttpResponse<Buffer>> = AsyncRetryer.retryer(
    backoffDelay = requestRetryConfig.backoffDelay,
    maxRetries = requestRetryConfig.maxRetries?.toInt(),
    timeout = requestRetryConfig.timeout,
    vertx = vertx
  )
) : VertxHttpRequestSender {
  override fun makeRequest(request: HttpRequest<Buffer>): SafeFuture<HttpResponse<Buffer>> {
    return asyncRetryer
      .retry(
        stopRetriesPredicate = { response: HttpResponse<Buffer> ->
          response.statusCode() !in retryableErrorCodes
        }
      ) {
        requestSender.makeRequest(request)
      }
  }

  companion object {
    val DEFAULT_RETRY_HTTP_CODES = setOf(429, 500, 503, 504)
  }
}
