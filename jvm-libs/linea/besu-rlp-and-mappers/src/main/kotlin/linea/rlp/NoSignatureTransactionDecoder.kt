package linea.rlp

import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.crypto.SECPSignature
import org.hyperledger.besu.datatypes.AccessListEntry
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.TransactionType
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.core.Transaction
import org.hyperledger.besu.ethereum.core.encoding.CodeDelegationTransactionDecoder
import org.hyperledger.besu.ethereum.rlp.RLP
import org.hyperledger.besu.ethereum.rlp.RLPInput
import java.math.BigInteger

class NoSignatureTransactionDecoder {
  fun decode(input: RLPInput): Transaction {
    if (!input.nextIsList()) {
      val typedTransactionBytes = input.readBytes()
      val transactionInput = RLP.input(typedTransactionBytes.slice(1))
      val transactionType = typedTransactionBytes[0]
      if (transactionType.toInt() == 0x01) {
        return decodeAccessList(transactionInput)
      }
      if (transactionType.toInt() == 0x02) {
        return decode1559CompatibleEnvelope(transactionInput, TransactionType.EIP1559)
      }
      if (transactionType.toInt() == 0x04) {
        return decode1559CompatibleEnvelope(transactionInput, TransactionType.DELEGATE_CODE)
      }
      throw IllegalArgumentException("Unsupported transaction type")
    } else { // Frontier transaction
      return decodeFrontier(input)
    }
  }

  private fun decodeAccessList(transactionInput: RLPInput): Transaction {
    val builder = Transaction.builder()

    transactionInput.enterList()
    builder
      .type(TransactionType.ACCESS_LIST)
      .chainId(BigInteger.valueOf(transactionInput.readLongScalar()))
      .nonce(transactionInput.readLongScalar())
      .gasPrice(Wei.of(transactionInput.readUInt256Scalar()))
      .gasLimit(transactionInput.readLongScalar())
      .to(
        transactionInput
          .readBytes { addressBytes: Bytes ->
            if (addressBytes.isEmpty) null else Address.wrap(addressBytes)
          },
      )
      .value(Wei.of(transactionInput.readUInt256Scalar()))
      .payload(transactionInput.readBytes())
      .accessList(
        transactionInput.readList { accessListEntryRLPInput: RLPInput ->
          accessListEntryRLPInput.enterList()
          val accessListEntry =
            AccessListEntry(
              Address.wrap(accessListEntryRLPInput.readBytes()),
              accessListEntryRLPInput.readList { obj: RLPInput -> obj.readBytes32() },
            )
          accessListEntryRLPInput.leaveList()
          accessListEntry
        },
      )
    transactionInput.readUnsignedByteScalar()
    builder.sender(Address.extract(transactionInput.readUInt256Scalar()))
    transactionInput.readUInt256Scalar()
    transactionInput.leaveList()
    return builder.signature(SECPSignature(BigInteger.ZERO, BigInteger.ZERO, 0.toByte())).build()
  }

  private fun decode1559CompatibleEnvelope(transactionInput: RLPInput, txType: TransactionType): Transaction {
    val builder = Transaction.builder()
    transactionInput.enterList()
    val chainId = transactionInput.readBigIntegerScalar()
    builder
      .type(txType)
      .chainId(chainId)
      .nonce(transactionInput.readLongScalar())
      .maxPriorityFeePerGas(Wei.of(transactionInput.readUInt256Scalar()))
      .maxFeePerGas(Wei.of(transactionInput.readUInt256Scalar()))
      .gasLimit(transactionInput.readLongScalar())
      .to(
        transactionInput.readBytes { v: Bytes ->
          if (v.isEmpty) {
            null
          } else {
            Address.wrap(
              v,
            )
          }
        },
      )
      .value(Wei.of(transactionInput.readUInt256Scalar()))
      .payload(transactionInput.readBytes())
      .accessList(
        transactionInput.readList { accessListEntryRLPInput: RLPInput ->
          accessListEntryRLPInput.enterList()
          val accessListEntry =
            AccessListEntry(
              Address.wrap(accessListEntryRLPInput.readBytes()),
              accessListEntryRLPInput.readList { obj: RLPInput -> obj.readBytes32() },
            )
          accessListEntryRLPInput.leaveList()
          accessListEntry
        },
      )
      .apply {
        if (txType == TransactionType.DELEGATE_CODE) {
          codeDelegations(transactionInput.readList(CodeDelegationTransactionDecoder::decodeInnerPayload))
        }
      }
    transactionInput.readUnsignedByteScalar()
    builder.sender(Address.extract(transactionInput.readUInt256Scalar()))
    transactionInput.readUInt256Scalar()
    transactionInput.leaveList()
    return builder.signature(SECPSignature(BigInteger.ZERO, BigInteger.ZERO, 0.toByte())).build()
  }

  private fun decodeFrontier(input: RLPInput): Transaction {
    val builder = Transaction.builder()
    input.enterList()
    builder
      .type(TransactionType.FRONTIER)
      .nonce(input.readLongScalar())
      .gasPrice(Wei.of(input.readUInt256Scalar()))
      .gasLimit(input.readLongScalar())
      .to(
        input.readBytes { v: Bytes ->
          if (v.isEmpty) {
            null
          } else {
            Address.wrap(
              v,
            )
          }
        },
      )
      .value(Wei.of(input.readUInt256Scalar()))
      .payload(input.readBytes())

    input.readBigIntegerScalar()
    builder.sender(Address.extract(input.readUInt256Scalar()))
    input.readUInt256Scalar()
    val signature = SECPSignature(BigInteger.ZERO, BigInteger.ZERO, 0.toByte())
    input.leaveList()
    return builder.signature(signature).build()
  }
}
