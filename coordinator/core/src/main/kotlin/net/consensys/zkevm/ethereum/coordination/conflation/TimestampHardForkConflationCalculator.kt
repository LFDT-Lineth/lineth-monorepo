package net.consensys.zkevm.ethereum.coordination.conflation

import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

/**
 * Conflation calculator that triggers on timestamp-based hard forks.
 * When a block's timestamp is at or after a hard fork timestamp,
 * it becomes the first block in a new conflation.
 */
class TimestampHardForkConflationCalculator(
  private val hardForkTimestamps: List<Instant>,
  private val initialTimestamp: Instant,
  private val log: Logger = LogManager.getLogger(TimestampHardForkConflationCalculator::class.java),
) : ConflationCalculator {
  override val id: String = "TIMESTAMP_HARD_FORK"

  private var lastProcessedTimestamp: Instant = initialTimestamp

  init {
    require(hardForkTimestamps.isNotEmpty()) {
      "Instantiation of TimestampHardForkConflationCalculator is pointless with empty timestamp list"
    }
    require(hardForkTimestamps.toSet().size == hardForkTimestamps.size) {
      "Timestamps list contains duplicates! Probably a misconfiguration!"
    }
    require(hardForkTimestamps.sorted() == hardForkTimestamps) {
      "Hard fork timestamps must be sorted in ascending order"
    }
  }

  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? {
    val blockTimestamp = blockCounters.blockTimestamp

    // Check if this block crosses any hard fork timestamp boundary
    val applicableTimestamp =
      hardForkTimestamps.find { timestamp ->
        // If the last processed block was before the timestamp and this block is at or after the timestamp
        lastProcessedTimestamp < timestamp && blockTimestamp >= timestamp
      }

    if (applicableTimestamp != null) {
      log.info(
        "Hard fork detected at blockNumber={} with timestamp={}. forkTimestamp={}",
        blockCounters.blockNumber,
        blockTimestamp,
        applicableTimestamp,
      )

      return ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.HARD_FORK,
        singleBlockOverSized = false,
      )
    }

    return null
  }

  override fun appendBlock(blockCounters: BlockCounters) {
    lastProcessedTimestamp = blockCounters.blockTimestamp
  }

  override fun reset() {
    // Note: We don't reset lastProcessedTimestamp as we need to track progress across conflations
  }

  override fun copyCountersTo(counters: ConflationCounters) {
    // No counters to copy - this calculator doesn't track conflation data
  }

  override fun toString(): String {
    return "$id: ${hardForkTimestamps.size} timestamps, initialized at $initialTimestamp, " +
      "timestamps=$hardForkTimestamps"
  }
}
