package linea.web3j

import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.response.TransactionReceipt
import kotlin.jvm.optionals.getOrNull
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

/**
 * Helper to wait for a transaction receipt to be available.
 * This is useful when you need to wait for a transaction to be mined in your tests before proceeding.
 *
 * @param txHash The transaction hash to wait for.
 * @param timeout The maximum time to wait for the transaction receipt.
 */
fun Web3j.waitForTxReceipt(
  txHash: String,
  expectedStatus: String? = null,
  timeout: Duration = 5.seconds,
  pollingInterval: Duration = 500.milliseconds
): TransactionReceipt {
  val waitLimit = System.currentTimeMillis() + timeout.inWholeMilliseconds
  while (System.currentTimeMillis() < waitLimit) {
    val receipt = this.ethGetTransactionReceipt(txHash).send().transactionReceipt.getOrNull()
    if (receipt != null) {
      if (expectedStatus != null && receipt.status != expectedStatus) {
        throw RuntimeException(
          "Transaction status does not match expected status: " +
            "txHash=$txHash, expected=$expectedStatus, actual=${receipt.status}"
        )
      }
      return receipt
    }

    Thread.sleep(pollingInterval.inWholeMilliseconds)
  }

  throw RuntimeException("Timed out waiting $timeout for transaction receipt for tx $txHash")
}
