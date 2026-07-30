package linea.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import linea.coordinator.config.v2.L1FinalizationMonitorConfig
import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class L1FinalizationMonitorConfigToml(
  @param:ConfigDoc(
    description = "L1 endpoint for finalization monitoring. Falls back to defaults.l1-endpoint.",
    example = "http://l1-el-node:8545",
  )
  val l1Endpoint: URL?,
  @param:ConfigDoc(
    description = "L2 endpoint for finalization monitoring. Falls back to defaults.l2-endpoint.",
    example = "http://sequencer:8545",
  )
  val l2Endpoint: URL?,
  @param:ConfigDoc(description = "Interval between L1 finalization polls.", default = "PT6S")
  val l1PollingInterval: Duration = 6.seconds,
  @param:ConfigDoc(
    description = "L1 block tag treated as finalized (e.g. FINALIZED, SAFE, LATEST).",
    default = "FINALIZED",
  )
  val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
  @param:ConfigSection("Retry policy for L1 requests; falls back to defaults.l1-request-retries.")
  val l1RequestRetries: RequestRetriesToml? = null,
  @param:ConfigSection("Retry policy for L2 requests; falls back to defaults.l2-request-retries.")
  val l2RequestRetries: RequestRetriesToml? = null,
) {
  fun reified(defaults: DefaultsToml): L1FinalizationMonitorConfig {
    return L1FinalizationMonitorConfig(
      l1Endpoint = this.l1Endpoint ?: defaults.l1Endpoint ?: throw AssertionError("l1Endpoint missing"),
      l2Endpoint = this.l2Endpoint ?: defaults.l2Endpoint ?: throw AssertionError("l2Endpoint missing"),
      l1PollingInterval = this.l1PollingInterval,
      l1QueryBlockTag = this.l1QueryBlockTag,
      l1RequestRetries = this.l1RequestRetries?.asDomain ?: defaults.l1RequestRetries.asDomain,
      l2RequestRetries = this.l2RequestRetries?.asDomain ?: defaults.l2RequestRetries.asDomain,
    )
  }
}
