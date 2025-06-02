package linea.http.vertx

import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger

interface VertxRequestLogger {
  fun logRequest(request: HttpRequest<Buffer>)
  fun logResponse(
    request: HttpRequest<Buffer>,
    response: HttpResponse<Buffer>? = null,
    failureCause: Throwable? = null,
  )
}

class VertxRestRequestLogger(
  private val log: Logger,
  private val logFormatter: VertxHttpLoggingFormatter,
  private val requestResponseLogLevel: Level = Level.TRACE,
  private val failuresLogLevel: Level = Level.DEBUG,
) : VertxRequestLogger {
  constructor(
    log: Logger,
    responseLogMaxSize: UInt? = null,
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG,
  ) : this(
    log = log,
    logFormatter = VertxRestLoggingFormatter(responseLogMaxSize = responseLogMaxSize),
    requestResponseLogLevel = requestResponseLogLevel,
    failuresLogLevel = failuresLogLevel,
  )

  private fun logRequest(request: HttpRequest<Buffer>, logLevel: Level = requestResponseLogLevel) {
    if (log.isEnabled(logLevel)) {
      log.log(logLevel, logFormatter.toLogString(request))
    }
  }

  override fun logRequest(request: HttpRequest<Buffer>) {
    logRequest(request, requestResponseLogLevel)
  }

  override fun logResponse(
    request: HttpRequest<Buffer>,
    response: HttpResponse<Buffer>?,
    failureCause: Throwable?,
  ) {
    val isError = response?.statusCode()?.let(::isNotSuccessStatusCode) ?: true
    val logLevel = if (isError) failuresLogLevel else requestResponseLogLevel
    if (isError && log.level != requestResponseLogLevel) {
      // in case of error, log the request that originated the error
      // to help replicate and debug later
      logRequest(request, logLevel)
    }

    if (log.isEnabled(logLevel)) {
      log.log(logLevel, logFormatter.toLogString(request, response, failureCause))
    }
  }

  private fun isNotSuccessStatusCode(statusCode: Int): Boolean {
    return statusCode !in 200..299
  }
}
