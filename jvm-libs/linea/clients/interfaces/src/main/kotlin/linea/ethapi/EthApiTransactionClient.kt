package linea.ethapi

import linea.domain.Transaction
import linea.domain.TransactionReceipt
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface EthApiTransactionClient {
  fun ethGetTransactionByHash(transactionHash: ByteArray): SafeFuture<Transaction?>
  fun ethGetTransactionReceipt(transactionHash: ByteArray): SafeFuture<TransactionReceipt?>

  // fun ethGetTransactionByBlockHashAndIndex(blockHash: ByteArray, transactionIndex: ULong): SafeFuture<Transaction?>
  // fun ethGetTransactionByBlockNumberAndIndex(blockParameter: BlockParameter, transactionIndex: ULong): SafeFuture<Transaction?>
  fun ethSendRawTransaction(signedTransactionData: ByteArray): SafeFuture<ByteArray>
}
