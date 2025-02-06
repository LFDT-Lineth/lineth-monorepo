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
  private val recoveryStatePersistence: RecoveryStatusPersistence,
  private val debugForceSyncStopBlockNumber: ULong? = null,
  headBlockNumber: ULong
) :
  BesuEvents.BlockAddedListener {
  private val log: Logger = LogManager.getLogger(RecoveryModeManager::class.java.name)
  private val recoveryModeTriggered = AtomicBoolean(false)
  val targetBlockNumber: ULong?
    get() = recoveryStatePersistence.getRecoveryStartBlockNumber()
  var headBlockNumber: ULong = headBlockNumber
    private set

  init {
    log.info("RecoveryModeManager initializing: headBlockNumber={}", headBlockNumber)
    enableRecoveryModeIfNecessary()
  }

  @Synchronized
  fun enableRecoveryModeIfNecessary() {
    if (hasReachedTargetBlock()) {
      log.info(
        "enabling recovery mode immediately at blockNumber={} recoveryTargetBlockNumber={}",
        headBlockNumber,
        targetBlockNumber
      )
      switchToRecoveryMode()
    }
  }

  /**
   * Called when a block is added.
   *
   * @param addedBlockContext the context of the added block
   */
  @Synchronized
  override fun onBlockAdded(addedBlockContext: AddedBlockContext) {
    val blockNumber = addedBlockContext.blockHeader.number
    headBlockNumber = blockNumber.toULong()
    if (!recoveryModeTriggered.get() && hasReachedTargetBlock()) {
      log.info(
        "Stopping synchronization services at block={} recoveryTargetBlockNumber={} was reached",
        headBlockNumber,
        targetBlockNumber
      )
      switchToRecoveryMode()
    } else if (debugForceSyncStopBlockNumber != null && headBlockNumber >= debugForceSyncStopBlockNumber) {
      log.info(
        "Stopping synchronization services at block={} debugForceSyncStopBlockNumber={}",
        headBlockNumber,
        debugForceSyncStopBlockNumber
      )
      stopBesuServices()
    }
  }

  private fun hasReachedTargetBlock(
    headBlockNumber: ULong = this.headBlockNumber,
    targetBlockNumber: ULong? = this.targetBlockNumber
  ): Boolean {
    return (headBlockNumber + 1u) >= (targetBlockNumber ?: ULong.MAX_VALUE)
  }

  /**
   * Sets the target block number for switching to recovery mode.
   *
   * @param targetBlockNumber the target block number to set
   */
  @Synchronized
  fun setTargetBlockNumber(targetBlockNumber: ULong) {
    if (recoveryModeTriggered.get()) {
      if (targetBlockNumber == this.targetBlockNumber) {
        log.info("recovery mode already enabled at blockNumber={}", headBlockNumber)
        return
      } else {
        check(!recoveryModeTriggered.get()) {
          "recovery mode has already been triggered at block=${this.targetBlockNumber} " +
            "trying new target=$targetBlockNumber"
        }
      }
    }

    val effectiveRecoveryStartBlockNumber =
      if (hasReachedTargetBlock(headBlockNumber, targetBlockNumber)) {
        val effectiveRecoveryStartBlockNumber = headBlockNumber + 1u
        effectiveRecoveryStartBlockNumber
      } else {
        targetBlockNumber
      }
    recoveryStatePersistence.saveRecoveryStartBlockNumber(effectiveRecoveryStartBlockNumber)
    enableRecoveryModeIfNecessary()
  }

  private fun switchToRecoveryMode() {
    stopBesuServices()
    recoveryModeTriggered.set(true)
  }

  private fun stopBesuServices() {
    log.info("Stopping synchronization service")
    synchronizationService.stop()

    log.info("Stopping P2P discovery service")
    p2pService.disableDiscovery()

    log.info("Stopping mining service")
    miningService.stop()
  }
}
