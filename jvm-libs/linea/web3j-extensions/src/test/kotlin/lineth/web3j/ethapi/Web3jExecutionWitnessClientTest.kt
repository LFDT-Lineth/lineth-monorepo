package lineth.web3j.ethapi

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.vertx.core.json.JsonObject
import lineth.domain.BlockParameter
import lineth.error.JsonRpcErrorResponseException
import lineth.ethapi.ExecutionWitness
import lineth.kotlin.decodeHex
import lineth.kotlin.encodeHex
import lineth.web3j.createWeb3jHttpService
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.assertj.core.api.Assertions.catchThrowable
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class Web3jExecutionWitnessClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var client: Web3jExecutionWitnessClient

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
    val witnessJson = """
      {
        "state": ["0xf902"],
        "codes": ["0x608060"],
        "headers": ["0xf902"]
      }
    """.trimIndent()
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"method\":\"debug_executionWitness\""))
        .withRequestBody(containing("\"params\":[\"42\"]"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(witnessJson)).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.BlockNumber(42UL)).get()

    assertThat(result).isEqualTo(
      ExecutionWitness(
        state = listOf("f902".decodeHex()),
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
    val witnessJson = """
      {
        "state": ["0xf902"],
        "codes": ["0x608060"],
        "headers": ["0xf902"]
      }
    """.trimIndent()

    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"params\":[\"$hashParam\"]"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(witnessJson)).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.fromHash(hash)).get()

    assertThat(result).isNotNull
    assertThat(result!!.state).isNotEmpty
    wiremock.verify(
      postRequestedFor(urlEqualTo("/")).withRequestBody(containing(hashParam)),
    )
  }

  @Test
  fun `getExecutionWitness parses multiple items in each list`() {
    // Real witnesses hold many trie nodes, several contract codes, and (with BLOCKHASH access)
    // multiple ancestor headers. Each array element must be decoded in order.
    val witnessJson = """
      {
        "state": ["0xf902", "0xf844", "0xa1b2"],
        "codes": ["0x608060", "0x6080"],
        "headers": ["0xf902", "0xc3d4"]
      }
    """.trimIndent()
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(witnessJson)).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.Tag.LATEST).get()

    assertThat(result).isEqualTo(
      ExecutionWitness(
        state = listOf("f902".decodeHex(), "f844".decodeHex(), "a1b2".decodeHex()),
        codes = listOf("608060".decodeHex(), "6080".decodeHex()),
        headers = listOf("f902".decodeHex(), "c3d4".decodeHex()),
      ),
    )
  }

  @Test
  fun `getExecutionWitness parses empty codes list`() {
    // Besu omits no fields and has no guard against empty `codes`: a block that touches only EOAs
    // (no contract/system code accessed) is serialized as "codes": []. It must parse to emptyList.
    val witnessJson = """
      {
        "state": ["0xf902"],
        "codes": [],
        "headers": ["0xf902"]
      }
    """.trimIndent()
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(witnessJson)).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.Tag.LATEST).get()

    assertThat(result).isEqualTo(
      ExecutionWitness(
        state = listOf("f902".decodeHex()),
        codes = emptyList(),
        headers = listOf("f902".decodeHex()),
      ),
    )
  }

  @Test
  fun `getExecutionWitness fails when a field is missing`() {
    // A field that is absent (rather than an empty array) is unexpected from besu and must fail
    // rather than silently default.
    val witnessJson = """
      {
        "state": ["0xf902"],
        "headers": ["0xf902"]
      }
    """.trimIndent()
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(witnessJson)).encode(),
          ),
        ),
    )

    val thrown = catchThrowable { client.getExecutionWitness(BlockParameter.Tag.LATEST).get() }
    val parseException = generateSequence(thrown) { it.cause }
      .filterIsInstance<IllegalArgumentException>()
      .firstOrNull()
    assertThat(parseException).isNotNull
    assertThat(parseException!!.message).contains("codes")
  }

  @Test
  fun `getExecutionWitness returns null when result is null`() {
    // Besu returns a JSON-RPC success with result=null when it has no witness for the block
    // (e.g. an unknown block hash); this surfaces as a null witness, not an error.
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .willReturn(
          ok(
            JsonObject.of("jsonrpc", "2.0", "id", 1, "result", null).encode(),
          ),
        ),
    )

    val result = client.getExecutionWitness(BlockParameter.Tag.LATEST).get()

    assertThat(result).isNull()
  }

  @Test
  fun `getExecutionWitness fails when result is malformed`() {
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
    val parseException = generateSequence(thrown) { it.cause }
      .filterIsInstance<IllegalArgumentException>()
      .firstOrNull()
    assertThat(parseException).isNotNull
    assertThat(parseException!!.message).contains("state")
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
