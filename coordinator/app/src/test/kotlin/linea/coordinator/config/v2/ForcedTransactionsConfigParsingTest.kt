package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.ForcedTransactionsConfigToml
import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.domain.BlockParameter
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class ForcedTransactionsConfigParsingTest {
  companion object {
    val toml =
      """
      [forced-transactions]
      disabled = false
      l1-endpoint = "http://l1-el-node:8545"
      l1-highest-block-tag = "FINALIZED"
      sequencer-endpoint = "http://sequencer-node:8545"
      processing-tick-interval = "PT5M"
      processing-delay = "PT30S"
      processing-batch-size = 20
      invalidity-proof-check-interval = "PT10M"

      [forced-transactions.l1-request-retries]
      max-retries = 4
      backoff-delay = "PT1S"
      timeout = "PT6S"
      failures-warning-threshold = 2

      [forced-transactions.sequencer-request-retries]
      max-retries = 3
      backoff-delay = "PT2S"
      timeout = "PT4S"
      failures-warning-threshold = 1

      [forced-transactions.l1-event-scraping]
      polling-interval = "PT2S"
      polling-timeout = "PT10S"
      eth-logs-search-success-backoff-delay = "PT0.5S"
      eth-logs-search-block-chunk-size = 500
      """.trimIndent()

    val config =
      ForcedTransactionsConfigToml(
        disabled = false,
        l1Endpoint = "http://l1-el-node:8545".toURL(),
        l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
        processingTickInterval = 5.minutes,
        processingDelay = 30.seconds,
        processingBatchSize = 20u,
        invalidityProofCheckInterval = 10.minutes,
        sequencerEndpoint = "http://sequencer-node:8545".toURL(),
        l1RequestRetries =
        RequestRetriesToml(
          maxRetries = 4u,
          backoffDelay = 1.seconds,
          timeout = 6.seconds,
          failuresWarningThreshold = 2u,
        ),
        sequencerRequestRetries =
        RequestRetriesToml(
          maxRetries = 3u,
          backoffDelay = 2.seconds,
          timeout = 4.seconds,
          failuresWarningThreshold = 1u,
        ),
        l1EventScraping =
        ForcedTransactionsConfigToml.L1EventScraping(
          pollingInterval = 2.seconds,
          pollingTimeout = 10.seconds,
          ethLogsSearchSuccessBackoffDelay = 500.milliseconds,
          ethLogsSearchBlockChunkSize = 500u,
        ),
      )

    val tomlMinimal =
      """
      [forced-transactions]
      sequencer-endpoint = "http://sequencer-node:8545"
      """.trimIndent()

    val configMinimal =
      ForcedTransactionsConfigToml(
        disabled = false,
        l1Endpoint = null,
        l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
        processingTickInterval = 2.minutes,
        processingDelay = 0.seconds,
        processingBatchSize = 10u,
        invalidityProofCheckInterval = 2.minutes,
        sequencerEndpoint = "http://sequencer-node:8545".toURL(),
        l1RequestRetries = null,
        sequencerRequestRetries = null,
        l1EventScraping =
        ForcedTransactionsConfigToml.L1EventScraping(
          pollingInterval = 12.seconds,
          pollingTimeout = 5.seconds,
          ethLogsSearchSuccessBackoffDelay = 1.milliseconds,
          ethLogsSearchBlockChunkSize = 1000u,
        ),
      )

    val tomlUndefined =
      """
      # totally disabled, not event section present
      """.trimIndent()

    val tomlMinimalDisabled =
      """
      [forced-transactions]
      disabled = true
      sequencer-endpoint = "http://sequencer-node:8545"
      """.trimIndent()
  }

  data class WrapperConfig(
    val forcedTransactions: ForcedTransactionsConfigToml? = null,
  )

  @Test
  fun `should parse forced transactions full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).forcedTransactions)
      .isEqualTo(config)
  }

  @Test
  fun `should parse forced transactions minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).forcedTransactions)
      .isEqualTo(configMinimal)
  }

  @Test
  fun `should parse forced transactions minimal config - disabled`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimalDisabled).forcedTransactions)
      .isEqualTo(configMinimal.copy(disabled = true))
  }

  @Test
  fun `should allow undefined forced transactions config and take the defaults`() {
    assertThat(parseConfig<WrapperConfig>(tomlUndefined).forcedTransactions)
      .isNull()
  }
}
