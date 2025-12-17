package linea.coordinator.config.v2

import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class StateManagerConfig(
  val version: String,
  val endpoints: List<URL>,
  val requestLimitPerEndpoint: UInt = UInt.MAX_VALUE,
  val requestTimeout: Duration? = null,
  val requestRetries: RetryConfig =
    RetryConfig.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
)
