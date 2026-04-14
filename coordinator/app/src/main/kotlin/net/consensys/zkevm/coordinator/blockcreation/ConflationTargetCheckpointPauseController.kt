package net.consensys.zkevm.coordinator.blockcreation

import linea.domain.Block
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Instant

/**
 * Optional gate used by [BlockCreationMonitor] to pause L2 block import / conflation at configured **target
 * checkpoints** (block numbers or timestamp thresholds) until L1 finalization and/or an operator API signal allows
 * resuming.
 *
 * The monitor is expected to call [shouldPauseConflation] at the start of each poll tick. Implementations may clear
 * internal pause state there when release conditions are met. [importBlock] is called after a block has been
 * successfully handed to conflation for that tick.
 *
 * @see ConflationTargetCheckpointPauseController
 */
interface TargetCheckpointPauseController {
  /**
   * Evaluates whether this poll tick must stay idle because a target checkpoint pause is still active, and may clear
   * that pause when configured gates (e.g. L1-finalized L2 height, API acknowledgement) are satisfied.
   *
   * Intended to be invoked once per [BlockCreationMonitor] poll tick before fetching the next block.
   *
   * @return `true` if the monitor must skip work for this tick (conflation stays paused at a checkpoint);
   *   `false` if the monitor may proceed (no pause, or pause was released this call).
   */
  fun shouldPauseConflation(): Boolean

  /**
   * Notifies the controller that [block] was successfully delivered to conflation for the current progression.
   * Implementations typically update last-seen timestamps and, when a checkpoint is crossed, record what must be true
   * on L1 (and optionally via [signalResumeFromApi]) before [shouldPauseConflation] stops returning `true`.
   *
   * @param block the L2 block that was just imported into conflation
   */
  fun importBlock(block: Block)

  /**
   * Operator-facing resume signal (e.g. JSON-RPC). Implementations should acknowledge only when an API-gated checkpoint
   * pause is actually active.
   *
   * @return `true` if the signal was applied (e.g. API gate enabled and a checkpoint pause is waiting on API ack);
   *   `false` if the signal was ignored.
   */
  fun signalResumeFromApi(): Boolean
}

/**
 * Coordinates pausing [BlockCreationMonitor] at configured target end blocks and timestamp thresholds when
 * [Config.waitTargetBlockL1Finalization] and/or [Config.waitApiResumeAfterTargetBlock] are enabled.
 *
 * L1 finalization height is read from [latestL1FinalizedBlockProvider] (same source as
 * [BatchesRepoBasedLastProvenBlockNumberProvider]). Pause release is driven from [BlockCreationMonitor] via
 * [shouldPauseConflation].
 */
