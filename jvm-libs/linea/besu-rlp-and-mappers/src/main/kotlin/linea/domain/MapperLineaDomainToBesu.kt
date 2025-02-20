package linea.domain

import linea.domain.MapperLineaDomainToBesu.mapToBesu
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.crypto.SECP256K1
import org.hyperledger.besu.crypto.SECPSignature
import org.hyperledger.besu.datatypes.AccessListEntry
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.core.BlockBody
import org.hyperledger.besu.ethereum.core.BlockHeaderBuilder
import org.hyperledger.besu.ethereum.core.Difficulty
import org.hyperledger.besu.ethereum.core.Transaction
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions
import org.hyperledger.besu.evm.log.LogsBloomFilter
import java.math.BigInteger

fun Block.toBesu(): org.hyperledger.besu.ethereum.core.Block = mapToBesu(this)
fun linea.domain.Transaction.toBesu(): Transaction = mapToBesu(this)

object MapperLineaDomainToBesu {
  private val secp256k1 = SECP256K1()
  private val blockHeaderFunctions = MainnetBlockHeaderFunctions()

  internal fun signature(tx: linea.domain.Transaction, recId: Byte): SECPSignature {
    return secp256k1.createSignature(
      tx.r,
      tx.s,
      recId
    )
  }

  fun recIdFromV(v: BigInteger): Pair<Byte, BigInteger?> {
    val recId: Byte
    var chainId: BigInteger? = null
    if (v == Transaction.REPLAY_UNPROTECTED_V_BASE || v == Transaction.REPLAY_UNPROTECTED_V_BASE_PLUS_1) {
      recId = v.subtract(Transaction.REPLAY_UNPROTECTED_V_BASE).byteValueExact()
    } else if (v > Transaction.REPLAY_PROTECTED_V_MIN) {
      chainId = v.subtract(Transaction.REPLAY_PROTECTED_V_BASE).divide(Transaction.TWO)
      recId = v.subtract(Transaction.TWO.multiply(chainId).add(Transaction.REPLAY_PROTECTED_V_BASE)).byteValueExact()
    } else {
      throw RuntimeException("An unsupported encoded `v` value of $v was found")
    }
    return Pair(recId, chainId)
  }

  fun getRecIdAndChainId(tx: linea.domain.Transaction): Pair<Byte, BigInteger?> {
    if (tx.type == TransactionType.FRONTIER) {
      return recIdFromV(tx.v!!.toBigInteger())
    } else {
      return (tx.yParity ?: tx.v)!!.toByte() to tx.chainId?.toBigInteger()
    }
  }

  fun mapToBesu(block: Block): org.hyperledger.besu.ethereum.core.Block {
    runCatching {
      val header = BlockHeaderBuilder.create()
        .parentHash(Hash.wrap(Bytes32.wrap(block.parentHash)))
        .ommersHash(Hash.wrap(Bytes32.wrap(block.ommersHash)))
        .coinbase(Address.wrap(Bytes.wrap(block.miner)))
        .stateRoot(Hash.wrap(Bytes32.wrap(block.stateRoot)))
        .transactionsRoot(Hash.wrap(Bytes32.wrap(block.transactionsRoot)))
        .receiptsRoot(Hash.wrap(Bytes32.wrap(block.receiptsRoot)))
        .logsBloom(LogsBloomFilter.fromHexString(block.logsBloom.encodeHex()))
        .difficulty(Difficulty.fromHexOrDecimalString(block.difficulty.toString()))
        .number(block.number.toLong())
        .gasLimit(block.gasLimit.toLong())
        .gasUsed(block.gasUsed.toLong())
        .timestamp(block.timestamp.toLong())
        .extraData(Bytes.wrap(block.extraData))
        .mixHash(Hash.wrap(Bytes32.wrap(block.mixHash)))
        .nonce(block.nonce.toLong())
        .baseFee(block.baseFeePerGas?.toWei())
        .blockHeaderFunctions(blockHeaderFunctions)
        .buildBlockHeader()

      val transactions =
        block.transactions.mapIndexed { index, transaction ->
          mapToBesu(block.number, index, transaction)
        }
      // linea does not support uncles, so we are not converting them
      // throwing an exception just in case we get one and we can fix it
      if (block.ommers.isNotEmpty()) {
        throw IllegalStateException("Uncles are not supported: block=${block.number}")
      }

      val body = BlockBody(transactions, emptyList())

      return org.hyperledger.besu.ethereum.core.Block(header, body)
    }.getOrElse { th ->
      if (th.message?.startsWith("Error mapping transaction to Besu") ?: false) {
        throw th
      } else {
        throw RuntimeException("Error mapping block=${block.number} to Besu: ${th.message}", th)
      }
    }
  }

  fun mapToBesu(blockNumber: ULong, txIndex: Int, tx: linea.domain.Transaction): Transaction {
    return runCatching { mapToBesu(tx) }
      .getOrElse { th ->
        throw RuntimeException(
          "Error mapping transaction to Besu: block=$blockNumber txIndex=$txIndex transaction=$tx",
          th
        )
      }
  }

  fun mapToBesu(tx: linea.domain.Transaction): Transaction {
    val (recId, recChainId) = getRecIdAndChainId(tx)
    val signature = signature(tx, recId)

    val besuType = tx.type.toBesu()
    val chainId = tx.chainId?.toBigInteger() ?: recChainId
    return Transaction.builder()
      .type(besuType)
      .nonce(tx.nonce.toLong())
      .apply { tx.gasPrice?.let { gasPrice(it.toWei()) } }
      .gasLimit(tx.gasLimit.toLong())
      .to(tx.to?.let { Address.wrap(Bytes.wrap(it)) })
      .value(tx.value.toWei())
      .payload(Bytes.wrap(tx.input))
      .apply { chainId?.let { chainId(it) } }
      .maxPriorityFeePerGas(tx.maxPriorityFeePerGas?.toWei())
      .maxFeePerGas(tx.maxFeePerGas?.toWei())
      .apply {
        if (besuType.supportsAccessList()) {
          val accList = tx.accessList?.map { entry ->
            AccessListEntry(
              Address.wrap(Bytes.wrap(entry.address)),
              entry.storageKeys.map { Bytes32.wrap(it) }
            )
          } ?: emptyList()
          accessList(accList)
        }
      }
      .signature(signature)
      .build()
  }

  fun ULong.toWei(): Wei = Wei.of(this.toBigInteger())
  fun BigInteger.toWei(): Wei = Wei.of(this)
}
