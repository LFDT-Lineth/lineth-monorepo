package linea.staterecovery.plugin

import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import linea.staterecovery.TransactionFromL1RecoveredData
import linea.staterecovery.TransactionFromL1RecoveredData.AccessTuple
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.crypto.SECPSignature
import org.hyperledger.besu.datatypes.AccessListEntry
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.core.Transaction
import java.math.BigInteger
import java.util.*

fun ByteArray.toBesuAddress(): Address = Address.wrap(Bytes.wrap(this))

object TransactionMapper {
  /**
   * Constructs a Transaction object from the given Transaction recovered from L1 and chainId.
   *
   * @param transaction the parameters for the transaction
   * @param chainId the chain ID for the transaction
   * @return a constructed Transaction object
   */
  fun mapToBesu(
    transaction: TransactionFromL1RecoveredData,
    chainId: ULong
  ): Transaction {
    val builder = Transaction.builder()
    builder
      .sender(Address.fromHexString(transaction.from.encodeHex()))
      .nonce(transaction.nonce.toLong())
      .gasLimit(transaction.gasLimit.toLong())
      .value(Wei.of(transaction.value))
      .payload(Bytes.wrap())
      .chainId(chainId.toBigInteger())
      // compressed transaction don't have signature,
      // so we use a dummy signature which is not verified by Besu in RecoveryMode
      .signature(SECPSignature(BigInteger.ZERO, BigInteger.ZERO, 0.toByte()))
    transaction.data?.let { data -> builder.payload(Bytes.wrap(data)) }
    transaction.to?.let { builder.to(Address.fromHexString(it.encodeHex())) }
    transaction.gasPrice?.let { builder.gasPrice(Wei.of(it)) }
    transaction.maxPriorityFeePerGas?.let { builder.maxPriorityFeePerGas(Wei.of(it)) }
    transaction.maxFeePerGas?.let { builder.maxFeePerGas(Wei.of(it)) }
    transaction.accessList?.let { builder.accessList(mapAccessListEntries(it)) }
    return builder.build()
  }

  private fun mapAccessListEntries(
    accessList: List<AccessTuple>?
  ): List<AccessListEntry>? {
    return accessList
      ?.map { accessTupleParameter ->
        AccessListEntry.createAccessListEntry(
          accessTupleParameter.address.toBesuAddress(),
          accessTupleParameter.storageKeys.map { it.encodeHex() }
        )
      }
  }

  /**
   * Converts a list of TransactionParameters from an ImportBlockFromBlobParameter into a list of
   * Transactions.
   *
   * @param importBlockFromBlobParameter the import block parameter containing transactions
   * @param defaultChainId the default chain ID to use for transactions
   * @return a list of constructed Transaction objects
   */
  fun mapToBesu(
    transactions: List<TransactionFromL1RecoveredData>,
    defaultChainId: ULong
  ): List<Transaction> {
    return transactions.map { tx -> mapToBesu(tx, defaultChainId) }
  }
}
