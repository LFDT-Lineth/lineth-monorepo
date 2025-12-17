package linea.http.vertx

import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import net.consensys.linea.async.toSafeFuture
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface VertxHttpRequestSender {
  fun makeRequest(request: HttpRequest<Buffer>): SafeFuture<HttpResponse<Buffer>> = request.send().toSafeFuture()
}

/**
 * A [VertxHttpRequestSender] that just sends the request without any additional logic.
 * Handy to avoid creating anonymous classes.
 */
class SimpleVertxHttpRequestSender(
  private val requestLogger: VertxRequestLogger,
) : VertxHttpRequestSender {
  override fun makeRequest(request: HttpRequest<Buffer>): SafeFuture<HttpResponse<Buffer>> {
    requestLogger.logRequest(request)
    return request.send().toSafeFuture()
      .thenPeek { response -> requestLogger.logResponse(request, response) }
      .whenException { error -> requestLogger.logResponse(request = request, failureCause = error) }
  }
}
