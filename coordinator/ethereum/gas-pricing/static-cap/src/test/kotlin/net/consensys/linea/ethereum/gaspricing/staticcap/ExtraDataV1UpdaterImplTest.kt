package net.consensys.linea.ethereum.gaspricing.staticcap

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.wireMockConfig
import com.github.tomakehurst.wiremock.matching.EqualToJsonPattern
import com.github.tomakehurst.wiremock.matching.EqualToPattern
import com.github.tomakehurst.wiremock.matching.RequestPatternBuilder
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataV1
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.net.URL
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class ExtraDataV1UpdaterImplTest {
  private lateinit var sequencerEndpoint: URL
  val requestRetry = RequestRetryConfig(
    maxRetries = 2u,
    backoffDelay = 10.milliseconds,
    failuresWarningThreshold = 1u,
  )
  lateinit var wiremock: WireMockServer
  private lateinit var jsonRpcClientFactory: VertxHttpJsonRpcClientFactory
  private val minerExtraData = MinerExtraDataV1(
    fixedCostInKWei = 1u,
    variableCostInKWei = 2u,
    ethGasPriceInKWei = 10u,
  )
  val setMinerExtraDataSuccessResponse =
    JsonObject.of(
      "jsonrpc",
      "2.0",
      "id",
      1,
      "result",
      true,
    )
  val expectedRequest = JsonObject.of(
    "jsonrpc",
    "2.0",
    "method",
    "miner_setExtraData",
    "params",
    listOf(minerExtraData.encode()),
  )

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    wiremock = WireMockServer(wireMockConfig().dynamicPort())
    wiremock.start()
    sequencerEndpoint = URI("http://localhost:${wiremock.port()}/sequencer/").toURL()
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    jsonRpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, metricsFacade)
  }

  @AfterEach
  fun afterEach() {
    wiremock.stop()
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun `extra data updater impl sets extra data on sequencer`(testContext: VertxTestContext) {
    wiremockStubForPost(wiremock, sequencerEndpoint, setMinerExtraDataSuccessResponse)
    val extraDataUpdaterImpl =
      ExtraDataV1UpdaterImpl(
        jsonRpcClientFactory,
        ExtraDataV1UpdaterImpl.Config(
          sequencerEndpoint = sequencerEndpoint,
          retryConfig = requestRetry,
        ),
      )
    extraDataUpdaterImpl.updateMinerExtraData(extraData = minerExtraData)
      .thenApply {
        testContext.verify { verifyRequest(wiremock, sequencerEndpoint, expectedRequest) }.completeNow()
      }
  }

  private fun wiremockStubForPost(wiremock: WireMockServer, requestOriginEndpoint: URL, response: JsonObject) {
    wiremock.stubFor(
      post(requestOriginEndpoint.path)
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(response.toString()),
        ),
    )
  }

  private fun verifyRequest(
    wiremock: WireMockServer,
    requestOriginEndpoint: URL,
    request: JsonObject,
  ) {
    wiremock.verify(
      RequestPatternBuilder.newRequestPattern()
        .withPort(wiremock.port())
        .withUrl(requestOriginEndpoint.path)
        .withHeader("content-type", EqualToPattern("application/json"))
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
