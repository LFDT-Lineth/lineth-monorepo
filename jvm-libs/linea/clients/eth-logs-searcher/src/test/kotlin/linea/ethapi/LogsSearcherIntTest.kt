package linea.ethapi

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.aResponse
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import linea.SearchDirection
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLog
import linea.domain.RetryConfig
import linea.jsonrpc.TestingJsonRpcServer
import linea.kotlin.encodeHex
import linea.kotlin.fromHexString
import linea.kotlin.toHexString
import linea.kotlin.toHexStringUInt256
import linea.log4j.configureLoggers
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.jsonrpc.JsonRpcError
import net.consensys.linea.jsonrpc.JsonRpcRequest
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import java.net.URI
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds

internal data class EthGetLogsRequest(
  val fromBlock: ULong,
  val toBlock: ULong,
  val topics: List<String>,
  val address: List<String>
)

class EthLogsSearcherImplIntTest {
  private lateinit var web3jClient: Web3j
  private lateinit var logsClient: EthLogsSearcherImpl
  private lateinit var vertx: Vertx
  private lateinit var wireMockServer: WireMockServer
  private lateinit var TestingJsonRpcServer: TestingJsonRpcServer
  private val address = "0x508ca82df566dcd1b0de8296e70a96332cd644ec"
  private val log = LogManager.getLogger("test.case.Web3JLogsSearcherIntTest")

