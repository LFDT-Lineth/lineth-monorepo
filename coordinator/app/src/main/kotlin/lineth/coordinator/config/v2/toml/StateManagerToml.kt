package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.config.v2.StateManagerConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class StateManagerToml(
  @param:ConfigDoc(
    description = "Shomei state manager JSON-RPC endpoints.",
    example = "[\"http://shomei:8888\"]",
  )
  val endpoints: List<URL>,
  @param:ConfigDoc(
    description = "Maximum number of concurrent in-flight requests per state manager endpoint.",
    default = "4294967295",
  )
  val requestLimitPerEndpoint: UInt = UInt.MAX_VALUE,
  @param:ConfigDoc(
    description = "Timeout for each state manager request. Omit to disable the timeout.",
    example = "PT30S",
  )
  val requestTimeout: Duration? = null,
  @param:ConfigSection("Retry policy for state manager requests.")
  val requestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
) {
  fun reified(): StateManagerConfig {
    return StateManagerConfig(
      endpoints = this.endpoints,
      requestLimitPerEndpoint = this.requestLimitPerEndpoint,
      requestTimeout = this.requestTimeout,
      requestRetries = this.requestRetries.asDomain,
    )
  }
}
