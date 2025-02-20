package linea.web3j

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
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
  pollingInterval: Duration = 500.milliseconds,
  log: Logger = LogManager.getLogger("linea.web3j.waitForTxReceipt")
): TransactionReceipt {
  val waitLimit = System.currentTimeMillis() + timeout.inWholeMilliseconds
  log.debug("polling tx receipt txHash=$txHash")
  while (System.currentTimeMillis() < waitLimit) {
    log.trace("polling tx receipt txHash=$txHash")
    val receipt = runCatching<TransactionReceipt?> {
      this.ethGetTransactionReceipt(txHash).send().transactionReceipt.getOrNull()
    }.onFailure {
      log.error("polling tx receipt txHash={}", txHash, it)
    }.getOrNull()

    if (receipt != null) {
      log.debug("tx receipt found: txHash={} receiptStatus={}", txHash, receipt.status)
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