  @BeforeEach
  fun beforeEach() {
    vertx = Vertx.vertx()
    configureLoggers(
      rootLevel = Level.INFO,
      log.name to Level.DEBUG,
      "test.case.Web3JLogsSearcher" to Level.DEBUG
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  private fun setupClientWithWireMockServer(
    retryConfig: RetryConfig = RetryConfig.noRetries
  ) {
    wireMockServer = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wireMockServer.start()

    web3jClient = Web3j.build(HttpService(URI("http://127.0.0.1:" + wireMockServer.port()).toURL().toString()))
    vertx = Vertx.vertx()
    logsClient = EthLogsSearcherImpl(
      vertx,
      ethApiClient = createEthApiClient(
        web3jClient = web3jClient,
        requestRetryConfig = retryConfig,
        vertx = vertx
      ),
      config = EthLogsSearcherImpl.Config(
        loopSuccessBackoffDelay = 1.milliseconds
      )
    )
  }

  private fun setupClientWithTestingJsonRpcServer(
    retryConfig: RetryConfig = RetryConfig.noRetries,
    subsetOfBlocksWithLogs: List<ULongRange>? = null
  ) {
    TestingJsonRpcServer = TestingJsonRpcServer(
      vertx = vertx,
      serverName = "fake-execution-layer-log-searcher",
      recordRequestsResponses = true
    )
    setUpFakeLogsServerToHandleEthLogs(TestingJsonRpcServer, subsetOfBlocksWithLogs)
    logsClient = EthLogsSearcherImpl(
      vertx = vertx,
      ethApiClient = createEthApiClient(
        vertx = vertx,
        web3jClient = Web3j.build(HttpService(URI("http://127.0.0.1:" + TestingJsonRpcServer.boundPort).toString())),
        requestRetryConfig = retryConfig
      ),
      config = EthLogsSearcherImpl.Config(
        loopSuccessBackoffDelay = 1.milliseconds
      ),
      log = LogManager.getLogger("test.case.Web3JLogsSearcher")
    )
  }

  private fun replyEthGetLogsWith(statusCode: Int, responseBody: String) {
    wireMockServer.stubFor(
      post(urlEqualTo("/"))
        .withRequestBody(containing("eth_getLogs"))
        .willReturn(
          aResponse()
            .withStatus(statusCode)
            .withBody(responseBody)
            .withHeader("Content-Type", "application/json")
        )
    )
  }

  @Test
  fun `when eth_getLogs returns json-rpc error shall return failed promise`() {
    setupClientWithWireMockServer()
    replyEthGetLogsWith(
      statusCode = 200,
      responseBody = """
        {
          "jsonrpc": "2.0",
          "error": {
            "code": -32000,
            "message": "Error: unable to retrieve logs"
          },
          "id": 1
        }
      """.trimIndent()
    )

    assertThatThrownBy {
      logsClient.getLogs(
        0UL.toBlockParameter(),
        20UL.toBlockParameter(),
        address = address,
        topics = emptyList()
      ).get()
    }
      .hasCauseInstanceOf(RuntimeException::class.java)
      .hasMessageContaining("json-rpc error: code=-32000 message=Error: unable to retrieve logs")
  }

  @Test
  fun `when eth_getLogs returns invalid json-rpc result shall return error instead of infinite retry`() {
    setupClientWithWireMockServer()
    replyEthGetLogsWith(
      statusCode = 200,
      responseBody = """
       {
          "jsonrpc": "2.0",
          "id": 1,
          "result": [{
                  "address": "0x508ca82df566dcd1b0de8296e70a96332cd644ec",
                  "blockHash": "0x216a74dcf",
                  "blockNumber": "0x864b52",
                  "data": "0x",
                  "logIndex": "0x0",
                  "removed": false,
                  "topics": ["0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"],
                  "transactionHash": "0xfcff12cba7002ec38f391f2452",
                  "transactionIndex": "0x0"
           }]
      }
      """.trimIndent()
    )

    assertThatThrownBy {
      logsClient.getLogs(
        0UL.toBlockParameter(),
        20UL.toBlockParameter(),
        address = address,
        topics = emptyList()
      ).get()
    }
  }

  @Test
  fun `when eth_getLogs request fails shall retry request until it succeeds`() {
    setupClientWithTestingJsonRpcServer(
      retryConfig = RetryConfig(
        backoffDelay = 1.milliseconds,
        maxRetries = 4u
      )
    )

    TestingJsonRpcServer.handle("eth_getLogs", { _ ->
      // simulate 2 failures
      log.debug("eth_getLogs callCount=${TestingJsonRpcServer.callCountByMethod("eth_getLogs")}")
      if (TestingJsonRpcServer.callCountByMethod("eth_getLogs") < 2) {
        throw JsonRpcError.internalError().asException()
      } else {
        generateLogsForBlockRange(fromBlock = 10, toBlock = 15)
      }
    })

    val getLogsFuture = logsClient.getLogs(
      0UL.toBlockParameter(),
      20UL.toBlockParameter(),
      address = address,
      topics = emptyList()
    )

    getLogsFuture.get().also { logs ->
      assertThat(logs).hasSize(6)
    }
  }

  @Test
  fun `when eth_getLogs gets an HTTP error shall return failed promise`() {
    setupClientWithWireMockServer()

    replyEthGetLogsWith(
      statusCode = 500,
      responseBody = "Internal Server Error"
    )

    assertThatThrownBy {
      logsClient.getLogs(
        0UL.toBlockParameter(),
        20UL.toBlockParameter(),
        address = address,
        topics = emptyList()
      ).get()
    }
      .hasCauseInstanceOf(org.web3j.protocol.exceptions.ClientConnectionException::class.java)
      .hasMessageContaining("Invalid response received: 500; Internal Server Error")
  }

  @Test
  fun `when eth_getLogs gets a DNS error shall return failed promise`() {
    val randomHostname = "nowhere-${Random.nextBytes(20).encodeHex()}.local"
    web3jClient = Web3j.build(HttpService("http://$randomHostname:1234"))
    vertx = Vertx.vertx()
    logsClient = EthLogsSearcherImpl(
      vertx,
      createEthApiClient(
        web3jClient,
        requestRetryConfig = RetryConfig.noRetries,
        vertx = null
      ),
      config = EthLogsSearcherImpl.Config(
        loopSuccessBackoffDelay = 1.milliseconds
      )
    )

    assertThatThrownBy {
      logsClient.getLogs(
        0UL.toBlockParameter(),
        20UL.toBlockParameter(),
        address = address,
        topics = emptyList()
      ).get()
    }
      .hasCauseInstanceOf(java.net.UnknownHostException::class.java)
      .hasMessageContaining("$randomHostname")
    vertx.close()
  }

  private fun shallContinueToSearch(
    ethLog: EthLog,
    targetNumber: ULong
  ): SearchDirection? {
    val number = ULong.fromHexString(ethLog.topics[1].encodeHex())
    val direction = when {
      number < targetNumber -> SearchDirection.FORWARD
      number > targetNumber -> SearchDirection.BACKWARD
      else -> null
    }
    log.debug(
      "EthLog{blockNumber={} number={}} targetNumber={} direction={}",
      ethLog.blockNumber,
      number,
      targetNumber,
      direction
    )
    return direction
  }

  @Test
  fun `findLogs searches and returns log when found`() {
    setupClientWithTestingJsonRpcServer()

    (100..200)
      .forEach { number ->
        logsClient.findLog(
          fromBlock = 100UL.toBlockParameter(),
          toBlock = 200UL.toBlockParameter(),
          address = address,
          topics = listOf("0xffaabbcc"),
          chunkSize = 10,
          shallContinueToSearch = { ethLog ->
            shallContinueToSearch(ethLog, targetNumber = number.toULong())
          }
        )
          .get()
          .also { log ->
            assertThat(log).isNotNull()
            assertThat(log!!.topics[1].encodeHex()).isEqualTo(number.toULong().toHexStringUInt256())
          }
      }
  }

  @Test
  fun `findLogs searches L1 and returns null when not found - target before fromBlock`() {
    setupClientWithTestingJsonRpcServer()
    val logsEvaluated = mutableListOf<ULong>()

    logsClient.findLog(
      fromBlock = 100UL.toBlockParameter(),
      toBlock = 200UL.toBlockParameter(),
      address = address,
      topics = listOf("0xffaabbcc"),
      chunkSize = 10,
      shallContinueToSearch = { ethLog ->
        logsEvaluated.add(ethLog.blockNumber)
        shallContinueToSearch(ethLog, targetNumber = 89UL)
      }
    )
      .get()
      .also { log ->
        assertThat(log).isNull()
        assertThat(TestingJsonRpcServer.callCountByMethod("eth_getLogs")).isBetween(1, 4)
        assertThat(logsEvaluated).hasSameElementsAs(logsEvaluated.toSet())
      }
  }

  // REFERENCE:
  @Test
  fun `findLogs searches L1 and returns null when not found - target expected in chunk that has no logs`() {
    setupClientWithTestingJsonRpcServer(
      subsetOfBlocksWithLogs = listOf(100UL..109UL, 150UL..159UL)
    )
    val logsEvaluated = mutableListOf<ULong>()

    logsClient.findLog(
      fromBlock = 100UL.toBlockParameter(),
      toBlock = 200UL.toBlockParameter(),
      address = address,
      topics = listOf("0xffaabbcc"),
      chunkSize = 10,
      shallContinueToSearch = { ethLog ->
        shallContinueToSearch(ethLog, targetNumber = 120UL)
      }
    )
      .get()
      .also { log ->
        assertThat(log).isNull()
        assertThat(logsEvaluated).hasSameElementsAs(logsEvaluated.toSet())
      }
  }

  @Test
  fun `findLogs searches L1 and returns null when not found - target is after toBlock`() {
    setupClientWithTestingJsonRpcServer(
      subsetOfBlocksWithLogs = listOf(100UL..109UL, 150UL..200UL)
    )
    val logsEvaluated = mutableListOf<ULong>()

    logsClient.findLog(
      fromBlock = 100UL.toBlockParameter(),
      toBlock = 200UL.toBlockParameter(),
      address = address,
      topics = listOf("0xffaabbcc"),
      chunkSize = 10,
      shallContinueToSearch = { ethLog ->
        shallContinueToSearch(ethLog, targetNumber = 250UL)
      }
    )
      .get()
      .also { log ->
        assertThat(log).isNull()
        assertThat(TestingJsonRpcServer.callCountByMethod("eth_getLogs")).isBetween(1, 4)
        assertThat(logsEvaluated).hasSameElementsAs(logsEvaluated.toSet())
      }
  }

  // REFERECE 2:
  @Test
  fun `findLogs searches L1 and returns item when - target found an chunk in the middle`() {
    setupClientWithTestingJsonRpcServer(
      subsetOfBlocksWithLogs = listOf(10UL..19UL, 30UL..37UL, 50UL..100UL)
    )
    val logsEvaluated = mutableListOf<ULong>()
    logsClient.findLog(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 100UL.toBlockParameter(),
      address = address,
      topics = listOf("0xffaabbcc"),
      chunkSize = 5,
      shallContinueToSearch = { ethLog ->
        shallContinueToSearch(ethLog, targetNumber = 35UL)
      }
    )
      .get()
      .also { log ->
        assertThat(log).isNotNull()
        assertThat(ULong.fromHexString(log!!.topics[1].encodeHex())).isEqualTo(35UL)
        assertThat(TestingJsonRpcServer.callCountByMethod("eth_getLogs")).isBetween(1, 11)
        assertThat(logsEvaluated).hasSameElementsAs(logsEvaluated.toSet())
      }
  }

  companion object {
    private fun generateLogsForBlockRange(
      fromBlock: Int,
      toBlock: Int,
      stepSize: Int = 1,
      topic: String = "0x"
    ): List<Map<String, Any>> {
      return (fromBlock..toBlock step stepSize)
        .map {
          generateLogJson(
            blockNumber = it,
            topic = topic
          )
        }
    }

    private fun generateLogJson(
      blockNumber: Int,
      topic: String = "0x",
      transactionHash: String = "0x"
    ): Map<String, Any> {
      val topics = listOf(
        topic,
        blockNumber.toULong().toHexStringUInt256()
      )
      return mapOf(
        "address" to "0x",
        "blockHash" to blockNumber.toULong().toHexStringUInt256(),
        "blockNumber" to blockNumber.toULong().toHexString(),
        "data" to "0x",
        "logIndex" to "0x0",
        "removed" to false,
        "topics" to topics,
        "transactionHash" to transactionHash,
        "transactionIndex" to "0x0"
      )
    }

    internal fun generateLogs(
      blocksWithLogs: List<ULongRange>,
      filter: EthGetLogsRequest
    ): List<Map<String, Any>> {
      return generateEffectiveIntervals(blocksWithLogs, filter.fromBlock, filter.toBlock)
        // .also {
        // println(
        // "filter=${CommonDomainFunctions.blockIntervalString(filter.fromBlock, filter.toBlock)} logs=$it"
        // )
        // }
        .flatMap {
          generateLogsForBlockRange(it.first.toInt(), it.last.toInt(), topic = filter.topics[0])
        }
    }

    @Suppress("UNCHECKED_CAST")
    private fun parseEthLogsRequest(request: JsonRpcRequest): EthGetLogsRequest {
      /** eth_getLogs request example
       {
       "jsonrpc": "2.0",
       "method": "eth_getLogs",
       "params": [{
       "topics": ["0xa0262dc79e4ccb71ceac8574ae906311ae338aa4a2044fd4ec4b99fad5ab60cb", "0xa0262dc79e4ccb71ceac8574ae906311ae338aa4a2044fd4ec4b99fad5ab60ff"],
       "fromBlock": "earliest",
       "toBlock": "latest",
       "address": ["0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9"]
       }],
       }*/
      val logsFilter = (request.params as List<Any>)[0] as Map<String, Any>
      val fromBlock = ULong.fromHexString(logsFilter["fromBlock"] as String)
      val toBlock = ULong.fromHexString(logsFilter["toBlock"] as String)
      val topics = logsFilter["topics"] as List<String>
      return EthGetLogsRequest(
        fromBlock = fromBlock,
        toBlock = toBlock,
        topics = topics,
        address = logsFilter["address"] as List<String>
      )
    }

    private fun setUpFakeLogsServerToHandleEthLogs(
      TestingJsonRpcServer: TestingJsonRpcServer,
      subsetOfBlocksWithLogs: List<ULongRange>?
    ) {
      TestingJsonRpcServer.apply {
        this.handle("eth_getLogs", { request ->
          val filter = parseEthLogsRequest(request)
          subsetOfBlocksWithLogs
            ?.let {
              generateLogs(subsetOfBlocksWithLogs, filter)
            } ?: generateLogsForBlockRange(
            fromBlock = filter.fromBlock.toInt(),
            toBlock = filter.toBlock.toInt(),
            topic = filter.topics[0]
          )
        })
      }
    }
  }
}
