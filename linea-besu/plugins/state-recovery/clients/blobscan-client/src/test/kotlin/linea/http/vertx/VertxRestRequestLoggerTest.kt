package linea.http.vertx

import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import linea.kotlin.encodeHex
import nl.altindag.log.LogCaptor
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import kotlin.random.Random

class VertxRestRequestLoggerTest {
  private class FakeLogFormatter() : VertxHttpLoggingFormatter {
    override fun toLogString(request: HttpRequest<Buffer>): String {
      return "request-log-string"
    }

    override fun toLogString(
      request: HttpRequest<Buffer>,
      response: HttpResponse<Buffer>?,
      failureCause: Throwable?,
    ): String {
      return "response-log-string"
    }
  }

  fun setUpLogger(
    // we need to use a different logger name for each test
    loggerName: String = "request-logger-test-" + Random.nextBytes(8).encodeHex(),
  ): Pair<VertxRestRequestLogger, LogCaptor> {
    val logCaptor: LogCaptor = LogCaptor.forName(loggerName)
    return VertxRestRequestLogger(
      log = LogManager.getLogger(loggerName),
      logFormatter = FakeLogFormatter(),
      requestResponseLogLevel = Level.TRACE,
      failuresLogLevel = Level.DEBUG,
    ) to logCaptor
  }

  @Test
  fun `should not log request when level is disabled`() {
    setUpLogger().also { (requestLogger, logCaptor) ->
      logCaptor.setLogLevelToInfo()
      requestLogger.logRequest(request = mock())
      assertThat(logCaptor.logs).isEmpty()
    }
  }

  @Test
  fun `should log request when level is enabled`() {
    setUpLogger().also { (requestLogger, logCaptor) ->
      logCaptor.setLogLevelToTrace()
      requestLogger.logRequest(request = mock())
      assertThat(logCaptor.traceLogs).containsExactly("request-log-string")
    }
  }

  @Test
  fun `should not log response when level is disabled even if response is an error`() {
    setUpLogger().also { (requestLogger, logCaptor) ->
      logCaptor.setLogLevelToInfo()
      requestLogger.logResponse(request = mock(), httpResponse(statusCode = 500))
      assertThat(logCaptor.logs).isEmpty()
    }
  }

  @Test
  fun `should log response when level is enabled`() {
    setUpLogger().also { (requestLogger, logCaptor) ->
      logCaptor.setLogLevelToTrace()
      requestLogger.logResponse(request = mock(), httpResponse(statusCode = 200))
      assertThat(logCaptor.traceLogs).containsExactly("response-log-string")
    }
  }

  @Test
  fun `should log response and request on response error when failuresLogLevel is enabled`() {
    setUpLogger().also { (requestLogger, logCaptor) ->
      logCaptor.setLogLevelToDebug()
      requestLogger.logResponse(request = mock(), httpResponse(statusCode = 500))
      assertThat(logCaptor.debugLogs).containsExactly("request-log-string", "response-log-string")
    }
  }
}
