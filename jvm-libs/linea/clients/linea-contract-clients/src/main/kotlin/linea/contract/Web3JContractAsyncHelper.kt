package net.consensys.linea.contract

import linea.domain.gas.GasPriceCaps
import linea.kotlin.toBigInteger
import linea.kotlin.toGWei
import linea.kotlin.toULong
import linea.web3j.SmartContractErrors
import linea.web3j.domain.Eip4844Transaction
import linea.web3j.gas.AtomicContractEIP1559GasProvider
import linea.web3j.gas.EIP1559GasFees
import linea.web3j.gas.EIP4844GasFees
import linea.web3j.gas.EIP4844GasProvider
import linea.web3j.getRevertReason
import linea.web3j.informativeEthCall
import linea.web3j.requestAsync
import linea.web3j.toWeb3jTxBlob
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
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
import kotlin.collections.map

class Web3JContractAsyncHelper(
  val contractAddress: String,
  val web3j: Web3j,
  val transactionManager: AsyncFriendlyTransactionManager,
  private val contractGasProvider: ContractGasProvider,
  private val smartContractErrors: SmartContractErrors,
  private val useEthEstimateGas: Boolean,
) {
  private val log: Logger = LogManager.getLogger(this::class.java)

  fun getCurrentBlock(): SafeFuture<BigInteger> {
    return web3j.ethBlockNumber().sendAsync()
      .thenApply { it.blockNumber }
      .toSafeFuture()
  }

  private fun getGasLimit(
    function: Function,
    blobs: List<Blob>? = null,
    blobVersionedHashes: List<ByteArray>? = null,
  ): SafeFuture<BigInteger> {
    return if (useEthEstimateGas) {
      getEthEstimatedGas(
        FunctionEncoder.encode(function),
        blobs,
        blobVersionedHashes,
      ).thenApply {
        it ?: contractGasProvider.gasLimit
      }
    } else {
      SafeFuture.completedFuture(contractGasProvider.gasLimit)
    }
  }

  private fun getEthEstimatedGas(
    encodedFunction: String,
    blobs: List<Blob>? = null,
    blobVersionedHashes: List<ByteArray>? = null,
  ): SafeFuture<BigInteger?> {
    return if (blobs != null && blobVersionedHashes != null) {
      createEip4844FunctionCallTransaction(encodedFunction, blobs, blobVersionedHashes)
    } else {
      createFunctionCallTransaction(encodedFunction)
    }.run(::callEthEstimateGas)
  }

  private fun callEthEstimateGas(tx: Transaction): SafeFuture<BigInteger?> {
    return web3j.ethEstimateGas(tx).sendAsync()
      .thenApply {
        val withBlobs = tx is Eip4844Transaction
        if (it.hasError()) {
          log.info(
            "eth_estimateGas failed for tx with blobCarrying={} error={} revertReason={}",
            withBlobs,
            it.error.message,
            getRevertReason(it.error, smartContractErrors),
          )
          null
        } else {
          log.debug(
            "eth_estimateGas for tx with blobCarrying={} estimatedGas={}",
            withBlobs,
            it.amountUsed,
          )
          it.amountUsed
        }
      }
      .toSafeFuture()
  }

  private fun createFunctionCallTransaction(encodedFunction: String): Transaction {
    return Transaction.createFunctionCallTransaction(
      transactionManager.fromAddress,
      null,
      null,
      null,
      contractAddress,
      encodedFunction,
    )
  }

  private fun createEip4844FunctionCallTransaction(
    encodedFunction: String,
    blobs: List<Blob>,
    blobVersionedHashes: List<ByteArray>,
  ): Eip4844Transaction {
    return Eip4844Transaction.createFunctionCallTransaction(
      from = transactionManager.fromAddress,
      to = contractAddress,
      data = encodedFunction,
      blobs = blobs,
      maxFeePerBlobGas = null,
      maxPriorityFeePerGas = null,
      maxFeePerGas = null,
      gasLimit = null,
      blobVersionedHashes = blobVersionedHashes,
    )
  }

  private fun createEip4844Transaction(
    function: Function,
    blobs: List<Blob>,
    gasPriceCaps: GasPriceCaps? = null,
  ): SafeFuture<Eip4844Transaction> {
    require(blobs.size in 1..9) { "Blobs size=${blobs.size} must be between 1 and 9." }

    val blobVersionedHashes = blobs
      .map(BlobUtils::getCommitment)
      .map(BlobUtils::kzgToVersionedHash)
      .map(Bytes::toArray)

    return getGasLimit(function, blobs, blobVersionedHashes)
      .thenApply { gasLimit ->
        val (_, maxFeePerBlobGas) = getEip4844GasFees()
        Eip4844Transaction.createFunctionCallTransaction(
          from = transactionManager.fromAddress,
          to = contractAddress,
          data = FunctionEncoder.encode(function),
          blobs = blobs,
          maxFeePerBlobGas = gasPriceCaps?.maxFeePerBlobGasCap?.toBigInteger() ?: maxFeePerBlobGas.toBigInteger(),
          maxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger(),
          maxFeePerGas = gasPriceCaps?.maxFeePerGasCap?.toBigInteger(),
          gasLimit = gasLimit,
          blobVersionedHashes = blobVersionedHashes,
        )
      }
  }

  private fun isGasProviderSupportedEIP1559(): Boolean {
    return contractGasProvider is ContractEIP1559GasProvider
  }

  private fun getEip1559GasFees(): EIP1559GasFees {
    return when (contractGasProvider) {
      is AtomicContractEIP1559GasProvider -> contractGasProvider.getEIP1559GasFees()
      is ContractEIP1559GasProvider -> EIP1559GasFees(
        maxPriorityFeePerGas = contractGasProvider.maxPriorityFeePerGas.toULong(),
        maxFeePerGas = contractGasProvider.maxFeePerGas.toULong(),
      )

      else -> throw UnsupportedOperationException("GasProvider does not support EIP1559!")
    }
  }

  private fun getEip4844GasFees(): EIP4844GasFees {
    if (contractGasProvider !is EIP4844GasProvider) {
      throw UnsupportedOperationException("GasProvider does not support EIP4844!")
    }
    return contractGasProvider.getEIP4844GasFees()
  }

  @Synchronized
  fun sendTransaction(function: Function, weiValue: BigInteger): EthSendTransaction {
    val encodedData = FunctionEncoder.encode(function)
    val sendRawTransactionResult: EthSendTransaction =
      if (isGasProviderSupportedEIP1559()) {
        val (maxPriorityFeePerGas, maxFeePerGas) = getEip1559GasFees()
        transactionManager.sendEIP1559Transaction(
          (contractGasProvider as ContractEIP1559GasProvider).chainId,
          maxPriorityFeePerGas.toBigInteger(),
          maxFeePerGas.toBigInteger(),
          contractGasProvider.gasLimit,
          contractAddress,
          encodedData,
          weiValue,
          false,
        )
      } else {
        transactionManager.sendTransaction(
          contractGasProvider.gasPrice,
          contractGasProvider.gasLimit,
          contractAddress,
          encodedData,
          weiValue,
          false,
        )
      }

    return sendRawTransactionResult
  }

  private fun createAndSendRawTransaction(
    function: Function,
    weiValue: BigInteger,
    gasPriceCaps: GasPriceCaps? = null,
    gasLimit: BigInteger,
  ): CompletableFuture<EthSendTransaction> {
    val transaction = if (isGasProviderSupportedEIP1559()) {
      val (maxPriorityFeePerGas, maxFeePerGas) = getEip1559GasFees()

      logGasPriceCapsInfo(
        logMessagePrefix = function.name,
        maxPriorityFeePerGas = maxPriorityFeePerGas,
        maxFeePerGas = maxFeePerGas,
        dynamicMaxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap,
        dynamicMaxFeePerGas = gasPriceCaps?.maxFeePerGasCap,
      )

      transactionManager.createRawTransaction(
        chainId = (contractGasProvider as ContractEIP1559GasProvider).chainId,
        maxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger()
          ?: maxPriorityFeePerGas.toBigInteger(),
        maxFeePerGas = gasPriceCaps?.maxFeePerGasCap?.toBigInteger() ?: maxFeePerGas.toBigInteger(),
        gasLimit = gasLimit,
        to = contractAddress,
        value = weiValue,
        data = FunctionEncoder.encode(function),
      )
    } else {
      transactionManager.createRawTransaction(
        gasPrice = contractGasProvider.gasPrice,
        gasLimit = gasLimit,
        to = contractAddress,
        value = weiValue,
        data = FunctionEncoder.encode(function),
      )
    }
    val signedMessage = transactionManager.sign(transaction)
    return web3j
      .ethSendRawTransaction(signedMessage)
      .requestAsync { it }
  }

  @Synchronized
  fun sendTransactionAsync(
    function: Function,
    weiValue: BigInteger,
    gasPriceCaps: GasPriceCaps? = null,
  ): CompletableFuture<EthSendTransaction> {
    return getGasLimit(function)
      .thenCompose { gasLimit ->
        createAndSendRawTransaction(function, weiValue, gasPriceCaps, gasLimit)
      }
  }

  @Synchronized
  fun sendTransactionAfterEthCallAsync(
    function: Function,
    weiValue: BigInteger,
    gasPriceCaps: GasPriceCaps? = null,
  ): CompletableFuture<EthSendTransaction> {
    return getGasLimit(function)
      .thenCompose { gasLimit ->
        executeEthCall(function, gasLimit)
          .thenCompose {
            createAndSendRawTransaction(function, weiValue, gasPriceCaps, gasLimit)
          }
      }
  }

  @Synchronized
  fun sendShnarfDataTransactionAndGetTxHash(function: Function, gasPriceCaps: GasPriceCaps?): SafeFuture<String> {
    return sendTransactionAsync(
      function = function,
      weiValue = BigInteger.ZERO,
      gasPriceCaps = gasPriceCaps,
    )
      .toSafeFuture()
      .thenApply { it.transactionHash }
  }

  fun sendBlobCarryingTransactionAndGetTxHash(
    function: Function,
    blobs: List<ByteArray>,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String> {
    require(blobs.size in 1..9) { "Blobs size=${blobs.size} must be between 1 and 9." }
    return sendBlobCarryingTransaction(function, BigInteger.ZERO, blobs.toWeb3jTxBlob(), gasPriceCaps)
      .thenApply { it.transactionHash }
  }

  @Synchronized
  private fun sendBlobCarryingTransaction(
    function: Function,
    weiValue: BigInteger,
    blobs: List<Blob>,
    gasPriceCaps: GasPriceCaps? = null,
  ): SafeFuture<EthSendTransaction> {
    val blobVersionedHashes = blobs
      .map { BlobUtils.kzgToVersionedHash(BlobUtils.getCommitment(it)).toArray() }
    return getGasLimit(function, blobs, blobVersionedHashes)
      .thenCompose { gasLimit ->
        val eip4844GasProvider = contractGasProvider as EIP4844GasProvider
        val (eip1559fees, maxFeePerBlobGas) = getEip4844GasFees()

        logGasPriceCapsInfo(
          logMessagePrefix = function.name,
          maxPriorityFeePerGas = eip1559fees.maxPriorityFeePerGas,
          maxFeePerGas = eip1559fees.maxFeePerGas,
          maxFeePerBlobGas = maxFeePerBlobGas,
          dynamicMaxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap,
          dynamicMaxFeePerGas = gasPriceCaps?.maxFeePerGasCap,
          dynamicMaxFeePerBlobGas = gasPriceCaps?.maxFeePerBlobGasCap,
        )

        val transaction = transactionManager.createRawTransaction(
          blobs = blobs,
          chainId = eip4844GasProvider.chainId,
          maxPriorityFeePerGas = gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger()
            ?: eip1559fees.maxPriorityFeePerGas.toBigInteger(),
          maxFeePerGas = gasPriceCaps?.maxFeePerGasCap?.toBigInteger() ?: eip1559fees.maxFeePerGas.toBigInteger(),
          gasLimit = gasLimit,
          to = contractAddress,
          data = FunctionEncoder.encode(function),
          value = weiValue,
          maxFeePerBlobGas = gasPriceCaps?.maxFeePerBlobGasCap?.toBigInteger() ?: maxFeePerBlobGas.toBigInteger(),
        )
        val signedMessage = transactionManager.sign(transaction)
        web3j.ethSendRawTransaction(signedMessage)
          .requestAsync { it }
      }
  }

  @Synchronized
  fun executeRemoteCallTransaction(function: Function, weiValue: BigInteger): RemoteFunctionCall<TransactionReceipt> {
    val encodedData = FunctionEncoder.encode(function)
    val transactionSent = sendTransaction(function, weiValue)
    return transactionManager.waitForTransaction(function, encodedData, weiValue, transactionSent)
  }

  @Synchronized
  fun executeRemoteCallTransaction(function: Function): RemoteFunctionCall<TransactionReceipt> {
    return executeRemoteCallTransaction(function, BigInteger.ZERO)
  }

  fun executeEthCall(function: Function, overrideGasLimit: BigInteger? = null): SafeFuture<String?> {
    return (overrideGasLimit?.let { SafeFuture.completedFuture(overrideGasLimit) } ?: getGasLimit(function))
      .thenCompose { gasLimit ->
        Transaction.createFunctionCallTransaction(
          transactionManager.fromAddress,
          null,
          null,
          gasLimit,
          contractAddress,
          FunctionEncoder.encode(function),
        ).let { tx ->
          web3j.informativeEthCall(tx, smartContractErrors)
        }
      }
  }

  fun executeEthCall(
    function: Function,
    gasPriceCaps: GasPriceCaps?,
    overrideGasLimit: BigInteger? = null,
  ): SafeFuture<String?> {
    return (overrideGasLimit?.let { SafeFuture.completedFuture(overrideGasLimit) } ?: getGasLimit(function))
      .thenApply { gasLimit ->
        Transaction(
          /* from */
          transactionManager.fromAddress,
          /* nonce */
          null,
          /* gasPrice */
          null,
          /* gasLimit */
          gasLimit,
          /* to */
          contractAddress,
          /* value */
          null,
          /* data */
          FunctionEncoder.encode(function),
          /* chainId */
          null,
          /* maxPriorityFeePerGasCap */
          gasPriceCaps?.maxPriorityFeePerGasCap?.toBigInteger(),
          /* maxFeePerGasCap */
          gasPriceCaps?.maxFeePerGasCap?.toBigInteger(),
        )
      }.thenCompose { tx ->
        web3j.informativeEthCall(tx, smartContractErrors)
      }
  }

  fun executeBlobEthCall(
    function: Function,
    blobs: List<ByteArray>,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String?> {
    return createEip4844Transaction(
      function,
      blobs.toWeb3jTxBlob(),
      gasPriceCaps,
    ).thenCompose { tx ->
      web3j.informativeEthCall(tx, smartContractErrors)
    }
  }

  private fun logGasPriceCapsInfo(
    logMessagePrefix: String? = "",
    maxPriorityFeePerGas: ULong,
    maxFeePerGas: ULong,
    maxFeePerBlobGas: ULong? = null,
    dynamicMaxPriorityFeePerGas: ULong?,
    dynamicMaxFeePerGas: ULong?,
    dynamicMaxFeePerBlobGas: ULong? = null,
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
        },
    )
  }
}
