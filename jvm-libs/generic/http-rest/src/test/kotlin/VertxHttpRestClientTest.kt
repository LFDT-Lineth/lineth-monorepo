import com.fasterxml.jackson.databind.node.JsonNodeFactory
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.get
import com.github.michaelbull.result.map
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.client.WireMock.get
import com.github.tomakehurst.wiremock.client.WireMock.notFound
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.serverError
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.core.json.JsonObject
import io.vertx.ext.web.client.WebClientOptions
import io.vertx.ext.web.client.impl.HttpResponseImpl
import io.vertx.junit5.VertxExtension
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.httprest.client.RestErrorType
import net.consensys.linea.httprest.client.VertxHttpRestClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith

@ExtendWith(VertxExtension::class)
class VertxHttpRestClientTest {

  private lateinit var client: VertxHttpRestClient
  private lateinit var wiremock: WireMockServer
  private val path = "/api/v1"
  private lateinit var meterRegistry: SimpleMeterRegistry

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    val clientOptions =
      WebClientOptions()
        .setMaxPoolSize(10)
        .setDefaultHost("localhost")
        .setDefaultPort(wiremock.port())
        .setLogActivity(true)

    meterRegistry = SimpleMeterRegistry()
    client = VertxHttpRestClient(clientOptions, vertx)
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun post_returnPlainText() {
    wiremock.stubFor(
      post("$path/text")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .willReturn(
          ok().withHeader("Content-type", "text/plain; charset=utf-8\n").withBody("hello"),
        ),
    )

    val response =
      client.post("$path/text", Buffer.buffer("")).get().map {
        it as HttpResponseImpl<*>
        it.body().toString()
      }
    assertThat(response).isEqualTo(Ok("hello"))
  }

  @Test
  fun get_returnJson() {
    val node = JsonNodeFactory.instance.objectNode()
    node.put("status", "correct")
    wiremock.stubFor(
      get("$path/json")
        .willReturn(ok().withHeader("Content-type", "application/json").withJsonBody(node)),
    )

    val response =
      client.get("$path/json").get().map {
        it as HttpResponseImpl<*>
        it.bodyAsJsonObject()
      }
    assertThat(response).isEqualTo(Ok(JsonObject(node.toString())))
  }

  @Test
  fun get_returnPlainText() {
    wiremock.stubFor(
      get("$path/text")
        .willReturn(
          ok().withHeader("Content-type", "text/plain; charset=utf-8\n").withBody("text"),
        ),
    )

    val response =
      client.get("$path/text").get().map {
        it as HttpResponseImpl<*>
        it.bodyAsString()
      }
    assertThat(response).isEqualTo(Ok("text"))
  }

  @Test
  fun get_multipleParameters() {
    val params = listOf("1" to "one", "2" to "two", "3" to "three", "4" to "four")
    wiremock.stubFor(
      get("$path/text?1=one&2=two&3=three&4=four")
        .willReturn(
          ok().withHeader("Content-type", "text/plain; charset=utf-8\n").withBody("text"),
        ),
    )

    val response =
      client.get("$path/text", params).get().map {
        it as HttpResponseImpl<*>
        it.bodyAsString()
      }
    assertThat(response).isEqualTo(Ok("text"))
  }

  @Test
  fun get_serverErrorResponse() {
    wiremock.stubFor(
      get("$path/serverError")
        .willReturn(serverError().withHeader("Content-type", "text/plain; charset=utf-8\n")),
    )

    val response = client.get("$path/serverError").get()
    assertThat(response)
      .isEqualTo(Err(ErrorResponse(RestErrorType.INTERNAL_SERVER_ERROR, "Server Error")))
  }

  @Test
  fun post_notFound() {
    wiremock.stubFor(
      post("$path/notFound")
        .willReturn(notFound().withHeader("Content-type", "text/plain; charset=utf-8\n")),
    )

    val response = client.post("$path/notFound", Buffer.buffer("")).get()
    assertThat(response).isEqualTo(Err(ErrorResponse(RestErrorType.NOT_FOUND, "Not Found")))
  }

  @Test
  fun errorWithCustomStatusMessage() {
    wiremock.stubFor(
      post("$path/notFound")
        .willReturn(
          notFound()
            .withHeader("Content-type", "text/plain; charset=utf-8\n")
            .withStatusMessage("Param not found"),
        ),
    )

    val response = client.post("$path/notFound", Buffer.buffer("")).get()
    assertThat(response).isEqualTo(Err(ErrorResponse(RestErrorType.NOT_FOUND, "Param not found")))
  }
}
