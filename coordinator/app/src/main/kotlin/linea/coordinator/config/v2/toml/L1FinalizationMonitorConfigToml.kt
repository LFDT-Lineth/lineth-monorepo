package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.L1FinalizationMonitorConfig
import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class L1FinalizationMonitorConfigToml(
  val l1Endpoint: URL?,
  val l2Endpoint: URL?,
  val l1PollingInterval: Duration = 6.seconds,
  val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
  val l1RequestRetries: RequestRetriesToml? = null,
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
