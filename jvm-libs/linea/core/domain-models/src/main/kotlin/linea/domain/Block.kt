package linea.domain

import linea.kotlin.encodeHex
import kotlin.time.Instant

typealias Block = BlockData<Transaction>
typealias BlockWithTxHashes = BlockData<ByteArray>

data class BlockData<TxData>(
  val number: ULong,
  val hash: ByteArray,
  val parentHash: ByteArray,
  val ommersHash: ByteArray,
  val miner: ByteArray,
  val stateRoot: ByteArray,
  val transactionsRoot: ByteArray,
  val receiptsRoot: ByteArray,
  val logsBloom: ByteArray,
  val difficulty: ULong,
  val gasLimit: ULong,
  val gasUsed: ULong,
  val timestamp: ULong,
  val extraData: ByteArray,
  val mixHash: ByteArray,
  val nonce: ULong,
  val baseFeePerGas: ULong? = null, // Optional field for EIP-1559 blocks
  val transactions: List<TxData> = emptyList(), // List of transaction hashes
  val ommers: List<ByteArray> = emptyList(), // List of uncle block hashes
  val size: ULong,
) {
  companion object {
    // companion object to allow static extension functions
  }

  init {
    if (transactions.isNotEmpty()) {
      require(transactions.first() is Transaction || transactions.first() is ByteArray) {
        "Invalid transaction type ${transactions.first()!!::class.java}. Supported types Transaction or ByteArray."
      }
    }
  }

  private val isTransactionHashOnly: Boolean
    get() = transactions.isNotEmpty() && transactions.first() is ByteArray
  val headerSummary = BlockHeaderSummary(this.number, this.hash, Instant.fromEpochSeconds(this.timestamp.toLong()))

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockData<*>

    if (number != other.number) return false
    if (!hash.contentEquals(other.hash)) return false
    if (!parentHash.contentEquals(other.parentHash)) return false
    if (!ommersHash.contentEquals(other.ommersHash)) return false
    if (!miner.contentEquals(other.miner)) return false
    if (!stateRoot.contentEquals(other.stateRoot)) return false
    if (!transactionsRoot.contentEquals(other.transactionsRoot)) return false
    if (!receiptsRoot.contentEquals(other.receiptsRoot)) return false
    if (!logsBloom.contentEquals(other.logsBloom)) return false
    if (difficulty != other.difficulty) return false
    if (gasLimit != other.gasLimit) return false
    if (gasUsed != other.gasUsed) return false
    if (timestamp != other.timestamp) return false
    if (!extraData.contentEquals(other.extraData)) return false
    if (!mixHash.contentEquals(other.mixHash)) return false
    if (nonce != other.nonce) return false
    if (baseFeePerGas != other.baseFeePerGas) return false
    if (!transactions.zip(other.transactions).all { (thisTx, otherTx) ->
        when {
          thisTx is ByteArray && otherTx is ByteArray -> thisTx.contentEquals(otherTx)
          thisTx is Transaction && otherTx is Transaction -> thisTx == otherTx
          else -> false
        }
      }
    ) {
      return false
    }
    if (ommers != other.ommers) return false
    if (headerSummary != other.headerSummary) return false
    if (size != other.size) return false

    return true
  }

  override fun hashCode(): Int {
    var result = number.hashCode()
    result = 31 * result + hash.contentHashCode()
    result = 31 * result + parentHash.contentHashCode()
    result = 31 * result + ommersHash.contentHashCode()
    result = 31 * result + miner.contentHashCode()
    result = 31 * result + stateRoot.contentHashCode()
    result = 31 * result + transactionsRoot.contentHashCode()
    result = 31 * result + receiptsRoot.contentHashCode()
    result = 31 * result + logsBloom.contentHashCode()
    result = 31 * result + difficulty.hashCode()
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + gasUsed.hashCode()
    result = 31 * result + timestamp.hashCode()
    result = 31 * result + extraData.contentHashCode()
    result = 31 * result + mixHash.contentHashCode()
    result = 31 * result + nonce.hashCode()
    result = 31 * result + (baseFeePerGas?.hashCode() ?: 0)
    result = 31 * result + transactions.fold(0) { acc, tx ->
      acc * 31 + when (tx) {
        is ByteArray -> tx.contentHashCode()
        is Transaction -> tx.hashCode()
        else -> 0
      }
    }
    result = 31 * result + ommers.hashCode()
    result = 31 * result + headerSummary.hashCode()
    result = 31 * result + size.hashCode()
    return result
  }

  override fun toString(): String {
    val txStr = if (isTransactionHashOnly) {
      transactions.joinToString(prefix = "[", postfix = "]") { (it as ByteArray).encodeHex() }
    } else {
      transactions.joinToString(prefix = "[", postfix = "]") { it.toString() }
    }
    return "Block(" +
      "number=$number, " +
      "hash=${hash.encodeHex()}, " +
      "parentHash=${parentHash.encodeHex()}, " +
      "ommersHash=${ommersHash.encodeHex()}, " +
      "miner=${miner.encodeHex()}, " +
      "stateRoot=${stateRoot.encodeHex()}, " +
      "transactionsRoot=${transactionsRoot.encodeHex()}, " +
      "receiptsRoot=${receiptsRoot.encodeHex()}, " +
      "logsBloom=${logsBloom.encodeHex()}, " +
      "difficulty=$difficulty, " +
      "gasLimit=$gasLimit, " +
      "gasUsed=$gasUsed, " +
      "timestamp=$timestamp, " +
      "extraData=${extraData.encodeHex()}, " +
      "mixHash=${mixHash.encodeHex()}, " +
      "nonce=$nonce, " +
      "baseFeePerGas=$baseFeePerGas, " +
      "transactions=$txStr, " +
      "ommers=$ommers, " +
      "size=$size" + ")"
  }
}

data class BlockHeaderSummary(
  val number: ULong,
  val hash: ByteArray,
  val timestamp: Instant,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockHeaderSummary

    if (number != other.number) return false
    if (!hash.contentEquals(other.hash)) return false
    if (timestamp != other.timestamp) return false

    return true
  }

  override fun hashCode(): Int {
    var result = number.hashCode()
    result = 31 * result + hash.contentHashCode()
    result = 31 * result + timestamp.hashCode()
    return result
  }

  override fun toString(): String {
    return "BlockHeaderSummary(" +
      "number=$number, " +
      "hash=${hash.contentToString()}, " +
      "timestamp=$timestamp)"
  }
}
