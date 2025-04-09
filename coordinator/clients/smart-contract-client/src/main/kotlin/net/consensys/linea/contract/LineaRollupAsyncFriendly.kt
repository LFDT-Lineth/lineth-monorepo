package net.consensys.linea.contract

import build.linea.contract.LineaRollupV6
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.web3j.SmartContractErrors
import linea.web3j.gas.AtomicContractEIP1559GasProvider
import linea.web3j.gas.EIP1559GasFees
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.datatypes.Function
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.gas.ContractEIP1559GasProvider
import org.web3j.tx.gas.ContractGasProvider
import org.web3j.tx.response.TransactionReceiptProcessor
import java.math.BigInteger

/**
 * This class shall be replaced by Web3JLineaRollupSmartContractClient
 * Please do not extend or add functionality to this class
 */
class LineaRollupAsyncFriendly(
  contractAddress: String,
  web3j: Web3j,
  private val asyncTransactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  private val smartContractErrors: SmartContractErrors
) : LineaRollupV6(
  contractAddress,
  web3j,
  asyncTransactionManager,
  contractGasProvider
) {
  companion object {
    fun load(
      contractAddress: String,
      web3j: Web3j,
      credentials: Credentials,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors
    ): LineaRollupAsyncFriendly {
      return LineaRollupAsyncFriendly(
        contractAddress,
        web3j,
        AsyncFriendlyTransactionManager(web3j, credentials),
        contractGasProvider,
        smartContractErrors
      )
    }

    fun load(
      contractAddress: String,
      web3j: Web3j,
      transactionManager: AsyncFriendlyTransactionManager,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors
    ): LineaRollupAsyncFriendly {
      return LineaRollupAsyncFriendly(
        contractAddress,
        web3j,
        transactionManager,
        contractGasProvider,
        smartContractErrors
      )
    }

    fun load(
      contractAddress: String,
      web3j: Web3j,
      credentials: Credentials,
      transactionReceiptProcessor: TransactionReceiptProcessor,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors
    ): LineaRollupAsyncFriendly {
      return LineaRollupAsyncFriendly(
        contractAddress,
        web3j,
        AsyncFriendlyTransactionManager(web3j, credentials, transactionReceiptProcessor),
        contractGasProvider,
        smartContractErrors
      )
    }
  }

  private fun getEip1559GasFees(
    eip1559GasProvider: ContractEIP1559GasProvider,
    functionName: String
  ): EIP1559GasFees {
    return if (gasProvider is AtomicContractEIP1559GasProvider) {
      val gasFees = (eip1559GasProvider as AtomicContractEIP1559GasProvider).getEIP1559GasFees()
      EIP1559GasFees(gasFees.maxPriorityFeePerGas, gasFees.maxFeePerGas)
    } else {
      EIP1559GasFees(
        maxPriorityFeePerGas = eip1559GasProvider.getMaxPriorityFeePerGas(functionName).toULong(),
        maxFeePerGas = eip1559GasProvider.getMaxFeePerGas(functionName).toULong()
      )
    }
  }

  @Synchronized
  private fun sendTransaction(
    function: Function,
    weiValue: BigInteger
  ): EthSendTransaction {
    val encodedData = FunctionEncoder.encode(function)
    val sendRawTransactionResult: EthSendTransaction =
      if (gasProvider is ContractEIP1559GasProvider &&
        (gasProvider as ContractEIP1559GasProvider).isEIP1559Enabled
      ) {
        val eip1559GasProvider = gasProvider as ContractEIP1559GasProvider
        val (maxPriorityFeePerGas, maxFeePerGas) = getEip1559GasFees(eip1559GasProvider, function.name)
        transactionManager.sendEIP1559Transaction(
          eip1559GasProvider.chainId,
          maxPriorityFeePerGas.toBigInteger(),
          maxFeePerGas.toBigInteger(),
          eip1559GasProvider.getGasLimit(function.name),
          contractAddress,
          encodedData,
          weiValue,
          false
        )
      } else {
        transactionManager.sendTransaction(
          gasProvider.getGasPrice(function.name),
          gasProvider.getGasLimit(function.name),
          contractAddress,
          encodedData,
          weiValue,
          false
        )
      }

    return sendRawTransactionResult
  }

  @Synchronized
  override fun executeRemoteCallTransaction(
    function: Function,
    weiValue: BigInteger
  ): RemoteFunctionCall<TransactionReceipt> {
    val encodedData = FunctionEncoder.encode(function)
    val transactionSent = sendTransaction(function, weiValue)
    return asyncTransactionManager.waitForTransaction(function, encodedData, weiValue, transactionSent)
  }

  @Synchronized
  override fun executeRemoteCallTransaction(
    function: Function
  ): RemoteFunctionCall<TransactionReceipt> {
    return executeRemoteCallTransaction(function, BigInteger.ZERO)
  }
}
