package lineth.contract

import io.vertx.core.Vertx
import lineth.contract.events.Upgraded
import lineth.domain.EthLog
import lineth.ethapi.EthLogsSearcherImpl
import lineth.ethapi.FakeEthApiClient
import lineth.kotlin.decodeHex
import lineth.kotlin.toHexStringUInt256
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.random.Random

class EventBasedContractDeploymentBlockNumberProviderTest {
  private val contractAddress = "0x" + "aa".repeat(20)
  private lateinit var vertx: Vertx
  private lateinit var ethApiClient: FakeEthApiClient
  private lateinit var provider: EventBasedContractDeploymentBlockNumberProvider

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx()
    ethApiClient = FakeEthApiClient()
    provider = EventBasedContractDeploymentBlockNumberProvider(
      ethLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = ethApiClient),
      contractAddress = contractAddress,
      // small so the forward scan spans multiple bounded chunks instead of one query
      ethLogsSearchMaxBlockRange = 1_000u,
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  @Test
  fun `getDeploymentBlock returns the earliest Upgraded event block across bounded chunks`() {
    // Two upgrades; the deployment block is the first one, several chunks past genesis.
    ethApiClient.setLogs(
      listOf(
        upgradedLog(l1Block = 3_000UL),
        upgradedLog(l1Block = 4_500UL),
      ),
    )
    ethApiClient.setLatestBlockTag(5_000UL)

    assertThat(provider.getDeploymentBlock().get()).isEqualTo(3_000UL)
  }

  @Test
  fun `getDeploymentBlock throws when no Upgraded event exists`() {
    ethApiClient.setLatestBlockTag(5_000UL)

    assertThatThrownBy { provider.getDeploymentBlock().get() }
      .hasRootCauseInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("Upgraded event not found")
  }

  private fun upgradedLog(l1Block: ULong): EthLog =
    EthLog(
      removed = false,
      logIndex = 0UL,
      transactionIndex = 0UL,
      transactionHash = Random.nextBytes(32),
      blockHash = l1Block.toHexStringUInt256().decodeHex(),
      blockNumber = l1Block,
      address = contractAddress.decodeHex(),
      data = ByteArray(0),
      topics = listOf(Upgraded.topic.decodeHex()),
    )
}
