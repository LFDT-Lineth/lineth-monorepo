package lineth.coordinator.config.v2.toml

import lineth.config.docs.ConfigDoc
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class RequestRetriesToml(
  @param:ConfigDoc(
    description = "Maximum number of retry attempts. Omit for endless retries.",
    example = "3",
  )
  val maxRetries: UInt? = null,
  @param:ConfigDoc(
    description = "Overall timeout across all retry attempts. Omit to disable the timeout.",
    example = "PT10S",
  )
  val timeout: Duration? = null,
  @param:ConfigDoc(
    description = "Grace period during which initial failures are ignored (not logged/counted) " +
      "before retries are treated as errors. Omit to disable.",
    example = "PT5S",
  )
  val ignoreFirstExceptionsUntilTimeElapsed: Duration? = null,
  @param:ConfigDoc(
    description = "Delay between retry attempts.",
    default = "PT1S",
    example = "PT1S",
  )
  val backoffDelay: Duration = 1.seconds,
  @param:ConfigDoc(
    description = "Number of consecutive failures after which warning logs start being emitted. " +
      "Omit to use the default threshold.",
    example = "3",
  )
  val failuresWarningThreshold: UInt? = null,
) {
  init {
    maxRetries?.also {
      require(maxRetries >= 1u) { "maxRetries must be >=1. value=$maxRetries" }
    }
    timeout?.also {
      require(timeout >= 1.milliseconds) { "timeout must be >= 1ms. value=$timeout" }
    }
    ignoreFirstExceptionsUntilTimeElapsed?.also {
      require(ignoreFirstExceptionsUntilTimeElapsed >= 1.milliseconds) {
        "ignoreFirstExceptionsUntilTimeElapsed must be >= 1ms. value=$ignoreFirstExceptionsUntilTimeElapsed"
      }
    }
    require(backoffDelay >= 1.milliseconds) {
      "backoffDelay must be >= 1ms. value=$backoffDelay"
    }
    require(failuresWarningThreshold == null || failuresWarningThreshold > 0u) {
      "failuresWarningThreshold must be greater than or equal to 0. value=$failuresWarningThreshold"
    }
  }

  internal val asDomain: lineth.domain.RetryConfig =
    lineth.domain.RetryConfig(
      maxRetries = maxRetries,
      timeout = timeout,
      ignoreFirstExceptionsUntilTimeElapsed = ignoreFirstExceptionsUntilTimeElapsed,
      backoffDelay = backoffDelay,
      failuresWarningThreshold = failuresWarningThreshold ?: 0u,
    )

  companion object {
    fun endlessRetry(
      backoffDelay: Duration = 1.seconds,
      failuresWarningThreshold: UInt = 3u,
    ) = RequestRetriesToml(
      maxRetries = null,
      timeout = null,
      backoffDelay = backoffDelay,
      failuresWarningThreshold = failuresWarningThreshold,
    )
  }
}
