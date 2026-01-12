package linea.web3j

import linea.domain.TransactionReceipt
import linea.ethapi.EthApiClient
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
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
fun EthApiClient.waitForTxReceipt(
  txHash: ByteArray,
  expectedStatus: ULong? = null,
  timeout: Duration = 5.seconds,
  pollingInterval: Duration = 500.milliseconds,
  log: Logger = LogManager.getLogger("linea.web3j.waitForTxReceipt"),
): TransactionReceipt {
  val waitLimit = System.currentTimeMillis() + timeout.inWholeMilliseconds
  log.debug("polling tx receipt txHash=${txHash.encodeHex()}")
  while (System.currentTimeMillis() < waitLimit) {
    log.trace("polling tx receipt txHash=${txHash.encodeHex()}")
    val receipt = runCatching<TransactionReceipt?> {
      this.ethGetTransactionReceipt(txHash).get()
    }.onFailure {
      log.error("polling tx receipt txHash={}", txHash, it)
    }.getOrNull()

    if (receipt != null) {
      log.debug("tx receipt found: txHash={} receiptStatus={}", txHash, receipt.status)
      if (expectedStatus != null && receipt.status != expectedStatus) {
        throw RuntimeException(
          "Transaction status does not match expected status: " +
            "txHash=${txHash.encodeHex()}, expected=$expectedStatus, actual=${receipt.status}",
        )
      }
      return receipt
    }

    Thread.sleep(pollingInterval.inWholeMilliseconds)
  }

  throw RuntimeException("Timed out waiting $timeout for transaction receipt for tx ${txHash.encodeHex()}")
}
