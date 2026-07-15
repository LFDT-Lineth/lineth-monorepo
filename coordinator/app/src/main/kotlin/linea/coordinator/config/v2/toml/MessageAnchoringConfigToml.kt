package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.MessageAnchoringConfig
import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class MessageAnchoringConfigToml(
  @param:ConfigDoc(description = "Whether L1 to L2 message anchoring is disabled.", default = "false")
  var disabled: Boolean = false,
  @param:ConfigDoc(description = "Interval between message anchoring ticks.", default = "PT10S")
  val anchoringTickInterval: Duration = 10.seconds,
  @param:ConfigDoc(description = "Maximum number of messages buffered awaiting anchoring.", default = "10000")
  val messageQueueCapacity: UInt = 10_000u,
  @param:ConfigDoc(
    description = "Maximum number of messages anchored in a single L2 transaction.",
    default = "100",
  )
  val maxMessagesToAnchorPerL2Transaction: UInt = 100u,
  @param:ConfigDoc(
    description = "L1 endpoint used for anchoring. Falls back to defaults.l1-endpoint.",
    example = "http://l1-el-node:8545",
  )
  val l1Endpoint: URL? = null, // shall default to L1 endpoint
  @param:ConfigDoc(
    description = "L1 block tag up to which message events are read (e.g. FINALIZED, LATEST, or a number).",
    default = "FINALIZED",
  )
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  @param:ConfigSection("Retry policy for L1 anchoring requests.")
  val l1RequestRetries: RequestRetriesToml = RequestRetriesToml.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  @param:ConfigSection("L1 event scraping (log polling) settings for message anchoring.")
  val l1EventScraping: L1EventScrapping = L1EventScrapping(),
  @param:ConfigDoc(
    description = "L2 endpoint used for anchoring. Falls back to defaults.l2-endpoint.",
    example = "http://sequencer:8545",
  )
  val l2Endpoint: URL? = null,
  @param:ConfigDoc(
    description = "L2 block tag up to which anchoring state is read (e.g. LATEST, FINALIZED, or a number).",
    default = "LATEST",
  )
  val l2HighestBlockTag: BlockParameter = BlockParameter.Tag.LATEST,
  @param:ConfigSection("Retry policy for L2 anchoring requests.")
  val l2RequestRetries: RequestRetriesToml = RequestRetriesToml(
    maxRetries = null,
    backoffDelay = 1.seconds,
    timeout = 8.seconds,
    failuresWarningThreshold = 3u,
  ),
  @param:ConfigSection("Signer used to sign anchoring transactions.")
  val signer: SignerConfigToml,
  @param:ConfigSection("Gas settings for anchoring transactions.")
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
    @param:ConfigDoc(description = "Interval between L1 log polling attempts.", default = "PT6S")
    val pollingInterval: Duration = 6.seconds,
    @param:ConfigDoc(description = "Timeout for each L1 log polling request.", default = "PT5S")
    val pollingTimeout: Duration = 5.seconds,
    @param:ConfigDoc(
      description = "Backoff delay after a successful eth_getLogs search before the next one.",
      default = "PT0.001S",
    )
    val ethLogsSearchSuccessBackoffDelay: Duration = 1.milliseconds,
    @param:ConfigDoc(
      description = "Number of blocks scanned per eth_getLogs chunk.",
      default = "1000",
    )
    val ethLogsSearchBlockChunkSize: UInt = 1000u,
    @param:ConfigDoc(
      description = "Maximum block range covered by a single eth_getLogs search.",
      default = "10000",
    )
    val ethLogsSearchMaxBlockRange: UInt = 10_000u,
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
      require(ethLogsSearchMaxBlockRange >= 1u) {
        "ethLogsSearchMaxBlockRange=$ethLogsSearchMaxBlockRange must be equal or greater than 1"
      }
    }
  }

  data class GasConfig(
    @param:ConfigDoc(
      description = "Cap on the EIP-1559 max fee per gas for anchoring transactions (wei).",
      default = "100000000000",
    )
    val maxFeePerGasCap: ULong = 100_000_000_000uL, // 100 gwei
    @param:ConfigDoc(description = "Gas limit for anchoring transactions.", default = "2500000")
    val gasLimit: ULong = 2_500_000uL,
    @param:ConfigDoc(
      description = "Number of blocks of fee history used to price anchoring transactions.",
      default = "4",
    )
    val feeHistoryBlockCount: UInt = 4u,
    @param:ConfigDoc(
      description = "Reward percentile (1-100) of fee history used to price anchoring transactions.",
      default = "15",
    )
    val feeHistoryRewardPercentile: UInt = 15u,
  )

  fun reified(l1DefaultEndpoint: URL?, l2DefaultEndpoint: URL?): MessageAnchoringConfig {
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
        ethLogsSearchMaxBlockRange = l1EventScraping.ethLogsSearchMaxBlockRange,
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
