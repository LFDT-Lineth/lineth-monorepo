package net.consensys.linea.contract.l2

import linea.web3j.SmartContractErrors
import linea.web3j.informativeEthCall
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.EIP1559GasProvider
import net.consensys.linea.contract.L2MessageService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.Type
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.exceptions.ContractCallException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class L2MessageServiceGasLimitEstimate(
  contractAddress: String,
  web3j: Web3j,
  private val l2TransactionManager: AsyncFriendlyTransactionManager,
  private val eip1559GasProvider: EIP1559GasProvider,
  private val smartContractErrors: SmartContractErrors
) : L2MessageService(
  contractAddress,
  web3j,
  l2TransactionManager,
  eip1559GasProvider
) {

  private val log: Logger = LogManager.getLogger(this::class.java)
  private val maxGasLimit: BigInteger = BigInteger.valueOf(30_000_000)

  companion object {
    fun load(
      contractAddress: String,
      web3j: Web3j,
      transactionManager: AsyncFriendlyTransactionManager,
      contractGasProvider: EIP1559GasProvider,
      smartContractErrors: SmartContractErrors
    ): L2MessageServiceGasLimitEstimate {
      return L2MessageServiceGasLimitEstimate(
        contractAddress,
        web3j,
        transactionManager,
        contractGasProvider,
        smartContractErrors
      )
    }
  }

  override fun anchorL1L2MessageHashes(
    _messageHashes: List<ByteArray>,
    _startingMessageNumber: BigInteger,
    _finalMessageNumber: BigInteger,
    _finalRollingHash: ByteArray
  ): RemoteFunctionCall<TransactionReceipt> {
    return estimateMessageAnchoringGasLimit(
      _messageHashes,
      _startingMessageNumber,
      _finalMessageNumber,
      _finalRollingHash
    )
      .thenApply { gasLimit ->
        eip1559GasProvider.overrideNextGasLimit(gasLimit)
        super.anchorL1L2MessageHashes(_messageHashes, _startingMessageNumber, _finalMessageNumber, _finalRollingHash)
      }
      .get()
  }

  private fun estimateMessageAnchoringGasLimit(
    messageHashes: List<ByteArray>,
    startingMessageNumber: BigInteger,
    finalMessageNumber: BigInteger,
    finalRollingHash: ByteArray
  ): SafeFuture<BigInteger> {
    val tx = createAnchorL1L2MessageHashesTransaction(
      messageHashes,
      startingMessageNumber,
      finalMessageNumber,
      finalRollingHash
    )
    val estimatedGasLimit = web3j.ethEstimateGas(tx).sendAsync()

    return SafeFuture.of(estimatedGasLimit)
      .thenApply { ethEstimateGas ->
        log.debug("Estimated gas limit: {}", ethEstimateGas.amountUsed)
        ethEstimateGas.amountUsed
      }
      .exceptionallyCompose { error ->
        handleError(
          messageHashes,
          startingMessageNumber,
          finalMessageNumber,
          finalRollingHash,
          error
        )
      }
  }

  private fun createAnchorL1L2MessageHashesTransaction(
    messageHashes: List<ByteArray>,
    startingMessageNumber: BigInteger,
    finalMessageNumber: BigInteger,
    finalRollingHash: ByteArray
  ): Transaction {
    val function = Function(
      FUNC_ANCHORL1L2MESSAGEHASHES,
      listOf<Type<*>>(
        DynamicArray(
          Bytes32::class.java,
          messageHashes.map { Bytes32(it) }
        ),
        Uint256(startingMessageNumber),
        Uint256(finalMessageNumber),
        Bytes32(finalRollingHash)
      ),
      emptyList<TypeReference<*>>()
    )
    val calldata = FunctionEncoder.encode(function)
    val tx = Transaction(
      l2TransactionManager.fromAddress,
      l2TransactionManager.currentNonce(),
      null,
      maxGasLimit,
      this.contractAddress,
      BigInteger.ZERO,
      calldata
    )
    return tx
  }

  private fun anchorMessagesCall(
    messageHashes: List<ByteArray>,
    startingMessageNumber: BigInteger,
    finalMessageNumber: BigInteger,
    finalRollingHash: ByteArray
  ): SafeFuture<String?> {
    val tx = createAnchorL1L2MessageHashesTransaction(
      messageHashes,
      startingMessageNumber,
      finalMessageNumber,
      finalRollingHash
    )

    return web3j.informativeEthCall(tx, smartContractErrors)
  }

  private fun handleError(
    messageHashes: List<ByteArray>,
    startingMessageNumber: BigInteger,
    finalMessageNumber: BigInteger,
    finalRollingHash: ByteArray,
    error: Throwable
  ): SafeFuture<BigInteger> {
    return anchorMessagesCall(messageHashes, startingMessageNumber, finalMessageNumber, finalRollingHash)
      .handleException { errorReason ->
        log.debug(
          "Eth Estimate Gas failed: Number of message hashes: {} error: {}",
          messageHashes.size,
          errorReason.message
        )
        val message =
          "Gas limit estimation failed - Eth call error: ${errorReason.localizedMessage} Error: ${error.message}"
        throw ContractCallException(message, errorReason)
      }
      .thenApply { null }
  }
}
