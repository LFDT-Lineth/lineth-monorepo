package linea.coordinator.config.v2

import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class TracesConfig(
  val expectedTracesApiVersion: String,
  val counters: ClientApiConfig,
  val conflation: ClientApiConfig,
  // val switchBlockNumberInclusive: UInt? = null,
  // val new: TracesConfig? = null
) {
  data class ClientApiConfig(
    val endpoints: List<URL>,
    val requestLimitPerEndpoint: UInt = 100u,
    val requestTimeout: Duration? = null,
    val requestRetries: RetryConfig = RetryConfig.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  )
}
