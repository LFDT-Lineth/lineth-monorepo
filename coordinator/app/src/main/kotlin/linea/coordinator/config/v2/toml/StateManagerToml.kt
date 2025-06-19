package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.StateManagerConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class StateManagerToml(
  val version: String,
  val endpoints: List<URL>,
  val requestLimitPerEndpoint: UInt = UInt.MAX_VALUE,
  val requestTimeout: Duration? = null,
  val requestRetries: RequestRetriesToml = RequestRetriesToml.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
) {
  fun reified(): StateManagerConfig {
    return StateManagerConfig(
      version = this.version,
      endpoints = this.endpoints,
      requestLimitPerEndpoint = this.requestLimitPerEndpoint,
      requestTimeout = this.requestTimeout,
      requestRetries = this.requestRetries.asDomain,
    )
  }
}
