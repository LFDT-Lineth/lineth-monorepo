package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.Type2StateProofManagerConfig
import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class Type2StateProofManagerToml(
  val disabled: Boolean = false,
  val endpoints: List<URL>,
  val requestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
  val l1PollingInterval: Duration = 6.seconds,
) {
  fun reified(): Type2StateProofManagerConfig {
    return Type2StateProofManagerConfig(
      disabled = this.disabled,
      endpoints = this.endpoints,
      requestRetries = this.requestRetries.asDomain,
      l1QueryBlockTag = this.l1QueryBlockTag,
      l1PollingInterval = this.l1PollingInterval,
    )
  }
}
