package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.junit5.VertxExtension
import linea.domain.BlockNumberAndHash
import linea.kotlin.ByteArrayExt
import linea.kotlin.encodeHex
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.net.URL

@ExtendWith(VertxExtension::class)
class ShomeiClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var meterRegistry: SimpleMeterRegistry
  private lateinit var fakeShomeiServerURI: URL
  private lateinit var jsonRpcClient: JsonRpcClient

  @BeforeEach
  fun setUp(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    val rpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, metricsFacade)
    fakeShomeiServerURI = URI("http://127.0.0.1:" + wiremock.port()).toURL()
    jsonRpcClient = rpcClientFactory.create(fakeShomeiServerURI)
  }

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    wiremock.stop()
    vertxStopFuture.get()
  }

  @Test
  fun success_all_ok() {
    val shomeiClient = ShomeiClient(jsonRpcClient)
    val successResponse =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        "1",
        "result",
        "success",
      )
    wiremock.stubFor(
      WireMock.post("/")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .willReturn(
          WireMock.ok().withHeader("Content-type", "application/json").withBody(successResponse.toString()),
        ),
    )
    val blockNumberAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = shomeiClient.rollupForkChoiceUpdated(blockNumberAndHash)
    val result = resultFuture.get()
    Assertions.assertThat(resultFuture)
      .isCompleted()
    Assertions.assertThat(result is Ok)

    val expectedJsonRequest =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        1,
        "method",
        "rollup_forkChoiceUpdated",
        "params",
        listOf(
          mapOf(
            "finalizedBlockNumber" to blockNumberAndHash.number.toString(),
            "finalizedBlockHash" to blockNumberAndHash.hash.encodeHex(),
          ),
        ),
      )
    wiremock.verify(
      WireMock.postRequestedFor(WireMock.urlEqualTo("/"))
        .withHeader("Content-Type", WireMock.equalTo("application/json"))
        .withRequestBody(WireMock.equalToJson(expectedJsonRequest.toString(), false, true)),
    )
  }

  @Test
  fun internal_error_shomei() {
    val shomeiClient = ShomeiClient(jsonRpcClient)
    val jsonRpcErrorResponse =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        "1",
        "error",
        mapOf("code" to "1", "message" to "Internal Error"),
      )

    wiremock.stubFor(
      WireMock.post("/")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .willReturn(
          WireMock.aResponse()
            .withStatus(200)
            .withBody(jsonRpcErrorResponse.toString()),
        ),
    )
    val blockNumberAndHash = BlockNumberAndHash(1U, ByteArrayExt.random32())
    val resultFuture = shomeiClient.rollupForkChoiceUpdated(blockNumberAndHash)
    val result = resultFuture.get()
    Assertions.assertThat(resultFuture)
      .isCompleted()
    Assertions.assertThat(result is Err)

    val expectedJsonRequest =
      JsonObject.of(
        "jsonrpc",
        "2.0",
        "id",
        1,
        "method",
        "rollup_forkChoiceUpdated",
        "params",
        listOf(
          mapOf(
            "finalizedBlockNumber" to blockNumberAndHash.number.toString(),
            "finalizedBlockHash" to blockNumberAndHash.hash.encodeHex(),
          ),
        ),
      )
    wiremock.verify(
      WireMock.postRequestedFor(WireMock.urlEqualTo("/"))
        .withHeader("Content-Type", WireMock.equalTo("application/json"))
        .withRequestBody(WireMock.equalToJson(expectedJsonRequest.toString(), false, true)),
    )
  }
}
