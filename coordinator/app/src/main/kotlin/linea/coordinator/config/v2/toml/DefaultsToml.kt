package linea.coordinator.config.v2.toml

import java.net.URL
import kotlin.time.Duration.Companion.seconds

data class DefaultsToml(
  val l1Endpoint: URL? = null,
  val l2Endpoint: URL? = null,
  val l1RequestRetries: RequestRetriesToml = RequestRetriesToml.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val l2RequestRetries: RequestRetriesToml = RequestRetriesToml.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
)
