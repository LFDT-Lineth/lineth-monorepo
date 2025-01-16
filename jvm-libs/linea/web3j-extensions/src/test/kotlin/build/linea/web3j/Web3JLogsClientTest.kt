package build.linea.web3j

import io.vertx.core.Vertx
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.argumentCaptor
import org.mockito.kotlin.atMost
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import org.web3j.abi.datatypes.Event
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthLog
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class Web3JLogsClientTest {
  private lateinit var web3jClient: Web3j
  private lateinit var logsClient: Web3JLogsClient
  private lateinit var vertx: Vertx

  @BeforeEach
  fun setup() {
    web3jClient = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
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

  private fun logObject(blockNumber: BigInteger) = EthLog.LogObject().apply {
    setBlockNumber("0x" + blockNumber.toString(16))
  }

  private fun String.hexToInt() = BigInteger(this.drop(2), 16).toInt()

  @Test
  fun `findLastLog should return the last found log`() {
    val expectedLog = logObject(BigInteger.TEN)
    val otherEarlierLog = logObject(BigInteger.TWO)

    whenever(web3jClient.ethGetLogs(any<EthFilter>()).sendAsync())
      .thenAnswer { SafeFuture.completedFuture(EthLog()) }
      .thenAnswer { SafeFuture.failedFuture<EthLog>(RuntimeException("TEST FORCED ERROR: Failed to get logs")) }
      .thenAnswer { SafeFuture.completedFuture(EthLog()) }
      .thenAnswer {
        SafeFuture.completedFuture(
          EthLog().apply {
            result = listOf(expectedLog, otherEarlierLog)
          }
        )
      }

    val result = logsClient.findLastLog(
      upToBlockNumberInclusive = 1000, // "0x3e8"
      address = "0x1234",
      lookbackBlockNumberLimitInclusive = 750,
      eventsFilter = emptyList<Event>()
    ).get()
    assertThat(result).isEqualTo(expectedLog)

    val captor = argumentCaptor<EthFilter>()

    verify(web3jClient, atMost(5)).ethGetLogs(captor.capture())
    val captures = captor.allValues.filterNotNull()
    // 1st call, success, empty list
    assertThat(captures[0].fromBlock.value.hexToInt()).isEqualTo(900)
    assertThat(captures[0].toBlock.value.hexToInt()).isEqualTo(1000)
    assertThat(captures[0].address).isEqualTo(listOf("0x1234"))
    assertThat(captures[0].topics.size).isEqualTo(1)

    // 2nd call, failed with reject promise shall be retried
    assertThat(captures[1].fromBlock.value.hexToInt()).isEqualTo(800)
    assertThat(captures[1].toBlock.value.hexToInt()).isEqualTo(899)
    assertThat(captures[1].address).isEqualTo(listOf("0x1234"))
    assertThat(captures[1].topics.size).isEqualTo(1)

    // 3rd call, retry, success
    assertThat(captures[2].fromBlock.value.hexToInt()).isEqualTo(800)
    assertThat(captures[2].toBlock.value.hexToInt()).isEqualTo(899)
    assertThat(captures[2].address).isEqualTo(listOf("0x1234"))
    assertThat(captures[2].topics.size).isEqualTo(1)

    // 3rd call, retry, success
    assertThat(captures[3].fromBlock.value.hexToInt()).isEqualTo(750)
    assertThat(captures[3].toBlock.value.hexToInt()).isEqualTo(799)
    assertThat(captures[3].address).isEqualTo(listOf("0x1234"))
    assertThat(captures[3].topics.size).isEqualTo(1)

    assertThat(captures.size).isEqualTo(4)
  }
}
