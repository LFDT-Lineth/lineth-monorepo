package net.consensys.linea.contract

import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.web3j.AtomicContractEIP1559GasProvider
import net.consensys.linea.web3j.EIP1559GasFees
import net.consensys.linea.web3j.EIP4844GasFees
import net.consensys.linea.web3j.EIP4844GasProvider
import net.consensys.linea.web3j.Eip4844Transaction
import net.consensys.linea.web3j.SmartContractErrors
import net.consensys.linea.web3j.informativeEthCall
import net.consensys.toBigInteger
import net.consensys.toGWei
import net.consensys.toULong
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCaps
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.datatypes.Function
import org.web3j.crypto.Blob
import org.web3j.crypto.BlobUtils
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.gas.ContractEIP1559GasProvider
import org.web3j.tx.gas.ContractGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.CompletableFuture

class Web3JContractAsyncHelper(
  val contractAddress: String,
  val web3j: Web3j,
  val transactionManager: AsyncFriendlyTransactionManager,
  private val gasProvider: ContractGasProvider,
  private val smartContractErrors: SmartContractErrors
) {
  private val log: Logger = LogManager.getLogger(this::class.java)

  fun getCurrentBlock(): SafeFuture<BigInteger> {
    return web3j.ethBlockNumber().sendAsync()
      .thenApply { it.blockNumber }
      .toSafeFuture()
  }

  fun createEip4844Transaction(
    function: Function,
    blobs: List<Blob>,
    gasPriceCaps: GasPriceCaps? = null
  ): Eip4844Transaction {
    require(blobs.size in 1..6) { "Blobs size=${blobs.size} must be between 1 and 6." }

    val gasLimit = gasProvider.getGasLimit(function.name)
    val (_, maxFeePerBlobGas) = getEip4844GasFees()
    return Eip4844Transaction.createFunctionCallTransaction(
      from = transactionManager.fromAddress,
      to = contractAddress,
      data = FunctionEncoder.encode(function),
      blobs = blobs,
      maxFeePerBlobGas = gasPriceCaps?.maxFeePerBlobGasCap?.toBigInteger() ?: maxFeePerBlobGas.toBigInteger(),
      maxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger(),
      maxFeePerGas = gasPriceCaps?.maxFeePerGasCap?.toBigInteger(),
      gasLimit = gasLimit,
      blobVersionedHashes = blobs.map { BlobUtils.kzgToVersionedHash(BlobUtils.getCommitment(it)) }
    )
  }

  private fun isGasProviderSupportedEIP1559(): Boolean {
    return gasProvider is ContractEIP1559GasProvider && gasProvider.isEIP1559Enabled
  }

  private fun getEip1559GasFees(
    functionName: String
  ): EIP1559GasFees {
    return when (gasProvider) {
      is AtomicContractEIP1559GasProvider -> gasProvider.getEIP1559GasFees()
      is ContractEIP1559GasProvider -> EIP1559GasFees(
        maxPriorityFeePerGas = gasProvider.getMaxPriorityFeePerGas(functionName).toULong(),
        maxFeePerGas = gasProvider.getMaxFeePerGas(functionName).toULong()
      )

      else -> throw UnsupportedOperationException("GasProvider does not support EIP1559!")
    }
  }

  private fun getEip4844GasFees(): EIP4844GasFees {
    if (gasProvider !is EIP4844GasProvider) {
      throw UnsupportedOperationException("GasProvider does not support EIP4844!")
    }
    return gasProvider.getEIP4844GasFees()
  }

  @Synchronized
  fun sendTransaction(
    function: Function,
    weiValue: BigInteger
  ): EthSendTransaction {
    val encodedData = FunctionEncoder.encode(function)
    val sendRawTransactionResult: EthSendTransaction =
      if (isGasProviderSupportedEIP1559()) {
        val (maxPriorityFeePerGas, maxFeePerGas) = getEip1559GasFees(function.name)
        transactionManager.sendEIP1559Transaction(
          (gasProvider as ContractEIP1559GasProvider).chainId,
          maxPriorityFeePerGas.toBigInteger(),
          maxFeePerGas.toBigInteger(),
          gasProvider.getGasLimit(function.name),
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
  fun sendTransactionAsync(
    function: Function,
    weiValue: BigInteger,
    gasPriceCaps: GasPriceCaps? = null
  ): CompletableFuture<EthSendTransaction> {
    val transaction = if (isGasProviderSupportedEIP1559()) {
      val (maxPriorityFeePerGas, maxFeePerGas) = getEip1559GasFees(function.name)

      logGasPriceCapsInfo(
        logMessagePrefix = function.name,
        maxPriorityFeePerGas = maxPriorityFeePerGas,
        maxFeePerGas = maxFeePerGas,
        dynamicMaxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap,
        dynamicMaxFeePerGas = gasPriceCaps?.maxFeePerGasCap
      )

      transactionManager.createRawTransaction(
        chainId = (gasProvider as ContractEIP1559GasProvider).chainId,
        maxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger()
          ?: maxPriorityFeePerGas.toBigInteger(),
        maxFeePerGas = gasPriceCaps?.maxFeePerGasCap?.toBigInteger() ?: maxFeePerGas.toBigInteger(),
        gasLimit = gasProvider.getGasLimit(function.name),
        to = contractAddress,
        value = weiValue,
        data = FunctionEncoder.encode(function)
      )
    } else {
      transactionManager.createRawTransaction(
        gasPrice = gasProvider.getGasPrice(function.name),
        gasLimit = gasProvider.getGasLimit(function.name),
        to = contractAddress,
        value = weiValue,
        data = FunctionEncoder.encode(function)
      )
    }
    val signedMessage = transactionManager.sign(transaction)
    return web3j.ethSendRawTransaction(signedMessage).sendAsync()
  }

  fun sendBlobCarryingTransactionAndGetTxHash(
    function: Function,
    blobs: List<ByteArray>,
    gasPriceCaps: GasPriceCaps?
  ): SafeFuture<String> {
    require(blobs.size in 0..6) { "Blobs size=${blobs.size} must be between 0 and 6." }
    return sendBlobCarryingTransaction(function, BigInteger.ZERO, blobs.toWeb3JTxBlob(), gasPriceCaps)
      .toSafeFuture()
      .thenApply { result ->
        throwExceptionIfJsonRpcErrorReturned("eth_sendRawTransaction", result)
        result.transactionHash
      }
  }

  @Synchronized
  private fun sendBlobCarryingTransaction(
    function: Function,
    weiValue: BigInteger,
    blobs: List<Blob>,
    gasPriceCaps: GasPriceCaps? = null
  ): CompletableFuture<EthSendTransaction> {
    val (eip1559fees, maxFeePerBlobGas) = getEip4844GasFees()
    val eip4844GasProvider = gasProvider as EIP4844GasProvider

    logGasPriceCapsInfo(
      logMessagePrefix = function.name,
      maxPriorityFeePerGas = eip1559fees.maxPriorityFeePerGas,
      maxFeePerGas = eip1559fees.maxFeePerGas,
      maxFeePerBlobGas = maxFeePerBlobGas,
      dynamicMaxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap,
      dynamicMaxFeePerGas = gasPriceCaps?.maxFeePerGasCap,
      dynamicMaxFeePerBlobGas = gasPriceCaps?.maxFeePerBlobGasCap
    )

    val transaction = transactionManager.createRawTransaction(
      blobs = blobs,
      chainId = eip4844GasProvider.chainId,
      maxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger()
        ?: eip1559fees.maxPriorityFeePerGas.toBigInteger(),
      maxFeePerGas = gasPriceCaps?.maxFeePerGasCap?.toBigInteger() ?: eip1559fees.maxFeePerGas.toBigInteger(),
      gasLimit = eip4844GasProvider.getGasLimit(function.name),
      to = contractAddress,
      data = FunctionEncoder.encode(function),
      value = weiValue,
      maxFeePerBlobGas = gasPriceCaps?.maxFeePerBlobGasCap?.toBigInteger() ?: maxFeePerBlobGas.toBigInteger()
    )
    val signedMessage = transactionManager.sign(transaction)
    return web3j.ethSendRawTransaction(signedMessage).sendAsync()
  }

  @Synchronized
  fun executeRemoteCallTransaction(
    function: Function,
    weiValue: BigInteger
  ): RemoteFunctionCall<TransactionReceipt> {
    val encodedData = FunctionEncoder.encode(function)
    val transactionSent = sendTransaction(function, weiValue)
    return transactionManager.waitForTransaction(function, encodedData, weiValue, transactionSent)
  }

  @Synchronized
  fun executeRemoteCallTransaction(
    function: Function
  ): RemoteFunctionCall<TransactionReceipt> {
    return executeRemoteCallTransaction(function, BigInteger.ZERO)
  }

  fun executeEthCall(function: Function): SafeFuture<String?> {
    val gasLimit = gasProvider.getGasLimit(function.name)
    val tx = Transaction.createFunctionCallTransaction(
      transactionManager.fromAddress,
      null,
      null,
      gasLimit,
      contractAddress,
      FunctionEncoder.encode(function)
    )

    return web3j.informativeEthCall(tx, smartContractErrors)
  }

  private fun logGasPriceCapsInfo(
    logMessagePrefix: String? = "",
    maxPriorityFeePerGas: ULong,
    maxFeePerGas: ULong,
    maxFeePerBlobGas: ULong? = null,
    dynamicMaxPriorityFeePerGas: ULong?,
    dynamicMaxFeePerGas: ULong?,
    dynamicMaxFeePerBlobGas: ULong? = null
  ) {
    val withBlob = maxFeePerBlobGas != null || dynamicMaxFeePerBlobGas != null
    log.info(
      "$logMessagePrefix gas price caps: " +
        "blobCarrying=$withBlob " +
        "maxPriorityFeePerGas=${maxPriorityFeePerGas.toGWei()} GWei, " +
        "dynamicMaxPriorityFeePerGas=${dynamicMaxPriorityFeePerGas?.toGWei()} GWei, " +
        "maxFeePerGas=${maxFeePerGas.toGWei()} GWei, " +
        "dynamicMaxFeePerGas=${dynamicMaxFeePerGas?.toGWei()} GWei, " +
        if (withBlob) {
          "maxFeePerBlobGas=${maxFeePerBlobGas?.toGWei()} GWei, " +
            "dynamicMaxFeePerBlobGas=${dynamicMaxFeePerBlobGas?.toGWei()} GWei"
        } else {
          ""
        }
    )
  }
}
