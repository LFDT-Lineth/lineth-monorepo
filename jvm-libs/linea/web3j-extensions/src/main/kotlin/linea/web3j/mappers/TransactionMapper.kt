package linea.web3j.mappers

import linea.domain.AccessListEntry
import linea.domain.CodeDelegation
import linea.domain.Transaction
import linea.domain.TransactionType
import linea.kotlin.decodeHex
import linea.kotlin.toBigIntegerFromHex
import linea.kotlin.toIntFromHex
import linea.kotlin.toULong
import linea.kotlin.toULongFromHex

fun mapType(type: String?): TransactionType {
  return type
    ?.let { TransactionType.fromEthApiSerializedValue(it.toIntFromHex()) }
    ?: TransactionType.FRONTIER
}

fun org.web3j.protocol.core.methods.response.Transaction.toDomain(): Transaction {
  val txType = mapType(this.type)
  var gasPrice: ULong? = null
  var maxFeePerGas: ULong? = null
  var maxPriorityFeePerGas: ULong? = null

  if (txType.supports1559FeeMarket()) {
    maxFeePerGas = this.maxFeePerGas?.toULong()
    maxPriorityFeePerGas = this.maxPriorityFeePerGas?.toULong()
  } else {
    gasPrice = this.gasPrice.toULong()
  }
  val accessList = this.accessList?.map { accessListEntry ->
    AccessListEntry(
      accessListEntry.address.decodeHex(),
      accessListEntry.storageKeys.map { it.decodeHex() },
    )
  }
  // TODO: Web3j doesn't support Type 4 / 7702 parsing transactions from the block
  val codeDelegations = emptyList<CodeDelegation>()

  val chainId = run {
    this.chainId?.toULong()?.let {
      when {
        it == 0UL -> null // Web3j uses 0 for chainId when it's not present
        else -> it
      }
    }
  }

  val domainTx = Transaction(
    nonce = this.nonce.toULong(),
    gasLimit = this.gas.toULong(),
    to = this.to?.decodeHex(),
    value = this.value,
    input = this.input.decodeHex(),
    r = this.r.toBigIntegerFromHex(),
    s = this.s.toBigIntegerFromHex(),
    v = this.v.toULong(),
    yParity = this.getyParity()?.toULongFromHex(),
    type = mapType(this.type), // Optional field for EIP-2718 typed transactions
    chainId = chainId, // Optional field for EIP-155 transactions
    gasPrice = gasPrice, // Optional field for EIP-1559 transactions
    maxFeePerGas = maxFeePerGas, // Optional field for EIP-1559 transactions
    maxPriorityFeePerGas = maxPriorityFeePerGas, // Optional field for EIP-1559 transactions,
    accessList = accessList,
    codeDelegations = codeDelegations,
  )
  return domainTx
}
