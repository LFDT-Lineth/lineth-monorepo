package linea.http.vertx

import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.LinkedList
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.TimeSource

/**
 * Rate limits the requests sent to a VertxHttpRequestSender by adding a static delay between each request.
 */
class StaticVertxHttpRequestRateLimiter(
  private val vertx: Vertx,
  private val requestSender: VertxHttpRequestSender,
  private val rateLimitBackoffDelay: Duration,
  private val queueProcessInterval: Duration = rateLimitBackoffDelay
    .div(5)
    .coerceAtLeast(1.milliseconds),
  private val requestLogFormatter: VertxHttpLoggingFormatter,
  private val logger: Logger = LogManager.getLogger(StaticVertxHttpRequestRateLimiter::class.java)
) : VertxHttpRequestSender {
  private data class RequestAndFutureResponse(
    val request: HttpRequest<Buffer>,
    val future: SafeFuture<HttpResponse<Buffer>>
  )

  private val rateLimitPerSecond = 1.seconds.div(rateLimitBackoffDelay).toInt()
  private val requestQueue = LinkedList<RequestAndFutureResponse>()
  private val monotonicClock = TimeSource.Monotonic
  private var lastRequestFiredTime = monotonicClock.markNow().minus(rateLimitBackoffDelay)

  init {
    vertx.setPeriodic(queueProcessInterval.inWholeMilliseconds) { processQueue() }
  }

  @Synchronized
  fun processQueue() {
    if (requestQueue.isEmpty()) {
      return
    }

    val elapsedTimeSinceLastRequest = lastRequestFiredTime.elapsedNow()
    if (elapsedTimeSinceLastRequest < rateLimitBackoffDelay) {
      logger.trace(
        "waiting to make request: rateLimit={} req/s queueSize={} " +
          "timeToNextRequest={} nextRequest={}",
        rateLimitPerSecond,
        requestQueue.size,
        rateLimitBackoffDelay - elapsedTimeSinceLastRequest,
        requestLogFormatter.toLogString(requestQueue.peek().request)
      )
      return
    }
    val requestAndFutureResponse = requestQueue.poll()

    fireRequest(requestAndFutureResponse)
  }

  private fun fireRequest(requestAndFutureResponse: RequestAndFutureResponse) {
    synchronized(this) {
      lastRequestFiredTime = monotonicClock.markNow()
    }

    requestSender.makeRequest(requestAndFutureResponse.request)
      .handle { response, error ->
        if (error != null) {
          requestAndFutureResponse.future.completeExceptionally(error)
        } else {
          requestAndFutureResponse.future.complete(response)
        }
      }
  }

  @Synchronized
  private fun canMakeRequest(): Boolean {
    return lastRequestFiredTime.elapsedNow() >= rateLimitBackoffDelay
  }

  @Synchronized
  override fun makeRequest(request: HttpRequest<Buffer>): SafeFuture<HttpResponse<Buffer>> {
    val req = RequestAndFutureResponse(request, SafeFuture())

    if (canMakeRequest()) {
      fireRequest(req)
    } else {
      logger.debug(
        "queueing request: rateLimit={} req/s queueSize={} timeToNextRequest={} request={}",
        rateLimitPerSecond,
        requestQueue.size,
        rateLimitBackoffDelay - lastRequestFiredTime.elapsedNow(),
        requestLogFormatter.toLogString(request)
      )
      synchronized(this) {
        requestQueue.add(req)
      }
    }

    return req.future
  }
}