class ConflationTargetCheckpointPauseController(
  private val config: Config,
  private val latestL1FinalizedBlockProvider: LatestL1FinalizedBlockProviderSync,
  private val log: Logger = LogManager.getLogger(ConflationTargetCheckpointPauseController::class.java),
) : TargetCheckpointPauseController {
  data class Config(
    val initialLastImportedBlockTimestamp: Instant,
    val targetEndBlocks: Set<ULong>,
    val targetTimestamps: List<Instant>,
    val waitTargetBlockL1Finalization: Boolean,
    val waitApiResumeAfterTargetBlock: Boolean,
  )

  init {
    val ts = config.targetTimestamps
    if (ts.isNotEmpty()) {
      require(ts.toSet().size == ts.size) {
        "Target timestamps list contains duplicates (misconfiguration)"
      }
      require(ts.sorted() == ts) {
        "Target timestamps must be sorted in ascending order"
      }
    }
  }

  private val apiAcknowledged = AtomicBoolean(false)

  /**
   * Minimum L2 block number finalized on L1 required before conflation may resume; when non-null, a target checkpoint
   * was hit and [BlockCreationMonitor] must idle until [shouldPauseConflation] returns false once L1 finalization
   * and/or API gates (see [Config]) allow release.
   */
  private val requiredL1FinalizedL2BlockToResumeConflation = AtomicReference<ULong?>(null)

  /**
   * O(1) lookup: a block-number checkpoint is the configured `targetBlockNumber` whose completion is observed when
   * conflation imports block `targetBlockNumber + 1`.
   */
  private val targetEndBlockNumbers: Set<ULong> = config.targetEndBlocks

  /**
   * Last processed block timestamp; crossing detection matches
   * [net.consensys.zkevm.ethereum.coordination.conflation.TimestampHardForkConflationCalculator.checkOverflow].
   */
  private val lastImportedBlockTimestamp = AtomicReference(config.initialLastImportedBlockTimestamp)

  val pauseFeatureEnabled: Boolean
    get() =
      config.waitTargetBlockL1Finalization ||
        config.waitApiResumeAfterTargetBlock

  /**
   * @return true if the API resume signal was applied: API gate is enabled, and a checkpoint pause
   *   is active (required L1-finalized L2 block to resume is set). Gates are re-evaluated on the next poll tick via
   *   [shouldPauseConflation].
   */
  override fun signalResumeFromApi(): Boolean {
    if (!config.waitApiResumeAfterTargetBlock) {
      return false
    }
    if (requiredL1FinalizedL2BlockToResumeConflation.get() == null) {
      return false
    }
    apiAcknowledged.set(true)
    return true
  }

  /**
   * Called after a block has been successfully delivered to conflation (listener completed).
   * May record a checkpoint pause; [BlockCreationMonitor] drives release through [shouldPauseConflation].
   */
  override fun importBlock(block: Block) {
    if (!pauseFeatureEnabled) {
      return
    }
    if (requiredL1FinalizedL2BlockToResumeConflation.get() != null) {
      return
    }

    val currentBlockNumber = block.number
    val currentBlockTimestamp = Instant.fromEpochSeconds(block.timestamp.toLong())
    val prevBlockTimestamp = lastImportedBlockTimestamp.get()

    var blockHit = false
    var tsHit = false
    var requiredL1 = 0UL

    if (currentBlockNumber >= 1U) {
      val targetBlockNumber = currentBlockNumber - 1U
      if (targetBlockNumber in targetEndBlockNumbers) {
        blockHit = true
        requiredL1 = maxOf(requiredL1, targetBlockNumber)
      }
    }

    // Same predicate as TimestampHardForkConflationCalculator.checkOverflow (first matching threshold in list order).
    val crossedTimestamp =
      config.targetTimestamps.find { t ->
        prevBlockTimestamp < t && currentBlockTimestamp >= t
      }
    if (crossedTimestamp != null) {
      tsHit = true
      val targetBlockNumber =
        if (currentBlockNumber >= 1U) {
          currentBlockNumber - 1U
        } else {
          0U
        }
      requiredL1 = maxOf(requiredL1, targetBlockNumber)
    }

    lastImportedBlockTimestamp.set(currentBlockTimestamp)

    if (!blockHit && !tsHit) {
      return
    }

    requiredL1FinalizedL2BlockToResumeConflation.set(requiredL1)
    if (config.waitApiResumeAfterTargetBlock) {
      apiAcknowledged.set(false)
    } else {
      apiAcknowledged.set(true)
    }

    log.info(
      "target checkpoint pause: blockNumber={} requiredL1FinalizedL2BlockToResumeConflation={} waitL1={} waitApi={}",
      currentBlockNumber,
      requiredL1,
      config.waitTargetBlockL1Finalization,
      config.waitApiResumeAfterTargetBlock,
    )
  }

  /**
   * Whether conflation must stay idle for a target-checkpoint pause, and clears that pause when L1 finalization and/or
   * API gates are satisfied. Invoked only from [BlockCreationMonitor] each poll tick.
   *
   * @return `true` if the monitor must skip this tick (checkpoint pause still active);
   *   `false` if the monitor may run (no checkpoint pause, or pause was released).
   */
  override fun shouldPauseConflation(): Boolean {
    if (!pauseFeatureEnabled) {
      return false
    }

    val required = requiredL1FinalizedL2BlockToResumeConflation.get() ?: return false

    val l1Ok =
      !config.waitTargetBlockL1Finalization ||
        latestL1FinalizedBlockProvider.getLatestL1FinalizedBlock().toULong() >= required
    val apiOk =
      !config.waitApiResumeAfterTargetBlock ||
        apiAcknowledged.get()

    if (l1Ok && apiOk) {
      requiredL1FinalizedL2BlockToResumeConflation.set(null)
      apiAcknowledged.set(false)
      log.info("target checkpoint pause released")
      return false
    }
    return true
  }
}
