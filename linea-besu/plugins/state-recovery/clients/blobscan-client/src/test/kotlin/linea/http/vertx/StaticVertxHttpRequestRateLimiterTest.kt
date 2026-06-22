package linea.http.vertx

import io.vertx.core.Vertx
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import io.vertx.junit5.VertxExtension
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class StaticVertxHttpRequestRateLimiterTest {
  private val rateLimitBackoffDelay = 100.milliseconds
  private lateinit var reqSender: FakeRequestSender
  private lateinit var rateLimiter: StaticVertxHttpRequestRateLimiter

  @BeforeEach
  fun setUp(vertx: Vertx) {
    reqSender = FakeRequestSender()
    rateLimiter = StaticVertxHttpRequestRateLimiter(
      vertx = vertx,
      requestSender = reqSender,
      rateLimitBackoffDelay = rateLimitBackoffDelay,
      requestLogFormatter = VertxRestLoggingFormatter(),
    )
  }

  @Test
  fun `should rate limit requests without waiting for response`(vertx: Vertx) {
    val client = WebClient.create(vertx, WebClientOptions())

    repeat(20) {
      rateLimiter.makeRequest(client.get("users/1"))
    }

    Thread.sleep(rateLimitBackoffDelay.inWholeMilliseconds * 5)
    assertThat(reqSender.requestsTimesDiffs.size).isGreaterThan(4)

    reqSender.requestsTimesDiffs.drop(1).forEach { reqTimeDelay ->
      assertThat(reqTimeDelay).isGreaterThanOrEqualTo(rateLimitBackoffDelay - 1.milliseconds)
    }
  }
}
