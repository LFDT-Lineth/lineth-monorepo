package linea.coordinator.config

import linea.domain.RetryConfig
import net.consensys.linea.jsonrpc.client.RequestRetryConfig

fun RetryConfig.toJsonRpcRetry(): RequestRetryConfig {
  return RequestRetryConfig(
    maxRetries = maxRetries,
    timeout = timeout,
    backoffDelay = backoffDelay,
    failuresWarningThreshold = failuresWarningThreshold,
  )
}
