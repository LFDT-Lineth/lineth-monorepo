package linea.web3j.ethapi

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.vertx.core.json.JsonObject
import linea.domain.BlockParameter
import linea.error.JsonRpcErrorResponseException
import linea.ethapi.ExecutionWitness
import linea.ethapi.ExecutionWitnessClientException
import linea.ethapi.ExecutionWitnessError
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.web3j.createWeb3jHttpService
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.assertj.core.api.Assertions.catchThrowable
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class Web3jExecutionWitnessClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var client: Web3jExecutionWitnessClient

  private val sampleWitnessJson = """
    {
      "state": ["0xf902"],
      "keys": ["0xf844"],
      "codes": ["0x608060"],
      "headers": ["0xf902"]
    }
  """.trimIndent()

  @BeforeEach
  fun setup() {
    wiremock = WireMockServer(options().dynamicPort())
    wiremock.start()
    val web3jService = createWeb3jHttpService(rpcUrl = "http://127.0.0.1:${wiremock.port()}")
    client = Web3jExecutionWitnessClient(web3jService)
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun `getExecutionWitness returns parsed witness for block number`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"method\":\"debug_executionWitness\""))
        .withRequestBody(containing("\"params\":[\"42\"]"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(sampleWitnessJson)).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.BlockNumber(42UL)).get()

    assertThat(result).isEqualTo(
      ExecutionWitness(
        state = listOf("f902".decodeHex()),
        keys = listOf("f844".decodeHex()),
        codes = listOf("608060".decodeHex()),
        headers = listOf("f902".decodeHex()),
      ),
    )
    wiremock.verify(postRequestedFor(urlEqualTo("/")))
  }

  @Test
  fun `getExecutionWitness returns parsed witness for block hash`() {
    val hash = ByteArray(32) { 0xab.toByte() }
    val hashParam = hash.encodeHex(prefix = true)

    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"params\":[\"$hashParam\"]"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(sampleWitnessJson)).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.fromHash(hash)).get()

    assertThat(result.state).isNotEmpty
    wiremock.verify(
      postRequestedFor(urlEqualTo("/")).withRequestBody(containing(hashParam)),
    )
  }

  @Test
  fun `getExecutionWitness throws NULL_RESULT when result is null`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", null).encode(),
          ),
        ),
    )

    assertThatThrownBy { client.getExecutionWitness(BlockParameter.Tag.LATEST).get() }
      .rootCause()
      .isInstanceOfSatisfying(ExecutionWitnessClientException::class.java) { ex ->
        assertThat(ex.errorType).isEqualTo(ExecutionWitnessError.NULL_RESULT)
        assertThat(ex.message).contains("returned null")
      }
  }

  @Test
  fun `getExecutionWitness throws PARSE_ERROR when result is malformed`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of(
              "jsonrpc",
              "2.0",
              "id",
              1,
              "result",
              // "state" is not an array -> parse failure
              JsonObject.of(
                "state",
                "not-an-array",
                "keys",
                JsonObject(),
                "codes",
                JsonObject(),
                "headers",
                JsonObject(),
              ),
            ).encode(),
          ),
        ),
    )

    val thrown = catchThrowable { client.getExecutionWitness(BlockParameter.Tag.LATEST).get() }
    val witnessException = generateSequence(thrown) { it.cause }
      .filterIsInstance<ExecutionWitnessClientException>()
      .firstOrNull()
    assertThat(witnessException).isNotNull
    assertThat(witnessException!!.errorType).isEqualTo(ExecutionWitnessError.PARSE_ERROR)
  }

  @Test
  fun `getExecutionWitness throws on json-rpc error`() {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of(
              "jsonrpc",
              "2.0",
              "id",
              1,
              "error",
              JsonObject.of("code", -32603, "message", "Internal error"),
            ).encode(),
          ),
        ),
    )

    assertThatThrownBy { client.getExecutionWitness(BlockParameter.Tag.LATEST).get() }
      .rootCause()
      .isInstanceOfSatisfying(JsonRpcErrorResponseException::class.java) { ex ->
        assertThat(ex.rpcErrorCode).isEqualTo(-32603)
        assertThat(ex.rpcErrorMessage).isEqualTo("Internal error")
      }
  }
}
