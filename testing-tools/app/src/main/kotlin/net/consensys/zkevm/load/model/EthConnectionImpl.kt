package net.consensys.zkevm.load.model

import net.consensys.zkevm.load.LineaEstimateGasResponse
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.slf4j.LoggerFactory
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthGetBalance
import org.web3j.protocol.core.methods.response.EthGetTransactionReceipt
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.protocol.core.methods.response.EthTransaction
import org.web3j.protocol.http.HttpService
import org.web3j.tuples.generated.Tuple2
import org.web3j.utils.Numeric
import java.io.IOException
import java.math.BigInteger
import java.net.SocketTimeoutException
import java.time.Instant
import java.time.temporal.ChronoUnit
import java.util.Optional
import java.util.concurrent.ConcurrentHashMap
import java.util.function.Function
import java.util.stream.Collectors
import kotlin.collections.HashMap
import kotlin.time.Duration.Companion.minutes
import kotlin.time.toJavaDuration

class EthConnectionImpl(url: String?) : EthConnection {
  private val httpService: HttpService = run {
    val builder = HttpService.getOkHttpClientBuilder()
    builder.readTimeout(5.minutes.toJavaDuration())
    HttpService(url, builder.build())
  }
  private val web3: Web3j = Web3j.build(httpService)
  private val logger = LoggerFactory.getLogger(EthConnectionImpl::class.java)

  override fun getBalance(sourceWallet: Wallet): BigInteger? {
    var balance: EthGetBalance? = null
    try {
      balance = web3.ethGetBalance(sourceWallet.encodedAddress(), DefaultBlockParameterName.LATEST).send()
    } catch (e: IOException) {
      logger.warn("[TRANSFER] Exception while getting account balance: $e")
    }
    logger.debug("[TRANSFER] Account balance: " + balance!!.balance)
    return balance.balance
  }

  override fun ethSendRawTransaction(
    rawTransaction: RawTransaction?,
    sourceWallet: Wallet,
    chainId: Int,
  ): Request<*, EthSendTransaction> {
    val signedMessage = TransactionEncoder.signMessage(rawTransaction, chainId.toLong(), sourceWallet.credentials)
    val hexValue = Numeric.toHexString(signedMessage)
    return web3.ethSendRawTransaction(hexValue)
  }

  override fun ethGetTransactionCount(
    sourceOfFundsAddress: String?,
    defaultBlockParameterName: DefaultBlockParameterName?,
  ): BigInteger {
    return web3.ethGetTransactionCount(sourceOfFundsAddress, defaultBlockParameterName).send().transactionCount
  }

  override fun estimateGas(transaction: Transaction): BigInteger {
    val estimationResponse = web3.ethEstimateGas(transaction).send()
    try {
      return estimationResponse.amountUsed
    } catch (e: Exception) {
      if (estimationResponse.error != null) {
        throw RuntimeException(estimationResponse.error.message, e)
      } else {
        throw e
      }
    }
  }

  @Throws(IOException::class)
  override fun ethGasPrice(): BigInteger {
    return web3.ethGasPrice().send().gasPrice // .add(BigInteger.valueOf(100000))
  }

  @Throws(IOException::class)
  override fun lineaEstimateGas(transaction: Transaction): LineaEstimateGasResponse {
    val lineaEstimateGasResponseRequest = Request(
      "linea_estimateGas",
      listOf(
        transaction,
      ),
      httpService,
      LineaEstimateGasResponse::class.java,
    )
    return httpService.send(lineaEstimateGasResponseRequest, LineaEstimateGasResponse::class.java)
  }

  fun ethGetTransactionByHash(s: String?): Request<*, EthTransaction> {
    return web3.ethGetTransactionByHash(s)
  }

  override fun estimateGasPriceAndLimit(transactionForEstimation: Transaction): Pair<BigInteger, BigInteger> {
    val gasEstimation = lineaEstimateGas(transactionForEstimation).getGasEstimation()
    return if (gasEstimation != null) {
      val gasPrice = gasEstimation.priorityFeePerGas + gasEstimation.baseFeePerGas
      val gasLimit = gasEstimation.gasLimit
      gasPrice to gasLimit
    } else {
      val gasPrice = ethGasPrice()
      val gasLimit = estimateGas(transactionForEstimation)
      gasPrice to gasLimit
    }
  }

  private fun innerSendAllTransactions(transactions: Map<Wallet, List<TransactionDetail>>): Map<Wallet, BigInteger> {
    return transactions.entries.parallelStream().collect(
      Collectors.toMap(
        { (key): Map.Entry<Wallet, List<TransactionDetail>> -> key },
        Function<Map.Entry<Wallet, List<TransactionDetail>>, BigInteger> { (key, value):
        Map.Entry<Wallet, List<TransactionDetail>>,
          ->
          val sorted =
            value.stream().sorted { s: TransactionDetail, t: TransactionDetail -> 1 * s.nonce.compareTo(t.nonce) }
              .toList()
          if (sorted.size == 0) {
            return@Function key.initialNonce!!
          } else {
            sendAllTransactions(sorted[0], sorted.subList(1, sorted.size))
          }
        },
      ),
    )
  }

