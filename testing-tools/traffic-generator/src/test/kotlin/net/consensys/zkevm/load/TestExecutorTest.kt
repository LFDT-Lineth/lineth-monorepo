package net.consensys.zkevm.load

import net.consensys.zkevm.load.model.DummyWeb3jService
import net.consensys.zkevm.load.model.EthConnection
import net.consensys.zkevm.load.model.TransactionDetail
import net.consensys.zkevm.load.model.Wallet
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.web3j.crypto.Keys
import org.web3j.crypto.RawTransaction
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthGetTransactionReceipt
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.nio.file.Files
import java.nio.file.Path

class TestExecutorTest {
  @TempDir
  lateinit var tempDir: Path

  @Test
  fun resyncsSourceWalletNonceFromChainBeforeEstimatingWalletFunding() {
    // Arrange
    val chainNonce = BigInteger.TEN
    val ethConnection = RecordingEthConnection(chainNonce)
    val privateKey = Numeric.toHexStringNoPrefixZeroPadded(Keys.createEcKeyPair().privateKey, 64)
    val requestPath = tempDir.resolve("request.json")
    Files.writeString(
      requestPath,
      """
      {
        "id": 1,
        "name": "nonce sync",
        "context": {
          "chainId": 59141,
          "url": "http://localhost",
          "nbOfExecutions": 1
        },
        "calls": [
          {
            "nbOfExecution": 1,
            "scenario": {
              "scenarioType": "RoundRobinMoneyTransfer",
              "wallet": "new",
              "nbTransfers": 1,
              "nbWallets": 1
            }
          }
        ]
      }
      """.trimIndent(),
    )
    val executor = TestExecutor(requestPath.toString(), privateKey, ethConnection)
    val sourceWalletField = TestExecutor::class.java.getDeclaredField("sourceWallet")
    sourceWalletField.isAccessible = true
    val sourceWallet = sourceWalletField.get(executor) as Wallet
    // Simulate a stale local nonce 3 that must be resynced back to the chain nonce 10 before estimation.
    sourceWallet.theoreticalNonce.set(BigInteger.valueOf(3))

    // Act
    executor.test(0)

    // Assert
    assertEquals(chainNonce, ethConnection.estimatedNonces.first())
  }
}

private class RecordingEthConnection(
  private val chainNonce: BigInteger,
) : EthConnection {
  val estimatedNonces = mutableListOf<BigInteger>()

  override fun logger(): Logger {
    return LogManager.getLogger(RecordingEthConnection::class.java)
  }

  override fun ethGasPrice(): BigInteger {
    return BigInteger.ONE
  }

  override fun lineaEstimateGas(transaction: org.web3j.protocol.core.methods.request.Transaction) =
    throw UnsupportedOperationException()

  override fun estimateGasPriceAndLimit(
    transactionForEstimation: org.web3j.protocol.core.methods.request.Transaction,
  ): Pair<BigInteger, BigInteger> {
    estimatedNonces.add(BigInteger(transactionForEstimation.nonce.removePrefix("0x"), 16))
    return BigInteger.ONE to EthConnection.SIMPLE_TX_PRICE
  }

  override fun ethSendRawTransaction(
    rawTransaction: RawTransaction?,
    sourceWallet: Wallet,
    chainId: Int,
  ): Request<*, EthSendTransaction> {
    return Request(
      "eth_sendRawTransaction",
      listOf<Any>(),
      DummyWeb3jService(),
      EthSendTransaction::class.java,
    )
  }

  override fun getBalance(sourceWallet: Wallet): BigInteger {
    return BigInteger.TEN
  }

  override fun getNonce(encodedAddress: String?): BigInteger {
    return if (encodedAddress == null) BigInteger.ZERO else chainNonce
  }

  override fun ethGetTransactionCount(
    sourceOfFundsAddress: String?,
    defaultBlockParameterName: DefaultBlockParameterName?,
  ): BigInteger {
    return chainNonce.add(BigInteger.ONE)
  }

  override fun estimateGas(transaction: org.web3j.protocol.core.methods.request.Transaction): BigInteger {
    return EthConnection.SIMPLE_TX_PRICE
  }

  override fun waitForTransactions(targetNonce: Map<Wallet, BigInteger?>, timeoutDelay: Long) {
  }

  override fun ethGetTransactionReceipt(transactionHash: String?): Request<*, EthGetTransactionReceipt> {
    throw UnsupportedOperationException()
  }

  override fun getCurrentEthBlockNumber(): BigInteger {
    return BigInteger.ONE
  }

  override fun sendAllTransactions(transactions: Map<Wallet, List<TransactionDetail>>): Map<Wallet, BigInteger> {
    return emptyMap()
  }

  override fun getEthGetBlockByNumber(blockId: Long): EthBlock {
    throw UnsupportedOperationException()
  }
}
