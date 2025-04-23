package net.consensys.zkevm.ethereum

import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.LineaRollupAsyncFriendly
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.http.HttpService
import org.web3j.tx.gas.StaticEIP1559GasProvider
import org.web3j.tx.response.PollingTransactionReceiptProcessor
import org.web3j.utils.Async
import java.math.BigInteger

object ContractHelper {
  private val gwei = BigInteger.valueOf(1000000000L)
  private val maxFeePerGas = gwei.multiply(BigInteger.valueOf(5L))
  private val l1Client =
    Web3j.build(HttpService("http://localhost:8445"), 1000, Async.defaultExecutorService())

  // WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
  private const val privateKey = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
  private val credentials = Credentials.create(privateKey)

  private val pollingTransactionReceiptProcessor = PollingTransactionReceiptProcessor(l1Client, 1000, 40)

  fun getL1Client(): Web3j = l1Client
  fun getTransactionManager(privateKey: String): AsyncFriendlyTransactionManager {
    val credentials = Credentials.create(privateKey)
    return AsyncFriendlyTransactionManager(
      l1Client,
      credentials,
      pollingTransactionReceiptProcessor
    )
  }
  fun getmaxFeePerGas(): BigInteger = maxFeePerGas

  fun connectToZkevmContract(
    l1Client: Web3j,
    maxFeePerGas: BigInteger,
    gasLimit: BigInteger,
    contractAddress: String,
    asyncFriendlyTransactionManager: AsyncFriendlyTransactionManager
  ): LineaRollupAsyncFriendly {
    val gasProvider = StaticEIP1559GasProvider(
      l1Client.ethChainId().send().chainId.toLong(),
      maxFeePerGas,
      maxFeePerGas.minus(BigInteger.valueOf(100)),
      gasLimit
    )
    return LineaRollupAsyncFriendly.load(
      contractAddress,
      l1Client,
      asyncFriendlyTransactionManager,
      gasProvider,
      emptyMap()
    )
  }

  @Deprecated("do not use this. It relies on a hardcoded private key")
  fun getCurrentTransactionCount(): BigInteger {
    return l1Client.ethGetTransactionCount(credentials.address, DefaultBlockParameter.valueOf("latest"))
      .send().transactionCount
  }
}