  private fun sendAllTransactions(first: TransactionDetail, remaining: List<TransactionDetail>): BigInteger {
    val succeeded: MutableSet<BigInteger> = ConcurrentHashMap.newKeySet()
    val failed: MutableSet<BigInteger> = ConcurrentHashMap.newKeySet()
    // sending in reverse nonce order to fill the pools before they get executed!
    val nonceToHash: MutableMap<BigInteger, String> = HashMap()
    val list =
      remaining.stream().parallel().map { transaction: TransactionDetail ->
        try {
          val walletId = transaction.walletId
          logger.debug("Wallet id:{}, sending transaction with nonce {}", walletId, transaction.nonce)
          val res = transaction.ethSendTransactionRequest.send()
          transaction.hash = res.transactionHash
          if (res.error == null) {
            logger.debug(
              "Wallet id:{}, transaction id:{} nonce:{} with hash:{} was sent.",
              walletId,
              res.id,
              transaction.nonce,
              res.transactionHash,
            )
            if (transaction.expectedOutcome == ExpectedOutcome.SUCCESS) {
              succeeded.add(transaction.nonce)
            } else {
              failed.add(transaction.nonce)
            }
          } else {
            logger.info(
              "transaction id:{}, {}-{} for walletId:{} has error:{}",
              res.id,
              transaction.nonce,
              res.transactionHash,
              walletId,
              res.error.message,
            )
            failed.add(transaction.nonce)
          }
          return@map Tuple2<TransactionDetail, Optional<EthSendTransaction>>(
            transaction,
            Optional.of<EthSendTransaction>(res),
          )
        } catch (e: IOException) {
          logger.error("Error while sending transaction " + transaction.ethSendTransactionRequest.id)
        }
        Tuple2(transaction, Optional.empty<EthSendTransaction>())
      }
        .collect(Collectors.toList())

    try {
      val res = first.ethSendTransactionRequest.send()
      if (res.error == null) {
        first.hash = (res.transactionHash)
        logger.debug(
          "Wallet id:{}, transaction id:{} nonce:{} with hash:{} was sent.",
          first.walletId,
          res.id,
          first.nonce,
          res.transactionHash,
        )
        if (first.expectedOutcome == ExpectedOutcome.SUCCESS) {
          succeeded.add(first.nonce)
        } else {
          failed.add(first.nonce)
        }
        nonceToHash[first.nonce] = res.transactionHash
      } else {
        logger.info(
          "transactionId:{}-{} for walletId:{} error:{}",
          res.id,
          res.transactionHash,
          first.walletId,
          res.error.message,
        )
        failed.add(first.nonce)
      }
    } catch (e: IOException) {
      logger.error("Error while sending transaction " + first.ethSendTransactionRequest.id)
    }
    nonceToHash.putAll(
      list.stream().collect(
        Collectors.toMap(
          { t: Tuple2<TransactionDetail, Optional<EthSendTransaction>> -> t.component1().nonce },
          { t: Tuple2<TransactionDetail, Optional<EthSendTransaction>> -> getTransactionHash(t) },
        ),
      ),
    )
    logger.info(
      "Wallet id: {}, number of transaction sent attempt:{}, succeeded:{}, failed:{}.",
      first.walletId,
      succeeded.size + failed.size,
      succeeded.size,
      failed.size,
    )
    logger.info(
      "Wallet id: {}, number of transaction hash:{}.",
      first.walletId,
      nonceToHash.values.stream().filter { v: String -> !v.startsWith("noTransactionHash") }.count(),
    )
    return if (failed.isEmpty()) {
      succeeded.stream().max { obj: BigInteger, `val`: BigInteger? -> obj.compareTo(`val`) }.get()
    } else {
      failed.stream().min { obj: BigInteger, `val`: BigInteger? -> obj.compareTo(`val`) }.get()
        .subtract(BigInteger.ONE)
    }
  }

  private fun getTransactionHash(t: Tuple2<TransactionDetail, Optional<EthSendTransaction>>): String {
    return if (t.component2().isEmpty || t.component2().get().error != null) {
      "noTransactionHash" + if (t.component2().isEmpty) ": " else t.component2().get().error.message
    } else {
      t.component2().get().transactionHash
    }
  }

  private fun getNonceWalletId(t: Tuple2<TransactionDetail, Optional<EthSendTransaction>>, maxWallet: Int): BigInteger {
    return t.component1().nonce.multiply(BigInteger.valueOf(maxWallet.toLong()))
      .add(BigInteger.valueOf(t.component1().walletId.toLong()))
  }

