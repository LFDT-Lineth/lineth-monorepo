package linea.coordinator.config

import lineth.domain.RetryConfig
import lineth.jsonrpc.client.RequestRetryConfig

fun RetryConfig.toJsonRpcRetry(): RequestRetryConfig {
  return RequestRetryConfig(
    maxRetries = maxRetries,
    timeout = timeout,
    backoffDelay = backoffDelay,
    failuresWarningThreshold = failuresWarningThreshold,
  )
}
