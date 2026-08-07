package lineth.coordinator.config.v2

<<<<<<< HEAD
<<<<<<< HEAD:coordinator/app/src/main/kotlin/lineth/coordinator/config/v2/ForcedTransactionsConfig.kt
=======
import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import linea.coordinator.config.v2.ForcedTransactionsConfig
>>>>>>> abc0edd8e (feat(coordinator): document all TOML config keys with @ConfigDoc/@ConfigSection (#3568)):coordinator/app/src/main/kotlin/linea/coordinator/config/v2/toml/ForcedTransactionsConfigToml.kt
=======
>>>>>>> 5d977f703 (chore(coordinator): package renaming to lineth (#3746))
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

<<<<<<< HEAD
<<<<<<< HEAD:coordinator/app/src/main/kotlin/lineth/coordinator/config/v2/ForcedTransactionsConfig.kt
=======
>>>>>>> 5d977f703 (chore(coordinator): package renaming to lineth (#3746))
data class ForcedTransactionsConfig(
  override val disabled: Boolean = false,
  val l1Endpoint: URL,
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l1RequestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val sequencerEndpoint: URL,
  val sequencerRequestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
<<<<<<< HEAD
=======
data class ForcedTransactionsConfigToml(
  @param:ConfigDoc(description = "Whether forced transactions handling is disabled.", default = "false")
  var disabled: Boolean = false,
  @param:ConfigDoc(
    description = "L1 endpoint used to read forced transactions. Falls back to defaults.l1-endpoint.",
    example = "http://l1-el-node:8545",
  )
  val l1Endpoint: URL? = null, // shall default to L1 endpoint
  @param:ConfigSection("Retry policy for L1 requests; falls back to defaults.l1-request-retries.")
  val l1RequestRetries: RequestRetriesToml? = null,
  @param:ConfigDoc(
    description = "L1 block tag up to which forced-transaction events are read.",
    default = "FINALIZED",
  )
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  @param:ConfigDoc(
    description = "Sequencer endpoint to which forced transactions are submitted.",
    example = "http://sequencer:8545",
  )
  val sequencerEndpoint: URL,
  @param:ConfigSection("Retry policy for sequencer requests; falls back to defaults.l2-request-retries.")
  val sequencerRequestRetries: RequestRetriesToml? = null,
  @param:ConfigDoc(description = "Interval between forced-transaction processing ticks.", default = "PT2M")
>>>>>>> abc0edd8e (feat(coordinator): document all TOML config keys with @ConfigDoc/@ConfigSection (#3568)):coordinator/app/src/main/kotlin/linea/coordinator/config/v2/toml/ForcedTransactionsConfigToml.kt
  val processingTickInterval: Duration = 2.minutes,
  @param:ConfigDoc(
    description = "Delay before processing a forced transaction after it is detected.",
    default = "PT0S",
  )
  val processingDelay: Duration = Duration.ZERO,
<<<<<<< HEAD:coordinator/app/src/main/kotlin/lineth/coordinator/config/v2/ForcedTransactionsConfig.kt
  val l1EventScraping: L1EventScraping = L1EventScraping(),
=======
  @param:ConfigDoc(description = "Number of forced transactions processed per batch.", default = "10")
>>>>>>> abc0edd8e (feat(coordinator): document all TOML config keys with @ConfigDoc/@ConfigSection (#3568)):coordinator/app/src/main/kotlin/linea/coordinator/config/v2/toml/ForcedTransactionsConfigToml.kt
  val processingBatchSize: UInt = 10u,
  @param:ConfigDoc(
    description = "Interval between checks for invalidity proofs of forced transactions.",
    default = "PT2M",
  )
  val invalidityProofCheckInterval: Duration = 2.minutes,
<<<<<<< HEAD:coordinator/app/src/main/kotlin/lineth/coordinator/config/v2/ForcedTransactionsConfig.kt
) : FeatureToggle {
=======
  @param:ConfigSection("L1 event scraping (log polling) settings for forced transactions.")
  val l1EventScraping: L1EventScraping = L1EventScraping(),
) {
>>>>>>> abc0edd8e (feat(coordinator): document all TOML config keys with @ConfigDoc/@ConfigSection (#3568)):coordinator/app/src/main/kotlin/linea/coordinator/config/v2/toml/ForcedTransactionsConfigToml.kt
=======
  val processingTickInterval: Duration = 2.minutes,
  val processingDelay: Duration = Duration.ZERO,
  val l1EventScraping: L1EventScraping = L1EventScraping(),
  val processingBatchSize: UInt = 10u,
  val invalidityProofCheckInterval: Duration = 2.minutes,
) : FeatureToggle {
>>>>>>> 5d977f703 (chore(coordinator): package renaming to lineth (#3746))
  init {
    require(processingTickInterval >= 1.milliseconds) {
      "processingSendTickInterval=$processingTickInterval must be equal or greater than 1ms"
    }
    require(processingDelay >= Duration.ZERO) {
      "processingDelay=$processingDelay must be equal or greater than 0ms"
    }
    require(processingBatchSize >= 1u) {
      "processingBatchSize=$processingBatchSize must be equal or greater than 1"
    }
    require(invalidityProofCheckInterval >= 1.milliseconds) {
      "invalidityProofCheckInterval=$invalidityProofCheckInterval must be equal or greater than 1ms"
    }
  }

  data class L1EventScraping(
<<<<<<< HEAD
    @param:ConfigDoc(description = "Interval between L1 log polling attempts.", default = "PT12S")
    val pollingInterval: Duration = 12.seconds,
    @param:ConfigDoc(description = "Timeout for each L1 log polling request.", default = "PT5S")
    val pollingTimeout: Duration = 5.seconds,
    @param:ConfigDoc(
      description = "Backoff delay after a successful eth_getLogs search before the next one.",
      default = "PT0.001S",
    )
    val ethLogsSearchSuccessBackoffDelay: Duration = 1.milliseconds,
    @param:ConfigDoc(description = "Number of blocks scanned per eth_getLogs chunk.", default = "1000")
    val ethLogsSearchBlockChunkSize: UInt = 1000u,
    @param:ConfigDoc(
      description = "Maximum block range covered by a single eth_getLogs search.",
      default = "10000",
    )
=======
    val pollingInterval: Duration = 12.seconds,
    val pollingTimeout: Duration = 5.seconds,
    val ethLogsSearchSuccessBackoffDelay: Duration = 1.milliseconds,
    val ethLogsSearchBlockChunkSize: UInt = 1000u,
>>>>>>> 5d977f703 (chore(coordinator): package renaming to lineth (#3746))
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
}
