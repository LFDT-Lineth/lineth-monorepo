package linea.coordinator.config.v2

import linea.domain.BlockParameter
import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class MessageAnchoringConfig(
  override val disabled: Boolean = false,
  val l1Endpoint: URL,
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l1RequestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val l1EventScrapping: L1EventScrapping = L1EventScrapping(),
  val l2Endpoint: URL,
  val l2HighestBlockTag: BlockParameter = BlockParameter.Tag.LATEST,
  val l2RequestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val anchoringTickInterval: Duration = 2.seconds,
  val messageQueueCapacity: UInt = 10_000u,
  val maxMessagesToAnchorPerL2Transaction: UInt = 100u,
  val signer: SignerConfig,
  val gas: GasConfig = GasConfig(),
) : FeatureToggle {
  init {
    require(messageQueueCapacity >= 1u) {
      "messageQueueCapacity=$messageQueueCapacity must be equal or greater than 1"
    }
    require(maxMessagesToAnchorPerL2Transaction >= 1u) {
      "maxMessagesToAnchorPerL2Transaction=$maxMessagesToAnchorPerL2Transaction be equal or greater than 1"
    }
    require(anchoringTickInterval >= 1.milliseconds) {
      "anchoringTickInterval must be equal or greater than 1ms"
    }
  }

  data class L1EventScrapping(
    val pollingInterval: Duration = 2.seconds,
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

  data class GasConfig(
    val maxFeePerGasCap: ULong = 100_000_000_000uL, // 100 gwei
    val gasLimit: ULong = 2_500_000uL,
    val feeHistoryBlockCount: UInt = 4u,
    val feeHistoryRewardPercentile: UInt = 15u,
  )
}
