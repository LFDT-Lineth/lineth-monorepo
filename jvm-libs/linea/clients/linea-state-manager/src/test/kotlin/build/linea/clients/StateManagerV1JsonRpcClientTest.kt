package build.linea.clients

import build.linea.domain.BlockInterval
import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.decodeHex
import net.consensys.fromHexString
import net.consensys.linea.async.get
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.testing.filesystem.findPathTo
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class StateManagerV1JsonRpcClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var stateManagerClient: StateManagerV1JsonRpcClient
  private lateinit var meterRegistry: SimpleMeterRegistry

  private fun wiremockStubForPost(response: String) {
    wiremock.stubFor(
      post("/")
        .withHeader("Content-Type", containing("application/json"))
        .willReturn(
          ok()
            .withHeader("Content-type", "application/json")
            .withBody(response.toByteArray())
        )
    )
  }

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(options().dynamicPort())
    wiremock.start()
    meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    stateManagerClient = StateManagerV1JsonRpcClient.create(
      rpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, metricsFacade),
      endpoints = listOf(URI(wiremock.baseUrl())),
      maxInflightRequestsPerClient = 1u,
      requestRetry = RequestRetryConfig(
        maxRetries = 2u,
        timeout = 2.seconds,
        10.milliseconds,
        1u
      ),
      zkStateManagerVersion = "0.1.2"
    )
  }

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    wiremock.stop()
    vertxStopFuture.get()
  }

  @Test
  fun getZkEVMStateMerkleProof_success() {
    val testFilePath = findPathTo("testdata")!!.resolve("type2state-manager/state-proof.json")
    val json = jacksonObjectMapper().readTree(testFilePath.toFile())
    val zkStateManagerVersion = json.get("zkStateManagerVersion").asText()
    val zkStateMerkleProof = json.get("zkStateMerkleProof") as ArrayNode
    val zkParentStateRootHash = json.get("zkParentStateRootHash").asText()
    val zkEndStateRootHash = json.get("zkEndStateRootHash").asText()

    wiremockStubForPost(
      """
      {
        "jsonrpc":"2.0",
        "id":"1",
        "result": {
          "zkParentStateRootHash": "$zkParentStateRootHash",
          "zkEndStateRootHash": "$zkEndStateRootHash",
          "zkStateMerkleProof": $zkStateMerkleProof,
          "zkStateManagerVersion": "$zkStateManagerVersion"
        }
      }
    """
    )

    assertThat(stateManagerClient.rollupGetStateMerkleProofWithTypedError(BlockInterval(50UL, 100UL)))
      .succeedsWithin(5.seconds.toJavaDuration())
      .isEqualTo(
        Ok(
          GetZkEVMStateMerkleProofResponse(
            zkStateManagerVersion = zkStateManagerVersion,
            zkStateMerkleProof = zkStateMerkleProof,
            zkParentStateRootHash = zkParentStateRootHash.decodeHex(),
            zkEndStateRootHash = zkEndStateRootHash.decodeHex()
          )
        )
      )
  }

  @Test
  fun getZkEVMStateMerkleProof_error_block_missing() {
    wiremockStubForPost(
      """
      {
        "jsonrpc":"2.0",
        "id":"1",
        "error":{
          "code":"-32600",
          "message":"BLOCK_MISSING_IN_CHAIN - block 1 is missing"
         }
      }"""
    )

    assertThat(stateManagerClient.rollupGetStateMerkleProofWithTypedError(BlockInterval(50UL, 100UL)))
      .succeedsWithin(5.seconds.toJavaDuration())
      .isEqualTo(
        Err(
          ErrorResponse(
            StateManagerErrorType.BLOCK_MISSING_IN_CHAIN,
            "BLOCK_MISSING_IN_CHAIN - block 1 is missing"
          )
        )
      )
  }

  @Test
  fun getZkEVMStateMerkleProof_error_unsupported_version() {
    val response = """
      {
        "jsonrpc":"2.0",
        "id":"1",
        "error":{
          "code":"-32602",
          "message":"UNSUPPORTED_VERSION",
          "data": {
            "requestedVersion": "0.1.2",
            "supportedVersion": "0.0.1-dev-3e607237"
          }
         }
      }"""

    wiremockStubForPost(response)

    assertThat(stateManagerClient.rollupGetStateMerkleProofWithTypedError(BlockInterval(50UL, 100UL)))
      .succeedsWithin(5.seconds.toJavaDuration())
      .isEqualTo(
        Err(
          ErrorResponse(
            StateManagerErrorType.UNSUPPORTED_VERSION,
            "UNSUPPORTED_VERSION: {requestedVersion=0.1.2, supportedVersion=0.0.1-dev-3e607237}"
          )
        )
      )
  }

  @Test
  fun getZkEVMStateMerkleProof_error_unknown() {
    wiremockStubForPost(
      """
      {
        "jsonrpc":"2.0",
        "id":"1",
        "error":{
          "code":-999,
          "message":"BRA_BRA_BRA_SOME_UNKNOWN_ERROR",
          "data": {"xyz": "1234", "abc": 100}
         }
      }"""
    )

    assertThat(stateManagerClient.rollupGetStateMerkleProofWithTypedError(BlockInterval(50L, 100L)))
      .succeedsWithin(5.seconds.toJavaDuration())
      .isEqualTo(
        Err(ErrorResponse(StateManagerErrorType.UNKNOWN, """BRA_BRA_BRA_SOME_UNKNOWN_ERROR: {xyz=1234, abc=100}"""))
      )
  }

  @Test
  fun rollupGetHeadBlockNumber_success_response() {
    wiremockStubForPost("""{"jsonrpc":"2.0","id":1,"result":"0xf1"}""")

    assertThat(stateManagerClient.rollupGetHeadBlockNumber().get())
      .isEqualTo(ULong.fromHexString("0xf1"))
  }

  @Test
  fun rollupGetHeadBlockNumber_error_response() {
    val response = """{"jsonrpc":"2.0","id":1,"error":{"code": -32603, "message": "Internal error"}}"""

    wiremockStubForPost(response)

    assertThatThrownBy { stateManagerClient.rollupGetHeadBlockNumber().get() }
      .hasMessageContaining("Internal error")
  }
}
