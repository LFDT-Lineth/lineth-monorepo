package linea.web3j.mappers

import linea.domain.Block
import linea.domain.BlockData
import linea.domain.BlockWithTxHashes
import linea.domain.Transaction
import linea.kotlin.decodeHex
import linea.kotlin.toULong
import org.web3j.protocol.core.methods.response.EthBlock

fun EthBlock.Block.toDomain(): Block = mapToDomain(this)

fun mapToDomain(web3jBlock: EthBlock.Block): Block {
  return mapToDomain(web3jBlock, ::mapFullTxDataToDomain)
}

fun mapToDomainWithTxHashes(web3jBlock: EthBlock.Block): BlockWithTxHashes {
  return mapToDomain(web3jBlock, ::mapTxHashToByteArray)
}

fun mapFullTxDataToDomain(web3jBlock: EthBlock.Block): List<Transaction> {
  if (web3jBlock.transactions.isNotEmpty() && web3jBlock.transactions[0] !is EthBlock.TransactionObject) {
    throw IllegalArgumentException(
      "Expected to be have full EthBlock.TransactionObject." +
        "Got just transaction hashes.",
    )
  }
  return web3jBlock.transactions.map { (it as EthBlock.TransactionObject).toDomain() }
}

fun mapTxHashToByteArray(web3jBlock: EthBlock.Block): List<ByteArray> {
  if (web3jBlock.transactions.isNotEmpty() && web3jBlock.transactions[0] !is EthBlock.TransactionHash) {
    throw IllegalArgumentException(
      "Expected to be have EthBlock.TransactionHash. Got instance of ${web3jBlock.transactions[0]::class.java}",
    )
  }
  return web3jBlock.transactions.map { (it as EthBlock.TransactionHash).get().decodeHex() }
}

fun <TxData> mapToDomain(web3jBlock: EthBlock.Block, txsMapper: (EthBlock.Block) -> List<TxData>): BlockData<TxData> {
  val block = BlockData(
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
    transactions = txsMapper(web3jBlock), // List of transactions
    size = web3jBlock.size.toULong(),
  )
  return block
}
