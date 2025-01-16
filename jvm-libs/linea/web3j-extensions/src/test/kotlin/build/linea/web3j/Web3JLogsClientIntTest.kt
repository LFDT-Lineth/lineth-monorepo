package build.linea.web3j

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.aResponse
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import net.consensys.encodeHex
import net.consensys.toBigInteger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.http.HttpService
import java.net.URI
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class Web3JLogsClientIntTest {
  private lateinit var web3jClient: Web3j
  private lateinit var logsClient: Web3JLogsClient
  private lateinit var vertx: Vertx
  private lateinit var wireMockServer: WireMockServer

  val ethFilter =
    EthFilter(
      DefaultBlockParameter.valueOf(1UL.toBigInteger()),
      DefaultBlockParameter.valueOf(9UL.toBigInteger()),
      "0x508ca82df566dcd1b0de8296e70a96332cd644ec"
    )

  @BeforeEach
  fun setup() {
    wireMockServer = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wireMockServer.start()

    web3jClient = Web3j.build(HttpService(URI("http://127.0.0.1:" + wireMockServer.port()).toURL().toString()))
    vertx = Vertx.vertx()
    logsClient = Web3JLogsClient(
      vertx,
      web3jClient,
      config = Web3JLogsClient.Config(
        timeout = 500.seconds,
        backoffDelay = 1.milliseconds,
        lookBackRange = 100
      )
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  @Test
  fun `when eth_getLogs returns json-rpc error shall return failed promise`() {
    val jsonRpcErrorResponse = """
        {
          "jsonrpc": "2.0",
          "error": {
            "code": -32000,
            "message": "Error: unable to retrieve logs"
          },
          "id": 1
        }
    """.trimIndent()

    wireMockServer.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("eth_getLogs"))
        .willReturn(
          aResponse()
            .withStatus(200)
            .withBody(jsonRpcErrorResponse)
            .withHeader("Content-Type", "application/json")
        )
    )

    assertThatThrownBy { logsClient.getLogs(ethFilter).get() }
      .hasCauseInstanceOf(RuntimeException::class.java)
      .hasMessageContaining("json-rpc error: code=-32000 message=Error: unable to retrieve logs")
  }

  @Test
  fun `when eth_getLogs gets an HTTP error shall return failed promise`() {
    wireMockServer.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("eth_getLogs"))
        .willReturn(
          aResponse()
            .withStatus(500)
            .withBody("Internal Server Error")
        )
    )

    assertThatThrownBy { logsClient.getLogs(ethFilter).get() }
      .hasCauseInstanceOf(org.web3j.protocol.exceptions.ClientConnectionException::class.java)
      .hasMessageContaining("Invalid response received: 500; Internal Server Error")
  }

  @Test
  fun `when eth_getLogs gets a DNS error shall return failed promise`() {
    val randomHostname = "nowhere-${Random.nextBytes(20).encodeHex()}.local"
    web3jClient = Web3j.build(HttpService("http://$randomHostname:1234"))
    vertx = Vertx.vertx()
    logsClient = Web3JLogsClient(
      vertx,
      web3jClient,
      config = Web3JLogsClient.Config(
        timeout = 500.seconds,
        backoffDelay = 1.milliseconds,
        lookBackRange = 100
      )
    )

    assertThatThrownBy { logsClient.getLogs(ethFilter).get() }
      .hasCauseInstanceOf(java.net.UnknownHostException::class.java)
      .hasMessageContaining("$randomHostname")
  }

  @Test
  fun `when eth_getLogs returns json-rpc result shall parse the logs`() {
    val jsonRpcErrorResponse = """
       {
          "jsonrpc": "2.0",
          "id": 1,
          "result": [
              {
                  "address": "0x508ca82df566dcd1b0de8296e70a96332cd644ec",
                  "blockHash": "0x216a74dcf2eff89d4de2018cb802f5afba0be20ad13fc04747982a48be1fa02c",
                  "blockNumber": "0x864b52",
                  "data": "0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005112ab49c64000000000000000000000000000000000000000000000000000000000000001253b00000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000",
                  "logIndex": "0x0",
                  "removed": false,
                  "topics": [
                      "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c",
                      "0x00000000000000000000000034d5011fc936784ed841ca7d010b27521eb5836a",
                      "0x00000000000000000000000034d5011fc936784ed841ca7d010b27521eb5836a",
                      "0x9949dcba752e2d685c69f6473546fbe44cd5cdbb7152ee83b722241993d93e0b"
                  ],
                  "transactionHash": "0xfcff12cba7002ec38f391f245232dbe362a79eaae7816e35f36356e302e8877f",
                  "transactionIndex": "0x0"
              },
              {
                  "address": "0x508ca82df566dcd1b0de8296e70a96332cd644ec",
                  "blockHash": "0xcf2bb49c88a3c0f3b76093bb76a53900484f9bb494a68369e138f37d838a76fa",
                  "blockNumber": "0x864b58",
                  "data": "0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000004b920659bad161816eaf400be8b79a21bf5746d24dfa7c65c72628c776e5de8b646b8c4b545e17a1b5a075dc754511cc7ce3a3ded01d240eae99326812020f8611444851d3da8efd96f17ae7ed230662165e3cf27decd525172d1886e9e7188d2295f5eae70c34e30ac9290534d114d662d538c6da6a36e9f4ada9be229dbab56",
                  "logIndex": "0x0",
                  "removed": false,
                  "topics": [
                      "0x9995fb3da0c2de4012f2b814b6fc29ce7507571dcb20b8d0bd38621a842df1eb"
                  ],
                  "transactionHash": "0xeda559915d3d1067b290406308f3d5ba961841e922658e57312860f76185e668",
                  "transactionIndex": "0x0"
              },
              {
                  "address": "0x508ca82df566dcd1b0de8296e70a96332cd644ec",
                  "blockHash": "0xcf2bb49c88a3c0f3b76093bb76a53900484f9bb494a68369e138f37d838a76fa",
                  "blockNumber": "0x864b58",
                  "data": "0x",
                  "logIndex": "0x1",
                  "removed": false,
                  "topics": [
                      "0x99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac",
                      "0x00000000000000000000000000000000000000000000000000000000000aaab9",
                      "0xb610d3d7c2d98c594d69ca54ca846dee1e7ffea0733fd90e7bec0ecaee9bd314"
                  ],
                  "transactionHash": "0xeda559915d3d1067b290406308f3d5ba961841e922658e57312860f76185e668",
                  "transactionIndex": "0x0"
              }
          ]
      }
    """.trimIndent()

    wireMockServer.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("eth_getLogs"))
        .willReturn(
          aResponse()
            .withStatus(200)
            .withBody(jsonRpcErrorResponse)
            .withHeader("Content-Type", "application/json")
        )
    )

    val logs = logsClient.getLogs(ethFilter).get()
    // assert 1st block logs
    assertThat(logs.size).isEqualTo(3)
    assertThat(logs[0].blockNumber).isEqualTo(8801106.toBigInteger())
    assertThat(logs[0].topics).isEqualTo(
      listOf(
        "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c",
        "0x00000000000000000000000034d5011fc936784ed841ca7d010b27521eb5836a",
        "0x00000000000000000000000034d5011fc936784ed841ca7d010b27521eb5836a",
        "0x9949dcba752e2d685c69f6473546fbe44cd5cdbb7152ee83b722241993d93e0b"
      )
    )
    assertThat(logs[0].transactionHash).isEqualTo("0xfcff12cba7002ec38f391f245232dbe362a79eaae7816e35f36356e302e8877f")
    assertThat(logs[0].transactionIndex).isEqualTo(0.toBigInteger())

    // assert 2nd block logs
    assertThat(logs[1].blockNumber).isEqualTo(8801112.toBigInteger())
    assertThat(logs[1].topics).isEqualTo(
      listOf(
        "0x9995fb3da0c2de4012f2b814b6fc29ce7507571dcb20b8d0bd38621a842df1eb"
      )
    )
    assertThat(logs[1].transactionHash).isEqualTo("0xeda559915d3d1067b290406308f3d5ba961841e922658e57312860f76185e668")
    assertThat(logs[1].transactionIndex).isEqualTo(0.toBigInteger())

    assertThat(logs[2].blockNumber).isEqualTo(8801112.toBigInteger())
    assertThat(logs[2].topics).isEqualTo(
      listOf(
        "0x99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac",
        "0x00000000000000000000000000000000000000000000000000000000000aaab9",
        "0xb610d3d7c2d98c594d69ca54ca846dee1e7ffea0733fd90e7bec0ecaee9bd314"
      )
    )
    assertThat(logs[2].transactionHash).isEqualTo("0xeda559915d3d1067b290406308f3d5ba961841e922658e57312860f76185e668")
    assertThat(logs[2].transactionIndex).isEqualTo(0.toBigInteger())
  }
}
