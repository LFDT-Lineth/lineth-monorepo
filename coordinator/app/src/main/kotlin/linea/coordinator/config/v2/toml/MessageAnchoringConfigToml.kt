package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.MessageAnchoringConfig
import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class MessageAnchoringConfigToml(
  var disabled: Boolean = false,
  val anchoringTickInterval: Duration = 10.seconds,
  val messageQueueCapacity: UInt = 10_000u,
  val maxMessagesToAnchorPerL2Transaction: UInt = 100u,
  val l1Endpoint: URL? = null, // shall default to L1 endpoint
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l1RequestRetries: RequestRetriesToml = RequestRetriesToml.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val l1EventScraping: L1EventScrapping = L1EventScrapping(),
  val l2Endpoint: URL? = null,
  val l2HighestBlockTag: BlockParameter = BlockParameter.Tag.LATEST,
  val l2RequestRetries: RequestRetriesToml = RequestRetriesToml(
    maxRetries = null,
    backoffDelay = 1.seconds,
    timeout = 8.seconds,
    failuresWarningThreshold = 3u,
  ),
  val signer: SignerConfigToml,
  val gas: GasConfig = GasConfig(),
) {
  init {
    require(messageQueueCapacity >= 1u) {
      "messageQueueCapacity=$messageQueueCapacity be equal or greater than 1"
    }
    require(maxMessagesToAnchorPerL2Transaction >= 1u) {
      "maxMessagesToAnchorPerL2Transaction=$maxMessagesToAnchorPerL2Transaction be equal or greater than 1"
    }

    require(anchoringTickInterval >= 1.milliseconds) {
      "anchoringTickInterval must be equal or greater than 1ms"
    }
  }

  data class L1EventScrapping(
    val pollingInterval: Duration = 6.seconds,
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

  fun reified(
    l1DefaultEndpoint: URL?,
    l2DefaultEndpoint: URL?,
  ): MessageAnchoringConfig {
    return MessageAnchoringConfig(
      disabled = disabled,
      l1Endpoint = l1Endpoint ?: l1DefaultEndpoint ?: throw AssertionError("l1Endpoint must be set"),
      l2Endpoint = l2Endpoint ?: l2DefaultEndpoint ?: throw AssertionError("l2Endpoint must be set"),
      l1HighestBlockTag = l1HighestBlockTag,
      l2HighestBlockTag = l2HighestBlockTag,
      l1RequestRetries = l1RequestRetries.asDomain,
      l2RequestRetries = l2RequestRetries.asDomain,
      l1EventScrapping = MessageAnchoringConfig.L1EventScrapping(
        pollingInterval = l1EventScraping.pollingInterval,
        pollingTimeout = l1EventScraping.pollingTimeout,
        ethLogsSearchSuccessBackoffDelay = l1EventScraping.ethLogsSearchSuccessBackoffDelay,
        ethLogsSearchBlockChunkSize = l1EventScraping.ethLogsSearchBlockChunkSize,
      ),
      anchoringTickInterval = anchoringTickInterval,
      messageQueueCapacity = messageQueueCapacity,
      maxMessagesToAnchorPerL2Transaction = maxMessagesToAnchorPerL2Transaction,
      signer = signer.reified(),
      gas = MessageAnchoringConfig.GasConfig(
        maxFeePerGasCap = gas.maxFeePerGasCap,
        gasLimit = gas.gasLimit,
        feeHistoryBlockCount = gas.feeHistoryBlockCount,
        feeHistoryRewardPercentile = gas.feeHistoryRewardPercentile,
      ),
    )
  }
}
