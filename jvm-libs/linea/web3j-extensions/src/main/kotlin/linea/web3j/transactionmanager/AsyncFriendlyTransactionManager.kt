package linea.web3j.transactionmanager

import linea.domain.BlockParameter
import linea.kotlin.toULong
import linea.web3j.domain.toWeb3j
import linea.web3j.requestAsync
import org.apache.logging.log4j.LogManager
import org.web3j.abi.datatypes.Function
import org.web3j.crypto.Blob
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.protocol.exceptions.TransactionException
import org.web3j.service.TxSignService
import org.web3j.tx.RawTransactionManager
import org.web3j.tx.response.EmptyTransactionReceipt
import org.web3j.tx.response.TransactionReceiptProcessor
import org.web3j.utils.RevertReasonExtractor
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference

class AsyncFriendlyTransactionManager : RawTransactionManager {
  private val log = LogManager.getLogger(this::class.java)
  private var web3j: Web3j

  private val nonce = AtomicReference<BigInteger>()

  constructor(web3j: Web3j, credentials: Credentials, chainId: Long) : super(web3j, credentials, chainId) {
    this.web3j = web3j
    resetNonce().get()
  }

  constructor(
    web3j: Web3j,
    txSignService: TxSignService,
    chainId: Long,
  ) : super(web3j, txSignService, chainId) {
    this.web3j = web3j
    resetNonce().get()
  }

  constructor(web3j: Web3j, credentials: Credentials) : super(web3j, credentials) {
    this.web3j = web3j
    resetNonce().get()
  }

  constructor(
    web3j: Web3j,
    credentials: Credentials,
    transactionReceiptProcessor: TransactionReceiptProcessor,
  ) : super(web3j, credentials, -1, transactionReceiptProcessor) {
    this.web3j = web3j
    resetNonce().get()
  }

  constructor(
    web3j: Web3j,
    credentials: Credentials,
    chainId: Long,
    transactionReceiptProcessor: TransactionReceiptProcessor,
  ) : super(web3j, credentials, chainId, transactionReceiptProcessor) {
    this.web3j = web3j
    resetNonce().get()
  }

  fun resetNonce(blockParameter: BlockParameter = BlockParameter.Tag.LATEST): SafeFuture<ULong> {
    return web3j.ethGetTransactionCount(
      fromAddress,
      blockParameter.toWeb3j(),
    )
      .requestAsync {
        setNonce(it.transactionCount)
        it.transactionCount.toULong()
      }
  }

  fun currentNonce(): BigInteger {
    return nonce.get()
  }

  override fun getNonce(): BigInteger {
    if (nonce.get() == null) {
      throw IllegalStateException("Nonce must be set or reset before any `getNonce` calls")
    }

    val returnedNonce = nonce.getAndUpdate { it.inc() }
    log.trace("account={} nonce={}", fromAddress, returnedNonce)
    return returnedNonce
  }

  private fun setNonce(value: BigInteger) {
    nonce.set(value)
  }

  fun waitForTransaction(
    function: Function,
    encodedData: String,
    weiValue: BigInteger,
    transactionSent: EthSendTransaction,
  ): RemoteFunctionCall<TransactionReceipt> {
    return RemoteFunctionCall(function) {
      val receipt = processResponse(transactionSent)

      if (receipt !is EmptyTransactionReceipt && receipt != null && !receipt.isStatusOK) {
        throw TransactionException(
          String.format(
            "Transaction %s has failed with status: %s. " +
              "Gas used: %s. " +
              "Revert reason: '%s'.",
            receipt.transactionHash,
            receipt.status,
            if (receipt.gasUsedRaw != null) receipt.gasUsed.toString() else "unknown",
            RevertReasonExtractor.extractRevertReason(receipt, encodedData, web3j, true, weiValue),
          ),
          receipt,
        )
      }
      receipt
    }
  }

  fun createRawTransaction(
    blobs: List<Blob>,
    chainId: Long,
    nonce: BigInteger = this.getNonce(),
    maxPriorityFeePerGas: BigInteger,
    maxFeePerGas: BigInteger,
    gasLimit: BigInteger,
    to: String,
    value: BigInteger,
    data: String,
    maxFeePerBlobGas: BigInteger,
  ): RawTransaction {
    return RawTransaction.createTransaction(
      blobs,
      chainId,
      nonce,
      maxPriorityFeePerGas,
      maxFeePerGas,
      gasLimit,
      to,
      value,
      data,
      maxFeePerBlobGas,
    )
  }

  fun createRawTransaction(
    chainId: Long,
    nonce: BigInteger = this.getNonce(),
    maxPriorityFeePerGas: BigInteger,
    maxFeePerGas: BigInteger,
    gasLimit: BigInteger,
    to: String,
    value: BigInteger,
    data: String,
  ): RawTransaction {
    return RawTransaction.createTransaction(
      chainId,
      nonce,
      gasLimit,
      to,
      value,
      data,
      maxPriorityFeePerGas,
      maxFeePerGas,
    )
  }

  fun createRawTransaction(
    nonce: BigInteger = this.getNonce(),
    gasPrice: BigInteger,
    gasLimit: BigInteger,
    to: String,
    value: BigInteger,
    data: String,
  ): RawTransaction {
    return RawTransaction.createTransaction(
      nonce,
      gasPrice,
      gasLimit,
      to,
      value,
      data,
    )
  }
}
