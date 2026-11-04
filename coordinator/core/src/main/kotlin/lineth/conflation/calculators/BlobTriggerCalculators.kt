package lineth.conflation.calculators

import linea.domain.BlocksConflation
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Instant

/**
 * Flushes the open blob right after a conflation ending at a target block number. Conflations are already cut at
 * these block numbers (see [ConflationTriggerCalculatorByTargetBlockNumbers]), so a target also ends a blob.
 */
class BlobTriggerCalculatorByTargetBlockNumbers(
  private val targetEndBlockNumbers: Set<ULong>,
  private val log: Logger = LogManager.getLogger(BlobTriggerCalculatorByTargetBlockNumbers::class.java),
) : BlobTriggerCalculator {
  override fun appendConflation(conflation: BlocksConflation) = Unit

  override fun checkTrigger(conflation: BlocksConflation): BlobTrigger? {
    if (conflation.endBlockNumber in targetEndBlockNumbers) return BlobTrigger.TARGET_BLOCK_NUMBER
    targetEndBlockNumbers
      .firstOrNull { it >= conflation.startBlockNumber && it < conflation.endBlockNumber }
      ?.let { log.warn("conflation={} overlaps target blob endBlockNumber={}", conflation.intervalString(), it) }
    return null
  }

  override fun reset() = Unit
}

/**
 * Flushes the open blob before the first conflation at or after a hard fork timestamp, so a blob never holds blocks
 * from both sides of a fork. Conflations are already cut at forks (see [ConflationTriggerCalculatorByHardForkTimestamp]).
 *
 * @param initialTimestamp timestamp of the last block already fed to the stream (on restart: the last block of the
 *   resume point). An earlier value makes a fork that is already behind fire once on the first conflation.
 */
class BlobTriggerCalculatorByHardForkTimestamp(
  private val hardForkTimestamps: List<Instant>,
  initialTimestamp: Instant,
  private val log: Logger = LogManager.getLogger(BlobTriggerCalculatorByHardForkTimestamp::class.java),
) : BlobTriggerCalculator {
  private var lastProcessedTimestamp: Instant = initialTimestamp

  init {
    require(hardForkTimestamps.isNotEmpty()) { "hard fork timestamps list cannot be empty" }
    require(hardForkTimestamps.sorted() == hardForkTimestamps) { "hard fork timestamps must be sorted ascending" }
  }

  override fun appendConflation(conflation: BlocksConflation) {
    lastProcessedTimestamp = Instant.fromEpochSeconds(conflation.blocks.last().timestamp.toLong())
  }

  override fun checkTrigger(conflation: BlocksConflation): BlobTrigger? = null

  override fun checkOverflow(next: BlocksConflation): BlobTrigger? {
    val nextTimestamp = Instant.fromEpochSeconds(next.blocks.first().timestamp.toLong())
    val fork = hardForkTimestamps.find { lastProcessedTimestamp < it && nextTimestamp >= it } ?: return null
    log.info("hard fork={} before conflation={}: flushing the open blob", fork, next.intervalString())
    return BlobTrigger.HARD_FORK
  }

  // lastProcessedTimestamp tracks progress across blobs, so it is not reset.
  override fun reset() = Unit
}
