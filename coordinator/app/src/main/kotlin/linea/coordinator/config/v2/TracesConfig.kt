package linea.coordinator.config.v2

import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class TracesConfig(
  val expectedTracesApiVersion: String,
  val common: ClientApiConfig? = null,
  val counters: ClientApiConfig? = null,
  val conflation: ClientApiConfig? = null,
  val ignoreTracesGeneratorErrors: Boolean = false,
  // val switchBlockNumberInclusive: UInt? = null,
  // val new: TracesConfig? = null
) {
  init {
    require(
      (common !== null && counters == null && conflation == null) ||
        (common == null && counters !== null && conflation !== null),
    ) { "traces endpoints are invalid. either common, counters or conflation must be set" }
  }

  data class ClientApiConfig(
    val endpoints: List<URL>,
    val requestLimitPerEndpoint: UInt = 100u,
    val requestTimeout: Duration? = null,
    val requestRetries: RetryConfig =
      RetryConfig.endlessRetry(
        backoffDelay = 1.seconds,
        failuresWarningThreshold = 3u,
      ),
  )
}
