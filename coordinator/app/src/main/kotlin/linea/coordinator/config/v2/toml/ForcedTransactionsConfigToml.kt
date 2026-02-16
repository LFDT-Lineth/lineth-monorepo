package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.ForcedTransactionsConfig
import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class ForcedTransactionsConfigToml(
  var disabled: Boolean = false,
  val l1Endpoint: URL? = null, // shall default to L1 endpoint
  val l1RequestRetries: RequestRetriesToml? = null,
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val processingTickInterval: Duration = 2.minutes,
  val processingDelay: Duration = Duration.ZERO,
  val processingBatchSize: UInt = 10u,
  val l1EventScraping: L1EventScraping = L1EventScraping(),
) {
  init {
    require(processingTickInterval >= 1.milliseconds) {
      "processingSendTickInterval=$processingTickInterval must be equal or greater than 1ms"
    }
    require(processingDelay >= Duration.ZERO) {
      "processingDelay=$processingDelay must be equal or greater than 0ms"
    }
  }

  data class L1EventScraping(
    val pollingInterval: Duration = 12.seconds,
    val pollingTimeout: Duration = 5.seconds,
    val ethLogsSearchSuccessBackoffDelay: Duration = 1.milliseconds,
    val ethLogsSearchBlockChunkSize: UInt = 1000u,
  ) {
    init {
      require(pollingInterval >= 1.milliseconds) {
        "pollingInterval=$pollingInterval must be equal or greater than 1ms"
      }
      require(pollingTimeout >= 1.milliseconds) {
        "pollingTimeout=$pollingTimeout must be equal or greater than 1ms"
      }
      require(ethLogsSearchSuccessBackoffDelay >= 1.milliseconds) {
        "ethLogsSearchSuccessBackoffDelay=$ethLogsSearchSuccessBackoffDelay must be equal or greater than 1ms"
      }
      require(ethLogsSearchBlockChunkSize >= 1u) {
        "ethLogsSearchBlockChunkSize=$ethLogsSearchBlockChunkSize must be equal or greater than 1"
      }
    }
  }

  fun reified(
    l1DefaultEndpoint: URL?,
    l1DefaultRequestRetries: RequestRetriesToml,
  ): ForcedTransactionsConfig {
    return ForcedTransactionsConfig(
      disabled = disabled,
      l1Endpoint = l1Endpoint ?: l1DefaultEndpoint ?: throw AssertionError("l1Endpoint must be set"),
      l1HighestBlockTag = l1HighestBlockTag,
      l1RequestRetries = l1RequestRetries?.asDomain ?: l1DefaultRequestRetries.asDomain,
      processingTickInterval = processingTickInterval,
      processingDelay = processingDelay,
      processingBatchSize = processingBatchSize,
      l1EventScraping = ForcedTransactionsConfig.L1EventScraping(
        pollingInterval = l1EventScraping.pollingInterval,
        pollingTimeout = l1EventScraping.pollingTimeout,
        ethLogsSearchSuccessBackoffDelay = l1EventScraping.ethLogsSearchSuccessBackoffDelay,
        ethLogsSearchBlockChunkSize = l1EventScraping.ethLogsSearchBlockChunkSize,
      ),
    )
  }
}
