package net.consensys.zkevm.load

import net.consensys.zkevm.load.model.EthConnection
import net.consensys.zkevm.load.model.TransactionDetail
import net.consensys.zkevm.load.model.Wallet
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.web3j.crypto.RawTransaction
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.utils.Numeric
import java.io.IOException
import java.math.BigInteger
import java.util.function.Consumer

class WalletsFunding(
  private val ethConnection: EthConnection,
  private val sourceOfFundsWallet: Wallet,
) {

  @Throws(IOException::class)
  fun generateTxWithRandomPayload(
    wallets: Map<Int, Wallet>,
    payloadSize: Int,
    chainId: Int,
    nbTransfers: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    val payload = Util.generateRandomPayloadOfSize(payloadSize)
    return generateTxsWithPayload(
      wallets = wallets,
      payLoad = payload,
      chainId = chainId,
      nbTransfers = nbTransfers,
    )
  }

  fun generateTxsWithPayload(
    wallets: Map<Int, Wallet>,
    payLoad: String,
    chainId: Int,
    nbTransfers: Int,
  ): MutableMap<Wallet, List<TransactionDetail>> {
    val result: MutableMap<Wallet, List<TransactionDetail>> = HashMap()

    for (sourceWallet in wallets.values) {
      val txs = ArrayList<TransactionDetail>()
      for (i in 0 until nbTransfers) {
        val transactionForEstimation = Transaction(
          /* from = */
          sourceWallet.address,
          /* nonce = */
          sourceWallet.theoreticalNonceValue,
          /* gasPrice = */
          null,
          /* gasLimit = */
          null,
          /* to = */
          Numeric.prependHexPrefix(sourceWallet.address),
          /* value = */
          null,
          /* data = */
          payLoad,
        )

        val (gasPrice, gasLimit) = ethConnection.estimateGasPriceAndLimit(transactionForEstimation)

        val rawTransaction = RawTransaction.createTransaction(
          /* nonce = */
          sourceWallet.theoreticalNonceValue,
          /* gasPrice = */
          gasPrice,
          /* gasLimit = */
          gasLimit,
          /* to = */
          Numeric.prependHexPrefix(sourceWallet.address),
          /* value = */
          BigInteger.ZERO,
          /* data = */
          payLoad,
        )
        txs.add(
          TransactionDetail(
            sourceWallet.id,
            sourceWallet.theoreticalNonceValue,
            ethConnection.ethSendRawTransaction(rawTransaction, sourceWallet, chainId),
          ),
        )

        sourceWallet.incrementTheoreticalNonce()
      }
      result[sourceWallet] = txs
    }

    return result
  }

  @Throws(IOException::class)
  fun generateUnderPricedTxs(
    wallets: Map<Int, Wallet>,
    chainId: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    val gasUnderPriced = ethConnection.ethGasPrice().multiply(BigInteger.valueOf(80))
      .divide(BigInteger.valueOf(100))
    val result: MutableMap<Wallet, List<TransactionDetail>> = HashMap()
    val costPerCall = EthConnection.SIMPLE_TX_PRICE
    logger.debug("[UNPROFITABLE] estimated cost per call: {}", costPerCall)
    for (sourceWallet in wallets.values) {
      val rawTransaction = RawTransaction.createTransaction(
        sourceWallet.theoreticalNonceValue,
        gasUnderPriced,
        costPerCall,
        Numeric.prependHexPrefix(sourceWallet.address),
        BigInteger.ZERO,
        null,
      )
      result[sourceWallet] = listOf(
        TransactionDetail(
          sourceWallet.id,
          sourceWallet.theoreticalNonceValue,
          ethConnection.ethSendRawTransaction(rawTransaction, sourceWallet, chainId),
        ),
      )
      sourceWallet.incrementTheoreticalNonce()
    }
    return result
  }

  // add some eth to the wallets. Needs to be run once when the wallets are created, or when the balance decreased too much (because of paid gas)
  @Throws(IOException::class, InterruptedException::class)
  fun initializeWallets(
    wallets: Map<Int, Wallet>,
    nbTransactions: Int,
    sourceWallet: Wallet,
    chainId: Int,
    gasPerCall: BigInteger,
    gasPricePerCall: BigInteger,
    valuePerCall: BigInteger,
  ): Map<Wallet, List<TransactionDetail>> {
    val balance = ethConnection.getBalance(sourceWallet)
    logger.info("[FUNDING] source of funds balance is {}.", balance)
    var nonce = ethConnection.getNonce(sourceOfFundsWallet.encodedAddress())
    logger.info("[FUNDING] initial nonce is {}.", nonce)
    // 5% of margin more just to ensure we have a little more than enough
    val ethForTransfers = BigInteger.valueOf((nbTransactions).toLong()).multiply(valuePerCall)
    val ethForGas =
      gasPricePerCall.multiply(BigInteger.valueOf((nbTransactions).toLong())).multiply(gasPerCall)
    val transferredAmount = ethForTransfers.add(ethForGas)
    logger.debug(
      "[FUNDING] gas price is {}, transferred amount is {}.",
      gasPricePerCall,
      transferredAmount,
    )
    val fundingTransfers: ArrayList<TransactionDetail> = ArrayList()
    for ((_, value) in wallets) {
      val rawTx = createFundTransferTransaction(
        wallet = sourceWallet,
        toAddress = value.address,
        nonce = nonce,
        gasPrice = gasPricePerCall,
        gasLimit = EthConnection.SIMPLE_TX_PRICE,
        initialAmount = transferredAmount,
        chainId = chainId,
      )
      fundingTransfers.add(
        TransactionDetail(
          sourceOfFundsWallet.id,
          nonce,
          rawTx,
        ),
      )
      val res = rawTx.send()
      logger.debug(
        "[FUNDING] Transfer fund transaction sent for nonce:{}, hash:{}, {}.",
        nonce,
        res.transactionHash,
        if (res.error != null) " error:" + res.error.message else "no error",
      )
      nonce = nonce.add(BigInteger.ONE)
    }
    logger.info("[FUNDING] Waiting for fund transfer.")
    while (ethConnection.ethGetTransactionCount(
        sourceOfFundsWallet.encodedAddress(),
        DefaultBlockParameterName.LATEST,
      ) < nonce
    ) {
      Thread.sleep(10)
    }
    logger.info("[FUNDING] completed.")

    return mapOf(sourceOfFundsWallet to fundingTransfers)
  }

  private fun createFundTransferTransaction(
    wallet: Wallet,
    toAddress: String,
    nonce: BigInteger,
    gasPrice: BigInteger,
    gasLimit: BigInteger,
    initialAmount: BigInteger,
    chainId: Int,
  ): Request<*, EthSendTransaction> {
    val rawTransaction = RawTransaction.createEtherTransaction(
      nonce,
      gasPrice,
      gasLimit,
      Numeric.prependHexPrefix(toAddress),
      initialAmount,
    )
    return ethConnection.ethSendRawTransaction(rawTransaction, wallet, chainId)
  }

  @Throws(Exception::class)
  fun generateTransactions(
    wallets: Map<Int, Wallet>,
    valueToTransfer: BigInteger,
    nbTransactions: Int,
    chainId: Int,
  ): Map<Wallet, MutableList<TransactionDetail>> {
    // check wallet balance, it helps to ensure wallets exist.
    wallets.values.forEach(Consumer { a: Wallet? -> ethConnection.getBalance(a!!) })
    val initialNoncePerWallet = wallets.entries.associate {
      it.value to ArrayList<TransactionDetail>()
    }
    return getTransactionDetailList(
      wallets,
      valueToTransfer,
      nbTransactions,
      initialNoncePerWallet,
      chainId,
    )
  }

  @Throws(InterruptedException::class)
  fun waitForTransactions(targetNonce: Map<Wallet, BigInteger>, timeoutDelay: Long = 600L) {
    ethConnection.waitForTransactions(targetNonce, timeoutDelay)
  }

  @Throws(IOException::class)
  private fun getTransactionDetailList(
    wallets: Map<Int, Wallet>,
    value: BigInteger,
    nbTransactions: Int,
    initialNoncePerWallet: Map<Wallet, List<TransactionDetail>>,
    chainId: Int,
  ): Map<Wallet, MutableList<TransactionDetail>> {
    val transactions: MutableMap<Wallet, MutableList<TransactionDetail>> = HashMap()
    val gasPrice = ethConnection.ethGasPrice()
    for ((wallet) in initialNoncePerWallet) {
      transactions[wallet] = ArrayList()
      logger.info("[TRANSFER] Preparing transactions for wallet " + wallet.address + " id: " + wallet.id)
      val walletId = wallet.id
      for (i in 0 until nbTransactions) {
        val nonce = wallet.theoreticalNonce.get()
        val toAddress = getDestinationWallet(wallets, wallet, walletId, i)
        val rawTransaction = createFundTransferTransaction(
          wallet,
          toAddress,
          nonce,
          gasPrice,
          EthConnection.SIMPLE_TX_PRICE,
          value,
          chainId,
        )
        transactions[wallet]!!.add(
          TransactionDetail(
            walletId,
            nonce,
            rawTransaction,
          ),
        )
        wallet.incrementTheoreticalNonce()
      }
    }
    return transactions
  }

  private fun getDestinationWallet(
    wallets: Map<Int, Wallet>,
    wallet: Wallet,
    walletId: Int,
    i: Int,
  ): String {
    val walletDestinationId = (walletId + i + 1) % wallets.size
    logger.debug(
      "[TRANSFER] preparing transactions from wallet {} to wallet {}",
      wallet,
      walletDestinationId,
    )
    return wallets[walletDestinationId]!!.address
  }

  companion object {
    val logger: Logger = LoggerFactory.getLogger(WalletsFunding::class.java)!!
  }
}
