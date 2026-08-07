package lineth.coordinator.config.v2.toml

import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import lineth.coordinator.config.v2.Type2StateProofManagerConfig
import lineth.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class Type2StateProofManagerToml(
  @param:ConfigDoc(description = "Whether the type-2 state proof provider is disabled.", default = "false")
  val disabled: Boolean = false,
  @param:ConfigDoc(
    description = "Type-2 state proof provider JSON-RPC endpoints.",
    example = "[\"http://shomei-frontend:8888\"]",
  )
  val endpoints: List<URL>,
  @param:ConfigSection("Retry policy for type-2 state proof requests.")
  val requestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  @param:ConfigDoc(
    description = "L1 block tag queried for state proofs (e.g. FINALIZED, LATEST).",
    default = "FINALIZED",
  )
  val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
  @param:ConfigDoc(description = "Interval between L1 polls for state proof updates.", default = "PT6S")
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
