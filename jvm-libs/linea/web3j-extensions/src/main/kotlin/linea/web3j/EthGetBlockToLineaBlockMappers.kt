package linea.web3j

import linea.domain.AccessListEntry
import linea.domain.Block
import linea.domain.Transaction
import linea.domain.TransactionType
import net.consensys.decodeHex
import net.consensys.toBigIntegerFromHex
import net.consensys.toIntFromHex
import net.consensys.toULong
import net.consensys.toULongFromHex
import org.web3j.protocol.core.methods.response.EthBlock

fun EthBlock.Block.toDomain(): Block = mapToDomain(this)

fun mapToDomain(web3jBlock: EthBlock.Block): Block {
  val block = Block(
    number = web3jBlock.number.toULong(),
    hash = web3jBlock.hash.decodeHex(),
    parentHash = web3jBlock.parentHash.decodeHex(),
    ommersHash = web3jBlock.sha3Uncles.decodeHex(),
    miner = web3jBlock.miner.decodeHex(),
    nonce = web3jBlock.nonce.toULong(),
    stateRoot = web3jBlock.stateRoot.decodeHex(),
    transactionsRoot = web3jBlock.transactionsRoot.decodeHex(),
    receiptsRoot = web3jBlock.receiptsRoot.decodeHex(),
    logsBloom = web3jBlock.logsBloom.decodeHex(),
    difficulty = web3jBlock.difficulty.toULong(),
    gasLimit = web3jBlock.gasLimit.toULong(),
    gasUsed = web3jBlock.gasUsed.toULong(),
    timestamp = web3jBlock.timestamp.toULong(),
    extraData = web3jBlock.extraData.decodeHex(),
    mixHash = web3jBlock.mixHash.decodeHex(),
    baseFeePerGas = web3jBlock.baseFeePerGas?.toULong(), // Optional field for EIP-1559 blocks
    ommers = web3jBlock.uncles.map { it.decodeHex() }, // List of uncle block hashes
    transactions = run {
      if (web3jBlock.transactions.isNotEmpty() && web3jBlock.transactions[0] !is EthBlock.TransactionObject) {
        throw IllegalArgumentException(
          "Expected to be have full EthBlock.TransactionObject." +
            "Got just transaction hashes."
        )
      }
      web3jBlock.transactions.map { (it as EthBlock.TransactionObject).toDomain() }
    }
  )
  return block
}

fun EthBlock.TransactionObject.toDomain(): Transaction {
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
      accessListEntry.storageKeys.map { it.decodeHex() }
    )
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
    chainId = this.chainId?.toULong(), // Optional field for EIP-155 transactions
    gasPrice = gasPrice, // Optional field for EIP-1559 transactions
    maxFeePerGas = maxFeePerGas, // Optional field for EIP-1559 transactions
    maxPriorityFeePerGas = maxPriorityFeePerGas, // Optional field for EIP-1559 transactions,
    accessList = accessList
  )
  return domainTx
}

fun mapType(type: String?): TransactionType {
  return type
    ?.let { TransactionType.fromEthApiSerializedValue(it.toIntFromHex()) }
    ?: TransactionType.FRONTIER
}
