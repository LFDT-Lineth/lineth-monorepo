package linea.jsonrpc

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.junit5.VertxExtension
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes

@ExtendWith(VertxExtension::class)
class TestingJsonRpcServerTest {
  private lateinit var jsonRpcServer: TestingJsonRpcServer
  private lateinit var client: JsonRpcV2Client

  @BeforeEach
  fun beforeEach(vertx: io.vertx.core.Vertx) {
    jsonRpcServer = TestingJsonRpcServer(
      vertx = vertx,
      recordRequestsResponses = true
    )
    val rpcClientFactory = VertxHttpJsonRpcClientFactory(
      vertx = vertx,
      meterRegistry = SimpleMeterRegistry()
    )
    client = rpcClientFactory.createJsonRpcV2Client(
      endpoints = listOf(URI.create("http://localhost:${jsonRpcServer.boundPort}")),
      retryConfig = RequestRetryConfig(
        maxRetries = 10u,
        backoffDelay = 10.milliseconds,
        timeout = 2.minutes
      ),
      shallRetryRequestsClientBasePredicate = {
        false
      } // disable retry
    )
  }

  @Test
  fun `when no method handler is defined returns method not found`() {
    assertThatThrownBy {
      client.makeRequest(
        method = "not_existing_method",
        params = mapOf("k1" to "v1", "k2" to 100),
        resultMapper = { it },
        shallRetryRequestPredicate = { false }
      ).get()
    }.hasCauseInstanceOf(JsonRpcErrorResponseException::class.java)
      .hasMessageContaining("Method not found")

    // check recorded request
    jsonRpcServer.recordedRequests().also {
      assertThat(it).hasSize(1)
      val (request, responseFuture) = it[0]
      assertThat(request.method).isEqualTo("not_existing_method")
      assertThat(request.params).isEqualTo(mapOf("k1" to "v1", "k2" to 100))
      assertThat(responseFuture.get()).isEqualTo(
        Err(JsonRpcErrorResponse.methodNotFound(request.id, data = "not_existing_method"))
      )
    }
  }

  @Test
  fun `when handlers are provided shall forward to correct one`() {
    jsonRpcServer.handle("add") { request ->
      @Suppress("UNCHECKED_CAST")
      val params = request.params as List<Int>
      params.sumOf { it }
    }
    jsonRpcServer.handle("addUser") { request ->
      @Suppress("UNCHECKED_CAST")
      val params = request.params as Map<String, Any?>
      "user=${params["name"]} email=${params["email"]}"
    }
    jsonRpcServer.handle("multiply") { _ -> "not expected" }

    assertThat(
      client.makeRequest(
        method = "add",
        params = listOf(1, 2, 3),
        resultMapper = { it }
      ).get()
    )
      .isEqualTo(6)

    assertThat(
      client.makeRequest(
        method = "addUser",
        params = mapOf("name" to "John", "email" to "john@email.com"),
        resultMapper = { it }
      ).get()
    )
      .isEqualTo("user=John email=john@email.com")

    // check recorded request
    jsonRpcServer.recordedRequests().also {
      assertThat(it).hasSize(2)
      it[0].also { (request, responseFuture) ->
        assertThat(request.method).isEqualTo("add")
        assertThat(request.params).isEqualTo(listOf(1, 2, 3))
        assertThat(responseFuture.get()).isEqualTo(Ok(JsonRpcSuccessResponse(id = request.id, result = 6)))
      }
      it[1].also { (request, responseFuture) ->
        assertThat(request.method).isEqualTo("addUser")
        assertThat(request.params).isEqualTo(mapOf("name" to "John", "email" to "john@email.com"))
        assertThat(responseFuture.get())
          .isEqualTo(Ok(JsonRpcSuccessResponse(id = request.id, result = "user=John email=john@email.com")))
      }
    }
  }
}
