package net.consensys.linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger

interface JsonRpcRequestResponseLogger {
  fun logRequest(endpoint: String, jsonBody: String, throwable: Throwable? = null)
  fun logResponse(
    endpoint: String,
    responseStatusCode: Int?,
    requestBody: String,
    responseBody: String,
    failureCause: Throwable? = null
  )

  fun isJsonRpcError(responseStatusCode: Int?, responseBody: String?): Boolean {
    return responseStatusCode != 200 || (responseBody?.contains("\"error\":", ignoreCase = true) ?: false)
  }
}

class MinimalInLineJsonRpcLogger(
  val logger: Logger,
  val requestResponseLogLevel: Level = Level.DEBUG,
  val failuresLogLevel: Level = Level.WARN,
  val maskEndpoint: LogFieldMask = ::noopMask
) : JsonRpcRequestResponseLogger {

  private fun logRequestOnLevel(level: Level, endpoint: String, jsonBody: String, throwable: Throwable?) {
    if (throwable == null) {
      logger.log(level, "--> {} {}", endpoint, jsonBody)
    } else {
      logger.log(level, "--> {} {} failed with error={}", endpoint, jsonBody, throwable.message, throwable)
    }
  }

  override fun logRequest(endpoint: String, jsonBody: String, throwable: Throwable?) {
    logRequestOnLevel(requestResponseLogLevel, maskEndpoint(endpoint), jsonBody, throwable)
  }

  override fun logResponse(
    endpoint: String,
    responseStatusCode: Int?,
    requestBody: String,
    responseBody: String,
    failureCause: Throwable?
  ) {
    val isError = failureCause != null || isJsonRpcError(responseStatusCode, responseBody)
    val logLevel = if (isError) failuresLogLevel else requestResponseLogLevel
    val maskedEndpoint = maskEndpoint(endpoint)
    if (isError && logger.level != requestResponseLogLevel) {
      // in case of error, log the request that originated the error
      // to help replicate and debug later
      logRequestOnLevel(logLevel, maskedEndpoint, requestBody, null)
    }

    if (failureCause == null) {
      logger.log(
        logLevel,
        "<-- {} {} {}",
        maskedEndpoint,
        responseStatusCode,
        responseBody
      )
    } else {
      logger.log(
        logLevel,
        "<-- {} {} {} failed with error={}",
        maskedEndpoint,
        responseStatusCode,
        responseBody,
        failureCause.message
      )
    }
  }
}
