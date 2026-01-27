package linea.coordinator.config.v2.toml

import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class RequestRetriesToml(
  val maxRetries: UInt? = null,
  val timeout: Duration? = null,
  val backoffDelay: Duration = 1.seconds,
  val failuresWarningThreshold: UInt? = null,
) {
  init {
    maxRetries?.also {
      require(maxRetries >= 1u) { "maxRetries must be >=1. value=$maxRetries" }
    }
    timeout?.also {
      require(timeout >= 1.milliseconds) { "timeout must be >= 1ms. value=$timeout" }
    }
    require(backoffDelay >= 1.milliseconds) {
      "backoffDelay must be >= 1ms. value=$backoffDelay"
    }
    require(failuresWarningThreshold == null || failuresWarningThreshold > 0u) {
      "failuresWarningThreshold must be greater than or equal to 0. value=$failuresWarningThreshold"
    }
  }

  internal val asJsonRpcRetryConfig =
    RequestRetryConfig(
      maxRetries = maxRetries,
      timeout = timeout,
      backoffDelay = backoffDelay,
      failuresWarningThreshold = failuresWarningThreshold ?: 0u,
    )

  internal val asDomain: linea.domain.RetryConfig =
    linea.domain.RetryConfig(
      maxRetries = maxRetries,
      timeout = timeout,
      backoffDelay = backoffDelay,
      failuresWarningThreshold = failuresWarningThreshold ?: 0u,
    )

  companion object {
    fun endlessRetry(backoffDelay: Duration, failuresWarningThreshold: UInt) = RequestRetriesToml(
      maxRetries = null,
      timeout = null,
      backoffDelay = backoffDelay,
      failuresWarningThreshold = failuresWarningThreshold,
    )
  }
}
