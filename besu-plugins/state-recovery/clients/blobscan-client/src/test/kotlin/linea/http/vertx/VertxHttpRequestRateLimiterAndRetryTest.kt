package linea.http.vertx

import io.vertx.core.Vertx
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import io.vertx.junit5.VertxExtension
import linea.domain.RetryConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class VertxHttpRequestRateLimiterAndRetryTest {
  private val rateLimitBackoffDelay = 10.milliseconds
  private lateinit var baseReqSender: FakeRequestSender
  private lateinit var reqSender: VertxHttpRequestSender

  @BeforeEach
  fun setUp(vertx: Vertx) {
    baseReqSender = FakeRequestSender()
    reqSender = VertxHttpRequestSenderFactory.createWithBaseSender(
      vertx = vertx,
      baseRequestSender = baseReqSender,
      retryableErrorCodes = setOf(429),
      requestRetryConfig = RetryConfig(
        maxRetries = 5u,
        backoffDelay = 10.milliseconds,
        timeout = rateLimitBackoffDelay * 30
      ),
      rateLimitBackoffDelay = rateLimitBackoffDelay,
      logFormatter = VertxRestLoggingFormatter(responseLogMaxSize = 1000u)
    )
    // Warn: this does not work in io.github.hakky54:logcaptor
    // don't have time to dig into it now. disabling it for now
    // configureLoggers(
    //   rootLevel = Level.DEBUG,
    //   FakeRequestSender::class.java.name to Level.DEBUG
    // )
  }

  @Test
  fun `should rate limit requests even if retries are fired at higher rate`(vertx: Vertx) {
    val client = WebClient.create(vertx, WebClientOptions())

    baseReqSender.responseStatusCode = 429

    val futures = (1..10).map { index ->
      reqSender.makeRequest(client.get("users/$index"))
    }

    runCatching { SafeFuture.collectAll(futures.stream()).get() }

    // shall retry each request once at least
    assertThat(baseReqSender.requestsTimesDiffs.size).isGreaterThanOrEqualTo(10)

    // lenient assertion to avoid flakiness in the tests due to clock drift/precision
    baseReqSender.requestsTimesDiffs
      .drop(1)
      .forEach { delay ->
        assertThat(delay).isGreaterThanOrEqualTo(rateLimitBackoffDelay)
      }
  }
}
