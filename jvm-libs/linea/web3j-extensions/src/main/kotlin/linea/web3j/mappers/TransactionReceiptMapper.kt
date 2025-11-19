package linea.web3j.mappers

import linea.domain.TransactionReceipt
import linea.domain.TransactionType
import linea.kotlin.decodeHex
import linea.kotlin.fromHexString
import linea.kotlin.toULong
import linea.web3j.domain.toDomain

fun org.web3j.protocol.core.methods.response.TransactionReceipt.toDomain(): TransactionReceipt {
  return TransactionReceipt(
    transactionHash = this.transactionHash.decodeHex(),
    transactionIndex = this.transactionIndex.toULong(),
    blockHash = this.blockHash.decodeHex(),
    blockNumber = this.blockNumber.toULong(),
    from = this.from.decodeHex(),
    to = this.to?.decodeHex(), // Nullable for contract creation transactions
    cumulativeGasUsed = this.cumulativeGasUsed.toULong(),
    gasUsed = this.gasUsed.toULong(),
    contractAddress = this.contractAddress?.decodeHex(), // Nullable, only present for contract creation
    logs = this.logs.map { it.toDomain() },
    logsBloom = this.logsBloom.decodeHex(),
    status = this.status?.let(ULong::fromHexString), // 1 for success, 0 for failure, null for pre-Byzantium
    root = this.root?.decodeHex(), // State root for pre-Byzantium transactions, null for post-Byzantium
    effectiveGasPrice = ULong.fromHexString(this.effectiveGasPrice),
    type = TransactionType.fromEthApiSerializedValue(this.type?.let { ULong.fromHexString(it).toInt() } ?: 0),
    blobGasUsed = this.blobGasUsed?.let(ULong::fromHexString), // EIP-4844 blob gas used, null for non-blob transactions
    blobGasPrice = this.blobGasPrice?.let(
      ULong::fromHexString,
    ), // EIP-4844 blob gas price, null for non-blob transactions
  )
}
