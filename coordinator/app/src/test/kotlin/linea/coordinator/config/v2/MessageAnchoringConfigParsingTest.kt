package linea.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.toml.MessageAnchoringConfigToml
import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.SignerConfigToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.domain.BlockParameter
import linea.kotlin.decodeHex
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class MessageAnchoringConfigParsingTest {
  companion object {
    val toml =
      """
      [message-anchoring]
      disabled = false
      anchoring-tick-interval = "PT13S"
      message-queue-capacity = 12_300
      max-messages-to-anchor-per-l2-transaction = 86
      l1-endpoint = "http://l1-el-node:8545"
      l2-endpoint = "http://sequencer:8545"
      l1-highest-block-tag="FINALIZED"
      l2-highest-block-tag="LATEST" # optional, default to LATEST it shall not be necessary as Linea has instant finality

      [message-anchoring.l1-request-retries]
      max-retries = 4
      backoff-delay = "PT1S"
      timeout = "PT6S"
      failures-warning-threshold = 2

      [message-anchoring.l2-request-retries]
      max-retries = 5
      backoff-delay = "PT0.1S"
      timeout = "PT10S"
      failures-warning-threshold = 3

      [message-anchoring.l1-event-scraping]
      polling-interval = "PT1S"
      polling-timeout = "PT50S"
      eth-logs-search-success-backoff-delay = "PT0.1S"
      eth-logs-search-block-chunk-size = 123

      [message-anchoring.gas]
      max-fee-per-gas-cap = 100000000000
      gas-limit = 10000000
      fee-history-block-count = 4
      fee-history-reward-percentile = 15

      [message-anchoring.signer]
      # Web3j/Web3signer
      type = "Web3j"

      [message-anchoring.signer.web3j]
      private-key = "0x0000000000000000000000000000000000000000000000000000000000000001"

      [message-anchoring.signer.web3signer]
      endpoint = "http://web3signer:9000"
      max-pool-size = 11
      keep-alive = true
      public-key = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"
      """.trimIndent()

    val config =
      MessageAnchoringConfigToml(
        disabled = false,
        l1Endpoint = "http://l1-el-node:8545".toURL(),
        l2Endpoint = "http://sequencer:8545".toURL(),
        l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
        l2HighestBlockTag = BlockParameter.Tag.LATEST,
        anchoringTickInterval = 13.seconds,
        messageQueueCapacity = 12_300u,
        maxMessagesToAnchorPerL2Transaction = 86u,
        l1EventScraping =
        MessageAnchoringConfigToml.L1EventScrapping(
          pollingInterval = 1.seconds,
          pollingTimeout = 50.seconds,
          ethLogsSearchSuccessBackoffDelay = 100.milliseconds,
          ethLogsSearchBlockChunkSize = 123u,
        ),
        l1RequestRetries =
        RequestRetriesToml(
          maxRetries = 4u,
          backoffDelay = 1.seconds,
          timeout = 6.seconds,
          failuresWarningThreshold = 2u,
        ),
        l2RequestRetries =
        RequestRetriesToml(
          maxRetries = 5u,
          backoffDelay = 100.milliseconds,
          timeout = 10.seconds,
          failuresWarningThreshold = 3u,
        ),
        gas =
        MessageAnchoringConfigToml.GasConfig(
          maxFeePerGasCap = 100_000_000_000u,
          gasLimit = 10_000_000u,
          feeHistoryBlockCount = 4u,
          feeHistoryRewardPercentile = 15u,
        ),
        signer =
        SignerConfigToml(
          type = SignerConfigToml.SignerType.WEB3J,
          web3j =
          SignerConfigToml.Web3jConfig(
            privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000001"),
          ),
          web3signer =
          SignerConfigToml.Web3SignerConfig(
            endpoint = "http://web3signer:9000".toURL(),
            maxPoolSize = 11,
            keepAlive = true,
            publicKey =
            (
              "0000000000000000000000000000000000000000000000000000000000000000" +
                "0000000000000000000000000000000000000000000000000000000000000001"
              ).decodeHex(),
            tls = null,
          ),
        ),
      )

    val tomlMinimal =
      """
      [message-anchoring]
      [message-anchoring.signer]
      type = "Web3j"
      [message-anchoring.signer.web3j]
      private-key = "0x0000000000000000000000000000000000000000000000000000000000000001"
      """.trimIndent()

    val configMinimal =
      MessageAnchoringConfigToml(
        disabled = false,
        anchoringTickInterval = 10.seconds,
        messageQueueCapacity = 10_000u,
        maxMessagesToAnchorPerL2Transaction = 100u,
        l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
        l2HighestBlockTag = BlockParameter.Tag.LATEST,
        l1Endpoint = null,
        l2Endpoint = null,
        l1EventScraping =
        MessageAnchoringConfigToml.L1EventScrapping(
          pollingInterval = 6.seconds,
          pollingTimeout = 5.seconds,
          ethLogsSearchSuccessBackoffDelay = 1.milliseconds,
          ethLogsSearchBlockChunkSize = 1000u,
        ),
        l1RequestRetries =
        RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          timeout = null,
          failuresWarningThreshold = 3u,
        ),
        l2RequestRetries =
        RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          timeout = 8.seconds,
          failuresWarningThreshold = 3u,
        ),
        gas =
        MessageAnchoringConfigToml.GasConfig(
          maxFeePerGasCap = 100_000_000_000u,
          gasLimit = 2_500_000u,
          feeHistoryBlockCount = 4u,
          feeHistoryRewardPercentile = 15u,
        ),
        signer =
        SignerConfigToml(
          type = SignerConfigToml.SignerType.WEB3J,
          web3j =
          SignerConfigToml.Web3jConfig(
            privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000001"),
          ),
          web3signer = null,
        ),
      )
  }

  data class WrapperConfig(
    val messageAnchoring: MessageAnchoringConfigToml,
  )

  @Test
  fun `should parse message anchoring full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).messageAnchoring)
      .isEqualTo(config)
  }

  @Test
  fun `should parse message anchoring minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).messageAnchoring)
      .isEqualTo(configMinimal)
  }
}
