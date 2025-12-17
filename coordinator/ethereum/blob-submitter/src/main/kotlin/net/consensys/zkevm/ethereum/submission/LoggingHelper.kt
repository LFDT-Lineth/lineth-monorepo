package net.consensys.zkevm.ethereum.submission

import org.apache.logging.log4j.Logger

private const val insufficientGasFeeRegexStr =
  "(max fee per (blob )?gas less than block (blob gas|base) fee:" +
    " address 0x[a-fA-F0-9]{40},? (blobGasFeeCap|maxFeePerGas): [0-9]+," +
    " (blobBaseFee|baseFee): [0-9]+ \\(supplied gas [0-9]+\\))"

private val insufficientGasFeeRegex = Regex(insufficientGasFeeRegexStr)
private val maxFeePerBlobGasRegex = Regex("max fee per blob gas|blobGasFeeCap")

private fun rewriteInsufficientGasFeeErrorMessage(errorMessage: String): String? {
  return insufficientGasFeeRegex.find(errorMessage)?.groupValues?.first()
    ?.replace("max fee per gas", "maxFeePerGas")
    ?.replace(maxFeePerBlobGasRegex, "maxFeePerBlobGas")
}

fun logUnhandledError(log: Logger, errorOrigin: String, error: Throwable) {
  if (error.message != null) {
    log.error(
      "Error from {}: errorMessage={}",
      errorOrigin,
      error.message,
    )
  } else {
    log.error("Error from {}: ", errorOrigin, error)
  }
}

/**
 * Logs submission errors
 * 3 parameters to the message:
 * "{ETH_CALL_TYPE} for blob submission failed: blob={INTERVAL_STING} errorMessage={ERROR_MESSAGE}"
 */
fun logSubmissionError(
  log: Logger,
  logMessage: String,
  intervalString: String,
  error: Throwable,
  isEthCall: Boolean = false,
) {
  var matchedInsufficientGasFeeRegex = false
  val ethMethod = if (isEthCall) "eth_call" else "eth_sendRawTransaction"
  val errorMessage = if (isEthCall) {
    error.message?.let {
      rewriteInsufficientGasFeeErrorMessage(it)?.also {
        matchedInsufficientGasFeeRegex = true
      }
    } ?: error.message
  } else {
    error.message
  }

  if (matchedInsufficientGasFeeRegex) {
    log.info(
      logMessage,
      ethMethod,
      intervalString,
      errorMessage,
    )
  } else {
    log.error(
      logMessage,
      ethMethod,
      intervalString,
      errorMessage,
      error,
    )
  }
}
