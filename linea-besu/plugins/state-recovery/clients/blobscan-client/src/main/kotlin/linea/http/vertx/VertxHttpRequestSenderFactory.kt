package linea.http.vertx

import io.vertx.core.Vertx
import linea.domain.RetryConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import kotlin.time.Duration

object VertxHttpRequestSenderFactory {
  internal fun createWithBaseSender(
    vertx: Vertx,
    requestRetryConfig: RetryConfig? = null,
    rateLimitBackoffDelay: Duration? = null,
    retryableErrorCodes: Set<Int> = setOf(429, 503, 504),
    logFormatter: VertxHttpLoggingFormatter,
    baseRequestSender: VertxHttpRequestSender,
  ): VertxHttpRequestSender {
    val rateLimitedSender = rateLimitBackoffDelay
      ?.let {
        StaticVertxHttpRequestRateLimiter(
          vertx = vertx,
          requestSender = baseRequestSender,
          rateLimitBackoffDelay = rateLimitBackoffDelay,
          requestLogFormatter = logFormatter,
        )
      } ?: baseRequestSender
    val sender = requestRetryConfig
      ?.let {
        VertxRequestRetrier(
          vertx = vertx,
          requestSender = rateLimitedSender,
          requestRetryConfig = requestRetryConfig,
          retryableErrorCodes = retryableErrorCodes,
        )
      } ?: rateLimitedSender

    return sender
  }

  fun create(
    vertx: Vertx,
    requestRetryConfig: RetryConfig? = null,
    rateLimitBackoffDelay: Duration? = null,
    logger: Logger,
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG,
    retryableErrorCodes: Set<Int> = setOf(429, 503, 504),
    logFormatter: VertxHttpLoggingFormatter,
  ): VertxHttpRequestSender {
    val requestLogger = VertxRestRequestLogger(
      log = logger,
      requestResponseLogLevel = requestResponseLogLevel,
      failuresLogLevel = failuresLogLevel,
      logFormatter = logFormatter,
    )
    return createWithBaseSender(
      vertx = vertx,
      requestRetryConfig = requestRetryConfig,
      rateLimitBackoffDelay = rateLimitBackoffDelay,
      retryableErrorCodes = retryableErrorCodes,
      logFormatter = logFormatter,
      baseRequestSender = SimpleVertxHttpRequestSender(requestLogger),
    )
  }
}
