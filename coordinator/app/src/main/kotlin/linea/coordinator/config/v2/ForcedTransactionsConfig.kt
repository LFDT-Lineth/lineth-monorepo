package linea.coordinator.config.v2

import linea.domain.BlockParameter
import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class ForcedTransactionsConfig(
  override val disabled: Boolean = false,
  val l1Endpoint: URL,
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l1RequestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val processingTickInterval: Duration = 2.minutes,
  val processingDelay: Duration = Duration.ZERO,
  val l1EventScraping: L1EventScraping = L1EventScraping(),
  val processingBatchSize: UInt = 10u,
) : FeatureToggle {
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
}
