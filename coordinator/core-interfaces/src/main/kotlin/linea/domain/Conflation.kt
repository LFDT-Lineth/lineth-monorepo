package linea.domain

import linea.kotlin.isSortedBy
import net.consensys.linea.traces.TracesCounters
import kotlin.time.Instant

data class BlocksConflation(
  val blocks: List<Block>,
  val conflationResult: ConflationCalculationResult,
  val blockRlps: List<ByteArray>,
) : BlockInterval {
  init {
    require(blocks.isSortedBy { it.number }) { "Blocks list must be sorted by blockNumber" }
    require(blockRlps.size == blocks.size) {
      "blockRlps.size (${blockRlps.size}) must equal blocks.size (${blocks.size})"
    }
  }

  override val startBlockNumber: ULong
    get() = blocks.first().number
  override val endBlockNumber: ULong
    get() = blocks.last().number

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as BlocksConflation
    if (blocks != other.blocks) return false
    if (conflationResult != other.conflationResult) return false
    if (blockRlps.size != other.blockRlps.size) return false
    return blockRlps.zip(other.blockRlps).all { (a, b) -> a.contentEquals(b) }
  }

  override fun hashCode(): Int {
    var result = blocks.hashCode()
    result = 31 * result + conflationResult.hashCode()
    result = 31 * result + blockRlps.fold(0) { acc, ba -> 31 * acc + ba.contentHashCode() }
    return result
  }
}

data class Batch(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
) {
  init {
    require(startBlockNumber <= endBlockNumber) {
      "startBlockNumber ($startBlockNumber) must be less than or equal to endBlockNumber ($endBlockNumber)"
    }
  }

  enum class Status {
    Finalized, // Batch is finalized on L1
    Proven, // Batch is ready to be sent to L1 to be finalized
  }

  fun intervalString(): String = CommonDomainFunctions.blockIntervalString(startBlockNumber, endBlockNumber)

  fun toStringSummary(): String {
    return "Batch(startBlockNumber=$startBlockNumber, endBlockNumber=$endBlockNumber)"
  }
}

enum class ConflationTrigger(val triggerPriority: Int) {
  // Business logic needs priority to pick the trigger in case multiple calculators trigger conflation.
  // the lower index of triggerPriority, the higher priority it has.
  // TARGET_BLOCK_NUMBER and FORCED_TRANSACTION need to be the highest priority
  // as it is used as conflation, blob and aggregation boundary.
  TARGET_BLOCK_NUMBER(1),
  FORCED_TRANSACTION(2),
  COINBASE_CHANGE(3),
  HARD_FORK(4),
  DATA_LIMIT(5),
  TRACES_LIMIT(6),
  TIME_LIMIT(7),
  BLOCKS_LIMIT(8),
}

data class ConflationCalculationResult(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val conflationTrigger: ConflationTrigger,
  val tracesCounters: TracesCounters,
) : BlockInterval {
  init {
    require(startBlockNumber <= endBlockNumber) {
      "startBlockNumber ($startBlockNumber) must be less than or equal to endBlockNumber ($endBlockNumber)"
    }
  }
}

data class BlockCounters(
  val blockNumber: ULong,
  val blockTimestamp: Instant,
  val tracesCounters: TracesCounters,
  val blockRLPEncoded: ByteArray,
  val numOfTransactions: UInt = 0u,
  val gasUsed: ULong = 0uL,
  val coinbase: String = "",
) {
  override fun toString(): String {
    return "BlockCounters(blockNumber=$blockNumber, " +
      "blockTimestamp=$blockTimestamp, " +
      "tracesCounters=$tracesCounters, " +
      "blockRLPEncoded=${blockRLPEncoded.size}bytes, " +
      "coinbase=$coinbase)"
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockCounters

    if (blockNumber != other.blockNumber) return false
    if (blockTimestamp != other.blockTimestamp) return false
    if (tracesCounters != other.tracesCounters) return false
    if (!blockRLPEncoded.contentEquals(other.blockRLPEncoded)) return false
    if (numOfTransactions != other.numOfTransactions) return false
    if (gasUsed != other.gasUsed) return false
    if (coinbase != other.coinbase) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blockNumber.hashCode()
    result = 31 * result + blockTimestamp.hashCode()
    result = 31 * result + tracesCounters.hashCode()
    result = 31 * result + blockRLPEncoded.contentHashCode()
    result = 31 * result + numOfTransactions.hashCode()
    result = 31 * result + gasUsed.hashCode()
    result = 31 * result + coinbase.hashCode()
    return result
  }
}
