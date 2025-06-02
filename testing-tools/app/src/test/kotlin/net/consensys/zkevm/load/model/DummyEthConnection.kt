package net.consensys.zkevm.load.model

import io.reactivex.Flowable
import net.consensys.zkevm.load.LineaEstimateGasResponse
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.mockito.Mockito.mock
import org.web3j.crypto.RawTransaction
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.BatchRequest
import org.web3j.protocol.core.BatchResponse
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthGetTransactionReceipt
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.protocol.websocket.events.Notification
import java.math.BigInteger
import java.util.Arrays
import java.util.concurrent.CompletableFuture
import kotlin.collections.HashMap

class DummyEthConnection : EthConnection {
  override fun ethGasPrice(): BigInteger {
    return BigInteger.valueOf(4000)
  }

  override fun lineaEstimateGas(transaction: Transaction): LineaEstimateGasResponse {
    TODO("Not yet implemented")
  }

  override fun estimateGasPriceAndLimit(transactionForEstimation: Transaction): Pair<BigInteger, BigInteger> {
    return BigInteger.valueOf(4000) to BigInteger.valueOf(4000)
  }

  val wallets: MutableMap<String, DummyWalletL1State> = HashMap()

  override fun ethSendRawTransaction(
    rawTransaction: RawTransaction?,
    sourceWallet: Wallet,
    chainId: Int,
  ): Request<*, EthSendTransaction> {
    wallets.get(sourceWallet.encodedAddress())?.wallet?.initialNonce =
      wallets.get(sourceWallet.encodedAddress())?.wallet?.initialNonce?.add(BigInteger.ONE)
    val web3Service: Web3jService = DummyWeb3jService()
    return Request(
      "eth_sendRawTransaction",
      Arrays.asList(rawTransaction?.transaction?.data),
      web3Service,
      EthSendTransaction::class.java,
    )
  }

  override fun getBalance(sourceWallet: Wallet): BigInteger? {
    return wallets.get(sourceWallet.encodedAddress())?.balance
  }

  override fun getNonce(encodedAddress: String?): BigInteger {
    return wallets.get(encodedAddress)?.wallet?.initialNonce ?: BigInteger.ZERO
  }

  override fun ethGetTransactionCount(
    sourceOfFundsAddress: String?,
    defaultBlockParameterName: DefaultBlockParameterName?,
  ): BigInteger {
    return wallets.get(sourceOfFundsAddress)?.wallet?.initialNonce!!
  }

  override fun estimateGas(transaction: Transaction): BigInteger {
    TODO("Not yet implemented")
  }

  override fun logger(): Logger {
    return LogManager.getLogger(DummyEthConnection::class.java)
  }

  override fun ethGetTransactionReceipt(transactionHash: String?): Request<*, EthGetTransactionReceipt> {
    TODO("Not yet implemented")
  }

  override fun getCurrentEthBlockNumber(): BigInteger {
    TODO("Not yet implemented")
  }

  override fun sendAllTransactions(transactions: Map<Wallet, List<TransactionDetail>>): Map<Wallet, BigInteger> {
    TODO("Not yet implemented")
  }

  override fun getEthGetBlockByNumber(blockId: Long): EthBlock {
    TODO("Not yet implemented")
  }
}

class DummyWeb3jService : Web3jService {
  override fun <T : Response<*>?> send(request: Request<*, out Response<*>>?, responseType: Class<T>?): T {
    return mock(responseType)
  }

  override fun <T : Response<*>?> sendAsync(request: Request<*, out Response<*>>?, responseType: Class<T>?):
    CompletableFuture<T> {
    TODO("Not yet implemented")
  }

  override fun sendBatch(batchRequest: BatchRequest?): BatchResponse {
    TODO("Not yet implemented")
  }

  override fun sendBatchAsync(batchRequest: BatchRequest?): CompletableFuture<BatchResponse> {
    TODO("Not yet implemented")
  }

  override fun <T : Notification<*>?> subscribe(
    request: Request<*, out Response<*>>?,
    unsubscribeMethod: String?,
    responseType: Class<T>?,
  ): Flowable<T> {
    TODO("Not yet implemented")
  }

  override fun close() {
    TODO("Not yet implemented")
  }
}

class DummyWalletL1State(val wallet: Wallet, val balance: BigInteger)