  override fun sendAllTransactions(transactions: Map<Wallet, List<TransactionDetail>>): Map<Wallet, BigInteger> {
    logger.info("Sending the transactions.")
    val startTime = Instant.now()

    // sending all transactions
    val targetNoncePerWallets = innerSendAllTransactions(transactions)
    logger.info(
      "{} walets used to send {} requests in {}s. Waiting for their completion.",
      transactions.size,
      transactions.values.stream().mapToInt { v: List<TransactionDetail> -> v.size }.sum(),
      Instant.now().epochSecond - startTime.epochSecond,
    )
    return targetNoncePerWallets
  }

  override fun getEthGetBlockByNumber(blockId: Long): EthBlock {
    return web3.ethGetBlockByNumber(
      DefaultBlockParameter.valueOf(BigInteger.valueOf(blockId)),
      true,
    ).send()
  }

  override fun getNonce(encodedAddress: String?): BigInteger {
    return getNonce(encodedAddress, 3)
  }

  fun getNonce(encodedAddress: String?, attemptNb: Int): BigInteger {
    if (attemptNb <= 0) {
      throw Exception("Failed to get nonce after max attempts.")
    }
    try {
      val ethGetTransactionCount = ethGetTransactionCount(
        encodedAddress,
        DefaultBlockParameterName.LATEST,
      )
      return ethGetTransactionCount
    } catch (e: SocketTimeoutException) {
      logger.error("Failed to get nonce, retrying. {} attempts lefts.", attemptNb)
      return getNonce(encodedAddress, attemptNb - 1)
    }
  }

  override fun ethGetTransactionReceipt(transactionHash: String?): Request<*, EthGetTransactionReceipt> {
    return web3.ethGetTransactionReceipt(transactionHash)
  }

  override fun getCurrentEthBlockNumber(): BigInteger {
    return web3.ethBlockNumber().send().blockNumber
  }

  override fun logger(): Logger {
    return LogManager.getLogger(EthConnectionImpl::class.java)
  }
}

data class GasAndFees(val gas: BigInteger, val minerTip: BigInteger, val baseFee: BigInteger)

interface EthConnection {
  fun logger(): Logger
  fun ethGasPrice(): BigInteger
  fun lineaEstimateGas(transaction: Transaction): LineaEstimateGasResponse

  fun estimateGasPriceAndLimit(transactionForEstimation: Transaction): Pair<BigInteger, BigInteger>
  fun ethSendRawTransaction(
    rawTransaction: RawTransaction?,
    sourceWallet: Wallet,
    chainId: Int,
  ): Request<*, EthSendTransaction>

  fun getBalance(sourceWallet: Wallet): BigInteger?
  fun getNonce(encodedAddress: String?): BigInteger
  fun ethGetTransactionCount(
    sourceOfFundsAddress: String?,
    defaultBlockParameterName: DefaultBlockParameterName?,
  ): BigInteger

  fun estimateGas(transaction: Transaction): BigInteger

  @Throws(InterruptedException::class)
  fun waitForTransactions(targetNonce: Map<Wallet, BigInteger?>, timeoutDelay: Long = 600L) {
    val startTime = Instant.now()
    val deadline = startTime.plus(timeoutDelay, ChronoUnit.SECONDS)
    var currentNoncePerWallet = targetNonce.keys.stream()
      .collect(Collectors.toMap({ e: Wallet -> e }, { e: Wallet -> getNonce(Numeric.prependHexPrefix(e.address)) }))
    var timedOut = false
    while (currentNoncePerWallet.isNotEmpty()) {
      val newNonces: MutableMap<Wallet, BigInteger> = ConcurrentHashMap()
      currentNoncePerWallet.entries.parallelStream().forEach { (key): Map.Entry<Wallet, BigInteger> ->
        val entryNewNonce = getNonce(Numeric.prependHexPrefix(key.address))
        if (entryNewNonce.compareTo(targetNonce[key]) <= 0) {
          newNonces[key] = entryNewNonce
        }
      }
      logger().info("{} wallets with remaining transactions to complete.", newNonces.size)
      logger().info("List of wallets: {}", newNonces.keys.map { w -> w.address }.toSet())
      Thread.sleep(300)
      if (Instant.now().isAfter(deadline)) {
        timedOut = true
        logger().debug("Timed out.")
        break
      }
      currentNoncePerWallet = newNonces
    }
    if (timedOut) {
      logger().error("Time out after {}s", Instant.now().epochSecond - startTime.epochSecond)
      throw InterruptedException()
    } else {
      logger().info("Completion time " + (Instant.now().epochSecond - startTime.epochSecond) + "s")
    }
  }

  fun ethGetTransactionReceipt(transactionHash: String?): Request<*, EthGetTransactionReceipt>
  fun getCurrentEthBlockNumber(): BigInteger
  fun sendAllTransactions(transactions: Map<Wallet, List<TransactionDetail>>): Map<Wallet, BigInteger>
  fun getEthGetBlockByNumber(blockId: Long): EthBlock

  companion object {
    @JvmField
    val SIMPLE_TX_PRICE = BigInteger.valueOf(21000L)
  }
}
