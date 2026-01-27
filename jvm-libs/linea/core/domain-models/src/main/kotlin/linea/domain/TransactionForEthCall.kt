package linea.domain

import linea.kotlin.encodeHex
import java.math.BigInteger

/**
 * Represents transaction parameters for eth_call method.
 * This is used to simulate transaction execution without actually executing it on-chain.
 */
data class TransactionForEthCall(
  val from: ByteArray,
  val to: ByteArray? = null,
  // transaction execution gas limit (optional, uses block gas limit if not provided)
  val gas: ULong? = null,
  val gasPrice: ULong? = null, // Gas price per unit (nullable for EIP-1559 transactions)
  // Maximum total fee per gas the sender is willing to pay (EIP-1559)
  val maxFeePerGas: ULong? = null,
  // Maximum priority fee per gas the sender is willing to pay (EIP-1559)
  val maxPriorityFeePerGas: ULong? = null,
  val maxFeePerBlobGas: ULong? = null,
  val nonce: ULong? = null,
  val value: BigInteger = BigInteger.ZERO,
  val data: ByteArray? = null, // The compiled code of a contract OR
  // the hash of the invoked method signature and encoded parameters
  val accessList: List<AccessListEntry>? = null, // Array of access list entries (EIP-2930)
  // Determines if the sender account balance is considered during gas estimation.
  // If true, the sender's balance is checked against the transaction's gas parameters.
  // This ensures the estimated gas reflects what the sender can actually afford.
  // If false, the balance checks are skipped. The default is true.
  val strict: Boolean = true,
  val blobVersionedHashes: List<ByteArray>? = null, // List of references to blobs introduced in EIP-4844.
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as TransactionForEthCall

    if (strict != other.strict) return false
    if (!from.contentEquals(other.from)) return false
    if (!to.contentEquals(other.to)) return false
    if (gas != other.gas) return false
    if (gasPrice != other.gasPrice) return false
    if (maxFeePerGas != other.maxFeePerGas) return false
    if (maxPriorityFeePerGas != other.maxPriorityFeePerGas) return false
    if (value != other.value) return false
    if (!data.contentEquals(other.data)) return false
    if (accessList != other.accessList) return false
    if (blobVersionedHashes != other.blobVersionedHashes) return false

    return true
  }

  override fun hashCode(): Int {
    var result = strict.hashCode()
    result = 31 * result + from.contentHashCode()
    result = 31 * result + (to?.contentHashCode() ?: 0)
    result = 31 * result + (gas?.hashCode() ?: 0)
    result = 31 * result + (gasPrice?.hashCode() ?: 0)
    result = 31 * result + (maxFeePerGas?.hashCode() ?: 0)
    result = 31 * result + (maxPriorityFeePerGas?.hashCode() ?: 0)
    result = 31 * result + (value.hashCode())
    result = 31 * result + (data?.contentHashCode() ?: 0)
    result = 31 * result + (accessList?.hashCode() ?: 0)
    result = 31 * result + (blobVersionedHashes?.hashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "TransactionForEthCall(from=${from.encodeHex()}, " +
      "to=${to?.encodeHex()}, " +
      "gas=$gas, " +
      "gasPrice=$gasPrice, " +
      "maxFeePerGas=$maxFeePerGas, " +
      "maxPriorityFeePerGas=$maxPriorityFeePerGas, " +
      "value=$value, " +
      "data=${data?.encodeHex()}, " +
      "accessList=$accessList, " +
      "strict=$strict, " +
      "blobVersionedHashes=$blobVersionedHashes)"
  }
}
