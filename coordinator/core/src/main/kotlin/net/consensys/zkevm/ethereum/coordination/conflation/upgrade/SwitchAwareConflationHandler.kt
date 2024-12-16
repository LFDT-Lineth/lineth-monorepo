package net.consensys.zkevm.ethereum.coordination.conflation.upgrade

import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationHandler
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

@Deprecated("We may use it for 4844 switch, but the switch procedure is yet unknown")
@Suppress("DEPRECATION")
class SwitchAwareConflationHandler(
  private val oldHandler: ConflationHandler,
  private val newHandler: ConflationHandler,
  private val switchProvider: SwitchProvider,
  private val newVersion: SwitchProvider.ProtocolSwitches
) : ConflationHandler {
  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    return switchProvider.getSwitch(newVersion).thenCompose {
        switchBlock ->
      val conflationStartBlockNumber = conflation.blocks.first().number.toULong()
      val conflationEndBlockNumber = conflation.blocks.last().number.toULong()
      if (switchBlock == null || conflationStartBlockNumber < switchBlock) {
        log.debug("Handing conflation [$conflationStartBlockNumber, $conflationEndBlockNumber] over to old handler")
        oldHandler.handleConflatedBatch(conflation)
      } else {
        log.debug("Handing conflation [$conflationStartBlockNumber, $conflationEndBlockNumber] over to new handler")
        newHandler.handleConflatedBatch(conflation)
      }
    }
  }
}
