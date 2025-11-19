package linea.domain

import linea.kotlin.encodeHex

data class TransactionReceipt(
  val transactionHash: ByteArray,
  val transactionIndex: ULong,
  val blockHash: ByteArray,
  val blockNumber: ULong,
  val from: ByteArray,
  val to: ByteArray?, // Nullable for contract creation transactions
  val cumulativeGasUsed: ULong,
  val gasUsed: ULong,
  val contractAddress: ByteArray?, // Nullable, only present for contract creation transactions
  val logs: List<EthLog>,
  val logsBloom: ByteArray,
  val status: ULong?, // 1 for success, 0 for failure, null for pre-Byzantium
  val root: ByteArray?, // State root for pre-Byzantium transactions, null for post-Byzantium
  val effectiveGasPrice: ULong, // Actual gas price used for the transaction
  val type: TransactionType,
  val blobGasUsed: ULong?, // EIP-4844 blob gas used, null for non-blob transactions
  val blobGasPrice: ULong?, // EIP-4844 blob gas price, null for non-blob transactions
) {

  /**
   * Returns true if the transaction was successful.
   * For pre-Byzantium transactions (where status is null), assumes success if no revert root is present.
   */
  fun isSuccessful(): Boolean {
    return status?.let { it == 1UL } ?: (root != null)
  }

  /**
   * Returns true if the transaction failed.
   */
  fun isFailed(): Boolean = !isSuccessful()

  /**
   * Returns true if this is a contract creation transaction.
   */
  fun isContractCreation(): Boolean = contractAddress != null

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as TransactionReceipt

    if (!transactionHash.contentEquals(other.transactionHash)) return false
    if (transactionIndex != other.transactionIndex) return false
    if (!blockHash.contentEquals(other.blockHash)) return false
    if (blockNumber != other.blockNumber) return false
    if (!from.contentEquals(other.from)) return false
    if (to != null) {
      if (other.to == null) return false
      if (!to.contentEquals(other.to)) return false
    } else if (other.to != null) return false
    if (cumulativeGasUsed != other.cumulativeGasUsed) return false
    if (gasUsed != other.gasUsed) return false
    if (contractAddress != null) {
      if (other.contractAddress == null) return false
      if (!contractAddress.contentEquals(other.contractAddress)) return false
    } else if (other.contractAddress != null) return false
    if (logs != other.logs) return false
    if (!logsBloom.contentEquals(other.logsBloom)) return false
    if (status != other.status) return false
    if (root != null) {
      if (other.root == null) return false
      if (!root.contentEquals(other.root)) return false
    } else if (other.root != null) return false
    if (effectiveGasPrice != other.effectiveGasPrice) return false
    if (type != other.type) return false
    if (blobGasUsed != other.blobGasUsed) return false
    if (blobGasPrice != other.blobGasPrice) return false

    return true
  }

  override fun hashCode(): Int {
    var result = transactionHash.contentHashCode()
    result = 31 * result + transactionIndex.hashCode()
    result = 31 * result + blockHash.contentHashCode()
    result = 31 * result + blockNumber.hashCode()
    result = 31 * result + from.contentHashCode()
    result = 31 * result + (to?.contentHashCode() ?: 0)
    result = 31 * result + cumulativeGasUsed.hashCode()
    result = 31 * result + gasUsed.hashCode()
    result = 31 * result + (contractAddress?.contentHashCode() ?: 0)
    result = 31 * result + logs.hashCode()
    result = 31 * result + logsBloom.contentHashCode()
    result = 31 * result + (status?.hashCode() ?: 0)
    result = 31 * result + (root?.contentHashCode() ?: 0)
    result = 31 * result + effectiveGasPrice.hashCode()
    result = 31 * result + type.hashCode()
    result = 31 * result + (blobGasUsed?.hashCode() ?: 0)
    result = 31 * result + (blobGasPrice?.hashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "TransactionReceipt(" +
      "transactionHash=${transactionHash.encodeHex()}, " +
      "transactionIndex=$transactionIndex, " +
      "blockHash=${blockHash.encodeHex()}, " +
      "blockNumber=$blockNumber, " +
      "from=${from.encodeHex()}, " +
      "to=${to?.encodeHex()}, " +
      "cumulativeGasUsed=$cumulativeGasUsed, " +
      "gasUsed=$gasUsed, " +
      "contractAddress=${contractAddress?.encodeHex()}, " +
      "logs=${logs.size} logs, " +
      "logsBloom=${logsBloom.encodeHex()}, " +
      "status=$status, " +
      "root=${root?.encodeHex()}, " +
      "effectiveGasPrice=$effectiveGasPrice, " +
      "type=$type, " +
      "blobGasUsed=$blobGasUsed, " +
      "blobGasPrice=$blobGasPrice" +
      ")"
  }
}
