package linea.http.vertx

import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import net.consensys.linea.vertx.setDefaultsFrom
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.net.URI

class VertxRestLoggingFormatterTest {
  val request = WebClient.create(
    Vertx.vertx(),
    WebClientOptions()
      .setDefaultsFrom(URI("http://service:9876/"))
  )
    .get("/users/1?appKey=SOME_APP_KEY")

  fun formatter(includeFullUri: Boolean): VertxRestLoggingFormatter {
    return VertxRestLoggingFormatter(
      includeFullUri = includeFullUri,
      uriTransformer = { it.replace("SOME_APP_KEY", "***") },
      responseLogMaxSize = null
    )
  }

  @Test
  fun `should format request correctly`() {
    assertThat(formatter(includeFullUri = false).toLogString(request))
      .isEqualTo("--> GET /users/1?appKey=***")

    assertThat(formatter(includeFullUri = true).toLogString(request))
      .isEqualTo("--> GET http://service:9876/users/1?appKey=***")
  }

  @Test
  fun `should format response correctly`() {
    val response = httpResponse(
      statusCode = 200,
      statusMessage = "OK",
      body = Buffer.buffer("some-response-body")
    )

    assertThat(formatter(includeFullUri = false).toLogString(request, response))
      .isEqualTo("<-- GET /users/1?appKey=*** 200 some-response-body")

    assertThat(formatter(includeFullUri = true).toLogString(request, response))
      .isEqualTo("<-- GET http://service:9876/users/1?appKey=*** 200 some-response-body")
  }

  @Test
  fun `should format response correctly with errors`() {
    assertThat(formatter(includeFullUri = false).toLogString(request, null, RuntimeException("socket timeout")))
      .isEqualTo("<-- GET /users/1?appKey=*** error=socket timeout")

    assertThat(formatter(includeFullUri = true).toLogString(request, null, RuntimeException("socket timeout")))
      .isEqualTo("<-- GET http://service:9876/users/1?appKey=*** error=socket timeout")
  }
}
