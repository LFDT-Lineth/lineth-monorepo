package linea.web3j.ethapi

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.vertx.core.json.JsonObject
import linea.error.JsonRpcErrorResponseException
import linea.ethapi.EthConfig
import linea.ethapi.ForkConfig
import linea.web3j.createWeb3jHttpService
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.catchThrowable
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class Web3jEthConfigClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var client: Web3jEthConfigClient

  @BeforeEach
  fun setup() {
    wiremock = WireMockServer(options().dynamicPort())
    wiremock.start()
    client = Web3jEthConfigClient(createWeb3jHttpService(rpcUrl = "http://127.0.0.1:${wiremock.port()}"))
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  private fun stubResult(resultJson: String) {
    wiremock.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("\"method\":\"eth_config\""))
        .willReturn(ok(JsonObject.of("jsonrpc", "2.0", "id", 1, "result", JsonObject(resultJson)).encode())),
    )
  }

  @Test
  fun `parses current, next and last fork configs`() {
    stubResult(
      """
      {
        "current": {
          "activationTime": 0,
          "blobSchedule": { "baseFeeUpdateFraction": 5007716, "max": 9, "target": 6 },
          "chainId": "0xe708",
          "forkId": "0x0929e24e",
          "precompiles": { "ECREC": "0x0000000000000000000000000000000000000001" },
          "systemContracts": {
            "BEACON_ROOTS_ADDRESS": "0x000f3df6d732807ef1319fb7b8bb8522d0beac02",
            "DEPOSIT_CONTRACT_ADDRESS": "0x94d4a7449b968b839dcbc64a61f195ca300986ae"
          }
        },
        "next": {
          "activationTime": 1800000000,
          "chainId": "0xe708",
          "systemContracts": {
            "BUILDER_EXIT_CONTRACT_ADDRESS": "0x000064d678505ad48f8ccb093bc65613800e8282"
          }
        },
        "last": {
          "activationTime": 1800000000,
          "chainId": "0xe708",
          "systemContracts": {}
        }
      }
      """.trimIndent(),
    )

    assertThat(client.ethConfig().get()).isEqualTo(
      EthConfig(
        current = ForkConfig(
          activationTime = 0UL,
          chainId = 59144UL,
          systemContracts = mapOf(
            "BEACON_ROOTS_ADDRESS" to "0x000f3df6d732807ef1319fb7b8bb8522d0beac02",
            "DEPOSIT_CONTRACT_ADDRESS" to "0x94d4a7449b968b839dcbc64a61f195ca300986ae",
          ),
        ),
        next = ForkConfig(
          activationTime = 1800000000UL,
          chainId = 59144UL,
          systemContracts = mapOf("BUILDER_EXIT_CONTRACT_ADDRESS" to "0x000064d678505ad48f8ccb093bc65613800e8282"),
        ),
        last = ForkConfig(activationTime = 1800000000UL, chainId = 59144UL, systemContracts = emptyMap()),
      ),
    )
  }

  @Test
  fun `parses null next and last and a fork without system contracts`() {
    stubResult("""{ "current": { "activationTime": 5, "chainId": "0x539" }, "next": null, "last": null }""")

    assertThat(client.ethConfig().get()).isEqualTo(
      EthConfig(
        current = ForkConfig(activationTime = 5UL, chainId = 1337UL, systemContracts = emptyMap()),
        next = null,
        last = null,
      ),
    )
  }

  @Test
  fun `fails when current is missing`() {
    stubResult("""{ "next": null, "last": null }""")

    val thrown = catchThrowable { client.ethConfig().get() }
    val parseException = generateSequence(thrown) { it.cause }.filterIsInstance<IllegalArgumentException>().first()
    assertThat(parseException.message).contains("current")
  }

  @Test
  fun `rejects JSON-RPC error responses`() {
    wiremock.stubFor(
      post(urlEqualTo("/")).willReturn(
        ok("""{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}"""),
      ),
    )

    val thrown = catchThrowable { client.ethConfig().get() }
    assertThat(generateSequence(thrown) { it.cause }.filterIsInstance<JsonRpcErrorResponseException>().first())
      .hasMessageContaining("Method not found")
  }
}
