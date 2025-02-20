package linea.domain

import linea.kotlin.encodeHex
import java.math.BigInteger
import java.util.EnumSet

enum class TransactionType(private val typeValue: Int) {
  FRONTIER(248),
  ACCESS_LIST(1),
  EIP1559(2),
  BLOB(3), // Not supported by Linea atm, but here for completeness
  DELEGATE_CODE(4); // Not supported by Linea atm, but here for completeness

  val serializedType: Byte
    get() = typeValue.toByte()

  val ethSerializedType: Byte
    get() = if (this == FRONTIER) 0 else serializedType

  fun compareTo(b: Byte?): Int {
    return serializedType.compareTo(b!!)
  }

  fun supports1559FeeMarket(): Boolean {
    return !TransactionType.LEGACY_FEE_MARKET_TRANSACTION_TYPES.contains(this)
  }

  companion object {
    private val ACCESS_LIST_SUPPORTED_TRANSACTION_TYPES: Set<TransactionType> =
      EnumSet.of(ACCESS_LIST, EIP1559, BLOB, DELEGATE_CODE)
    private val LEGACY_FEE_MARKET_TRANSACTION_TYPES: Set<TransactionType> = EnumSet.of(FRONTIER, ACCESS_LIST)

    fun fromSerializedValue(serializedTypeValue: Int): TransactionType {
      return entries
        .firstOrNull { type: TransactionType -> type.typeValue == serializedTypeValue }
        ?: throw IllegalArgumentException(
          String.format(
            "Unsupported transaction type %x",
            serializedTypeValue
          )
        )
    }

    fun fromEthApiSerializedValue(serializedTypeValue: Int): TransactionType {
      if (serializedTypeValue == 0) {
        return FRONTIER
      }
      return fromSerializedValue(serializedTypeValue)
    }
  }
}

data class Transaction(
  val type: TransactionType,
  val nonce: ULong,
  val gasLimit: ULong,
  val to: ByteArray?, // Nullable for contract creation transactions
  val value: BigInteger,
  val input: ByteArray,
  val r: BigInteger,
  val s: BigInteger,
  val v: ULong?, // is defined if type is FRONTIER
  val yParity: ULong?, // EIP-2718 yParity is defined for all transactions types after FRONTIER
  val chainId: ULong? = null, // Optional field for EIP-155 transactions
  val gasPrice: ULong?, // null for EIP-1559 transactions
  val maxFeePerGas: ULong? = null, // null for EIP-1559 transactions
  val maxPriorityFeePerGas: ULong? = null, // null for non EIP-1559 transactions
  val accessList: List<AccessListEntry>? // null non for EIP-2930 transactions
) {
  companion object {
    // companion object to allow static extension functions
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as Transaction

    if (nonce != other.nonce) return false
    if (gasPrice != other.gasPrice) return false
    if (gasLimit != other.gasLimit) return false
    if (to != null) {
      if (other.to == null) return false
      if (!to.contentEquals(other.to)) return false
    } else if (other.to != null) return false
    if (value != other.value) return false
    if (!input.contentEquals(other.input)) return false
    if (r != other.r) return false
    if (s != other.s) return false
    if (v != other.v) return false
    if (yParity != other.yParity) return false
    if (type != other.type) return false
    if (chainId != other.chainId) return false
    if (maxPriorityFeePerGas != other.maxPriorityFeePerGas) return false
    if (maxFeePerGas != other.maxFeePerGas) return false
    if (accessList != other.accessList) return false

    return true
  }

  override fun hashCode(): Int {
    var result = nonce.hashCode()
    result = 31 * result + gasPrice.hashCode()
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + (to?.contentHashCode() ?: 0)
    result = 31 * result + value.hashCode()
    result = 31 * result + input.contentHashCode()
    result = 31 * result + r.hashCode()
    result = 31 * result + s.hashCode()
    result = 31 * result + v.hashCode()
    result = 31 * result + yParity.hashCode()
    result = 31 * result + type.hashCode()
    result = 31 * result + (chainId?.hashCode() ?: 0)
    result = 31 * result + (maxPriorityFeePerGas?.hashCode() ?: 0)
    result = 31 * result + (maxFeePerGas?.hashCode() ?: 0)
    result = 31 * result + accessList.hashCode()
    return result
  }

  override fun toString(): String {
    return "Transaction(" +
      "type=$type, " +
      "nonce=$nonce, " +
      "gasLimit=$gasLimit, " +
      "to=${to?.encodeHex()}, " +
      "value=$value, " +
      "input=${input.encodeHex()}, " +
      "r=$r, " +
      "s=$s, " +
      "v=$v, " +
      "yParity=$yParity, " +
      "chainId=$chainId, " +
      "gasPrice=$gasPrice, " +
      "maxFeePerGas=$maxFeePerGas, " +
      "maxPriorityFeePerGas=$maxPriorityFeePerGas, " +
      "accessList=$accessList)"
  }
}

data class AccessListEntry(
  val address: ByteArray,
  val storageKeys: List<ByteArray>
) {

  override fun toString(): String {
    return "AccessListEntry(" +
      "address=${address.encodeHex()}, " +
      "storageKeys=[${storageKeys.joinToString(",") { it.encodeHex() }}]" +
      ")"
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as AccessListEntry

    if (!address.contentEquals(other.address)) return false
    if (storageKeys.size != other.storageKeys.size) return false
    storageKeys.zip(other.storageKeys).forEach { (a, b) ->
      if (!a.contentEquals(b)) return false
    }

    return true
  }

  override fun hashCode(): Int {
    var result = address.contentHashCode()
    result = 31 * result + storageKeys.hashCode()
    return result
  }
}
