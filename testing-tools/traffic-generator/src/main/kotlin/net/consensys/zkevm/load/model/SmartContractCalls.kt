package net.consensys.zkevm.load.model

import net.consensys.zkevm.load.model.inner.ArrayParameter
import net.consensys.zkevm.load.model.inner.CreateContract
import net.consensys.zkevm.load.model.inner.Parameter
import net.consensys.zkevm.load.model.inner.SimpleParameter
import org.slf4j.LoggerFactory
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.datatypes.Address
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.Type
import org.web3j.abi.datatypes.Utf8String
import org.web3j.abi.datatypes.generated.Uint256
import org.web3j.crypto.RawTransaction
import org.web3j.protocol.core.methods.request.Transaction
import java.io.IOException
import java.math.BigInteger
import java.util.*

class SmartContractCalls(private val ethConnection: EthConnection) {
  private val logger = LoggerFactory.getLogger(SmartContractCalls::class.java)

  fun genericCall(methodName: String, inputParameters: List<Parameter>): String {
    val inputCallParameters = types(inputParameters)
    val function = Function(methodName, inputCallParameters, emptyList())

    return FunctionEncoder.encode(function)
  }

  private fun types(inputParameters: List<Parameter>): List<Type<*>> {
    return inputParameters.map { p -> mapParameter(p) }.toList()
  }

  private fun mapParameter(p: Parameter): Type<*> {
    return when (p) {
      is ArrayParameter -> {
        @Suppress("DEPRECATION")
        DynamicArray(p.values.map { item -> mapParameter(item) }.toList())
      }

      is SimpleParameter -> {
        when (p.solidityType) {
          "Address" -> Address(if (p.value.startsWith("0x")) p.value else "0x${p.value}")
          "Uint256" -> Uint256(p.value.toLong().let { BigInteger.valueOf(it) })
          "String" -> Utf8String(p.value)
          else -> {
            throw RuntimeException("Unsupported type for parameter $p")
          }
        }
      }

      else -> {
        throw RuntimeException("Unsupported type for parameter $p")
      }
    }
  }

  fun transferOwnership(address: String): String {
    val hexPrefixedAddress = if (address.startsWith("0x")) address else "0x$address"
    val function = Function("transferOwnership", listOf<Type<*>>(Address(hexPrefixedAddress)), emptyList())
    return FunctionEncoder.encode(function)
  }

  fun mint(addressS: String?, inputAmount: Long): String {
    val address = Address(addressS)
    val amount = Uint256(BigInteger.valueOf(inputAmount))
    val function = Function("mint", listOf<Type<*>>(address, amount), emptyList())
    return FunctionEncoder.encode(function)
  }

  fun batchMint(addressesInput: List<String?>, inputAmount: Long): String {
    @Suppress("DEPRECATION")
    val addresses = DynamicArray(
      addressesInput.stream().map { address: String? -> Address(address) }.toList(),
    )
    val amount = Uint256(BigInteger.valueOf(inputAmount))
    val function = Function("batchMint", listOf(addresses, amount), emptyList())
    return FunctionEncoder.encode(function)
  }

  @Throws(IOException::class, InterruptedException::class)
  fun createContract(
    sourceWallet: Wallet,
    contractCode: CreateContract,
    chainId: Int,
    details: ExecutionDetails,
  ): String {
    val contractCreationTx =
      getCreateContractTransaction(sourceWallet, contractCode.byteCode, chainId)
    details.addContractDeployment(contractCreationTx)
    val receipt = contractCreationTx.ethSendTransactionRequest.send()
    logger.debug("[DEBUG] contract creation tx hash {}", receipt.transactionHash)
    // get contract address
    var transactionReceipt = ethConnection.ethGetTransactionReceipt(receipt.transactionHash).send()
    // maybe transaction receipt is not directly present, we may have to retry.
    var attempt = 0
    while ((
        !transactionReceipt.transactionReceipt.isPresent ||
          transactionReceipt.transactionReceipt.get().contractAddress == null
        ) &&
      attempt < 3
    ) {
      transactionReceipt = ethConnection.ethGetTransactionReceipt(receipt.transactionHash).send()
      attempt += 1
      Thread.sleep(2000)
      logger.info("Waiting for transaction receipt to get new contract address.")
    }
    return if (transactionReceipt.transactionReceipt.isPresent) {
      transactionReceipt.transactionReceipt.get().contractAddress
    } else {
      throw RuntimeException("Failed to create smart contract.")
    }
  }

  @Throws(IOException::class)
  fun getCreateContractTransaction(sourceWallet: Wallet, contractCode: String?, chainId: Int): TransactionDetail {
    val nonce = sourceWallet.theoreticalNonceValue
    sourceWallet.incrementTheoreticalNonce()
    val transactionForEstimation = Transaction(
      /* from = */
      sourceWallet.address,
      /* nonce = */
      nonce,
      /* gasPrice = */
      null,
      /* gasLimit = */
      null,
      /* to = */
      null,
      /* value = */
      null,
      /* data = */
      contractCode,
    )

    val (gasPrice, gasLimit) = ethConnection.estimateGasPriceAndLimit(transactionForEstimation)

    val rawTransaction = RawTransaction.createContractTransaction(
      /* nonce = */
      nonce,
      /* gasPrice = */
      gasPrice,
      /* gasLimit = */
      gasLimit,
      /* value = */
      BigInteger.ZERO,
      /* init = */
      contractCode,
    )
    val contractCreationTx = ethConnection.ethSendRawTransaction(rawTransaction, sourceWallet, chainId)
    return TransactionDetail(sourceWallet.id, nonce, contractCreationTx)
  }

  @Throws(IOException::class)
  fun getRequests(
    contractAddress: String,
    sourceWallet: Wallet,
    encodedFunction: String,
    txCount: Int,
    chainId: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    val txs: MutableList<TransactionDetail> = ArrayList()

    for (i in 0 until txCount) {
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
        contractAddress,
        /* value = */
        null,
        /* data = */
        encodedFunction,
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
        contractAddress,
        /* data = */
        encodedFunction,
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
    return mapOf<Wallet, List<TransactionDetail>>(sourceWallet to txs)
  }
}
