package net.consensys.zkevm.ethereum.error.handling

import org.web3j.protocol.core.methods.response.TransactionReceipt
import tech.pegasys.teku.infrastructure.async.SafeFuture
class SubmissionException(message: String, cause: Throwable) : RuntimeException(message, cause)
object ErrorHandling {
  fun handleError(
    messagePrefix: String,
    error: Throwable,
    ethCall: () -> SafeFuture<*>,
  ): SafeFuture<TransactionReceipt> {
    return ethCall().handleException { errorWithRevertReason ->
      val txHash = extractTransactionHashFromErrorMessage(error.message!!)
      val message =
        "$messagePrefix tx hash: $txHash error: ${error.message}"
      throw SubmissionException(message, errorWithRevertReason)
    }.thenApply {
      throw error
    }
  }

  private val transactionHashRegex = Regex("Transaction (.+?) ")

  private fun extractTransactionHashFromErrorMessage(message: String): String? {
    val txHashMatches = transactionHashRegex.findAll(message).toList()
    return if (txHashMatches.isEmpty()) {
      null
    } else {
      txHashMatches.first().groupValues[1]
    }
  }
}
