package linea.forcedtx

import linea.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class ForcedTransactionInclusionResult {
  Included,
  BadNonce,
  BadBalance,
  BadPrecompile,
  TooManyLogs,
  FilteredAddresses,
  Phylax,
}

data class ForcedTransactionInclusionStatus(
  val blockNumber: ULong,
  val inclusionResult: ForcedTransactionInclusionResult,
  val transactionHash: ByteArray,
  val from: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransactionInclusionStatus

    if (!transactionHash.contentEquals(other.transactionHash)) return false
    if (blockNumber != other.blockNumber) return false
    if (!from.contentEquals(other.from)) return false
    if (inclusionResult != other.inclusionResult) return false

    return true
  }

  override fun hashCode(): Int {
    var result = transactionHash.contentHashCode()
    result = 31 * result + blockNumber.hashCode()
    result = 31 * result + from.contentHashCode()
    result = 31 * result + inclusionResult.hashCode()
    return result
  }

  override fun toString(): String {
    return "ForcedTransactionsReceipt(blockNumber=$blockNumber, inclusionResult=$inclusionResult, " +
      "transactionHash=${transactionHash.encodeHex()}, from=${from.encodeHex()})"
  }
}

interface ForcedTransactionsClient {
  /**
   * Sends a list of RLP encoded signed transactions, just like eth_sendRawTransaction
   * Transactions order must follow L1 submission, forcedTransactionIndex.
   * Sequencer must evaluate them by this order
   *
   * @return list of transaction hashes
   */
  fun lineaSendForcedRawTransaction(transactions: List<ByteArray>): SafeFuture<List<ByteArray>>
  fun lineaFindForcedTransactionStatus(transactionHash: ByteArray): SafeFuture<ForcedTransactionInclusionStatus?>
  fun lineaGetForcedTransactionStatus(transactionHash: ByteArray): SafeFuture<ForcedTransactionInclusionStatus> =
    lineaFindForcedTransactionStatus(transactionHash).thenApply {
      it ?: throw IllegalStateException("Forced transaction not found: txHash=${transactionHash.encodeHex()}")
    }
}
