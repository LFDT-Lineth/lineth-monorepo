package linea.staterecovery.plugin

import linea.staterecovery.RecoveryStatusPersistence
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.plugin.data.AddedBlockContext
import org.hyperledger.besu.plugin.services.BesuEvents
import org.hyperledger.besu.plugin.services.mining.MiningService
import org.hyperledger.besu.plugin.services.p2p.P2PService
import org.hyperledger.besu.plugin.services.sync.SynchronizationService
import java.util.concurrent.atomic.AtomicBoolean

class RecoveryModeManager(
  private val synchronizationService: SynchronizationService,
  private val p2pService: P2PService,
  private val miningService: MiningService,
  private val recoveryStatePersistence: RecoveryStatusPersistence
) :
  BesuEvents.BlockAddedListener {
  private val log: Logger = LogManager.getLogger(RecoveryModeManager::class.java.name)
  private val recoveryModeTriggered = AtomicBoolean(false)
  private var currentBlockNumber: ULong = 0u
  val targetBlockNumber: ULong?
    get() = recoveryStatePersistence.getRecoveryStartBlockNumber()

  val headBlockNumber: ULong
    get() = currentBlockNumber

  /**
   * Called when a block is added.
   *
   * @param addedBlockContext the context of the added block
   */
  @Synchronized
  override fun onBlockAdded(addedBlockContext: AddedBlockContext) {
    val blockNumber = addedBlockContext.blockHeader.number
    currentBlockNumber = blockNumber.toULong()
    if (!recoveryModeTriggered.get() && hasReachedTargetBlock()) {
      switchToRecoveryMode()
    }
  }

  private fun hasReachedTargetBlock(): Boolean {
    return currentBlockNumber >= ((targetBlockNumber ?: ULong.MAX_VALUE) - 1u)
  }

  /**
   * Sets the target block number for switching to recovery mode.
   *
   * @param targetBlockNumber the target block number to set
   */
  @Synchronized
  fun setTargetBlockNumber(targetBlockNumber: ULong) {
    check(!recoveryModeTriggered.get()) {
      "Cannot set target block number after recovery mode has been triggered"
    }
    val effectiveRecoveryStartBlockNumber = if (targetBlockNumber <= currentBlockNumber + 1u) {
      log.warn(
        "targetBlockNumber={} is less than or equal to headBlockNumber={}" +
          " enabling recovery mode immediately at blockNumber={}",
        targetBlockNumber,
        currentBlockNumber,
        currentBlockNumber + 1u

      )
      switchToRecoveryMode()
      currentBlockNumber + 1u
    } else {
      targetBlockNumber
    }
    recoveryStatePersistence.saveRecoveryStartBlockNumber(effectiveRecoveryStartBlockNumber)
  }

  /** Switches the node to recovery mode.  */
  private fun switchToRecoveryMode() {
    check(!recoveryModeTriggered.get()) {
      "cannot enable already enabled recovery mode"
    }
    log.warn("Stopping synchronization service")
    synchronizationService.stop()

    log.warn("Stopping P2P discovery service")
    p2pService.disableDiscovery()

    log.warn("Stopping mining service")
    miningService.stop()

    log.info(
      "Switched to state recovery mode at block={}",
      headBlockNumber
    )
    recoveryModeTriggered.set(true)
  }
}
