package net.consensys.linea.ethereum.gaspricing.staticcap

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import com.github.tomakehurst.wiremock.matching.EqualToJsonPattern
import com.github.tomakehurst.wiremock.matching.EqualToPattern
import com.github.tomakehurst.wiremock.matching.RequestPatternBuilder
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.net.URL
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class GasPriceUpdaterImplTest {
  private lateinit var gethRecipients: List<URL>
  private lateinit var besuRecipients: List<URL>
  private val requestRetry = RequestRetryConfig(
    maxRetries = 2u,
    backoffDelay = 10.milliseconds,
    failuresWarningThreshold = 1u,
  )
  private lateinit var wiremock: WireMockServer
  private lateinit var jsonRpcClientFactory: VertxHttpJsonRpcClientFactory
  private val setPriceSuccessResponse =
    JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      true,
    )
  private val expectedGethRequest = JsonObject.of(
    "jsonrpc",
    "2.0",
    "method",
    "miner_setGasPrice",
    "params",
    listOf("0xa"),
  )
  private val expectedBesuRequest = expectedGethRequest.copy()
    .put("method", "miner_setMinGasPrice")

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    wiremock = WireMockServer(
      WireMockConfiguration.wireMockConfig().dynamicPort(),
    )
    wiremock.start()
    gethRecipients = listOf(
      URI("http://localhost:${wiremock.port()}/geth-1/").toURL(),
      URI("http://localhost:${wiremock.port()}/geth-2/").toURL(),
      URI("http://localhost:${wiremock.port()}/geth-3/").toURL(),
    )
    besuRecipients = listOf(
      URI("http://localhost:${wiremock.port()}/besu-1/").toURL(),
      URI("http://localhost:${wiremock.port()}/besu-2/").toURL(),
      URI("http://localhost:${wiremock.port()}/besu-3/").toURL(),
    )
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    jsonRpcClientFactory = VertxHttpJsonRpcClientFactory(
      vertx,
      metricsFacade,
    )
  }

  @AfterEach
  fun afterEach() {
    wiremock.stop()
  }

  @Test
  fun throwsExceptionIfNoEndpoints() {
    val exception = assertThrows<IllegalArgumentException> {
      GasPriceUpdaterImpl.Config(
        gethEndpoints = emptyList(),
        besuEndPoints = emptyList(),
        retryConfig = requestRetry,
      )
    }

    Assertions.assertThat(exception)
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("Must have at least one geth or besu endpoint to update the gas price")

    // works with at least one geth endpoint
    GasPriceUpdaterImpl.Config(
      gethEndpoints = listOf(URI("http://localhost:8545").toURL()),
      besuEndPoints = emptyList(),
      retryConfig = requestRetry,
    )
    // works with at least one Besu endpoint
    GasPriceUpdaterImpl.Config(
      gethEndpoints = emptyList(),
      besuEndPoints = listOf(URI("http://localhost:8545").toURL()),
      retryConfig = requestRetry,
    )
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun gasPriceUpdaterImpl_setsPriceOnGethAndBesu(testContext: VertxTestContext) {
    testPriceUpdateForEndpoints(testContext, gethRecipients, besuRecipients)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun gasPriceUpdaterImpl_setsPriceOnGethOnly(testContext: VertxTestContext) {
    testPriceUpdateForEndpoints(testContext, gethRecipients = gethRecipients, besuRecipients = emptyList())
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun gasPriceUpdaterImpl_setsPriceOnBesuOnly(testContext: VertxTestContext) {
    testPriceUpdateForEndpoints(testContext, gethRecipients = emptyList(), besuRecipients = besuRecipients)
  }

  private fun testPriceUpdateForEndpoints(
    testContext: VertxTestContext,
    gethRecipients: List<URL>,
    besuRecipients: List<URL>,
  ) {
    val gasPrice = 10uL
    gethRecipients.forEach { endpoint -> wiremockStubForPost(wiremock, endpoint, setPriceSuccessResponse) }
    besuRecipients.forEach { endpoint -> wiremockStubForPost(wiremock, endpoint, setPriceSuccessResponse) }

    val l2GasPriceUpdaterImpl =
      GasPriceUpdaterImpl(
        jsonRpcClientFactory,
        GasPriceUpdaterImpl.Config(
          gethEndpoints = gethRecipients,
          besuEndPoints = besuRecipients,
          retryConfig = requestRetry,
        ),
      )

    l2GasPriceUpdaterImpl.updateMinerGasPrice(gasPrice)
      .thenApply {
        testContext
          .verify {
            gethRecipients.forEach { endpoint -> verifyRequest(wiremock, endpoint, expectedGethRequest) }
            besuRecipients.forEach { endpoint -> verifyRequest(wiremock, endpoint, expectedBesuRequest) }
          }
          .completeNow()
      }
  }

  private fun wiremockStubForPost(wiremock: WireMockServer, requestOriginEndpoint: URL, response: JsonObject) {
    wiremock.stubFor(
      WireMock.post(requestOriginEndpoint.path)
        .withHeader(
          "Content-Type",
          WireMock.containing("application/json"),
        )
        .willReturn(
          WireMock.ok()
            .withHeader("Content-type", "application/json")
            .withBody(response.toString()),
        ),
    )
  }

  private fun verifyRequest(wiremock: WireMockServer, requestOriginEndpoint: URL, request: JsonObject) {
    wiremock.verify(
      RequestPatternBuilder.newRequestPattern()
        .withPort(wiremock.port())
        .withUrl(requestOriginEndpoint.path)
        .withHeader(
          "content-type",
          EqualToPattern("application/json"),
        )
        .withRequestBody(
          EqualToJsonPattern(
            request.toString(), /*ignoreArrayOrder*/
            false, /*ignoreExtraElements*/
            true,
          ),
        ),
    )
  }
}
