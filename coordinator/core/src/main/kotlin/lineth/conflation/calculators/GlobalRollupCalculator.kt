package lineth.conflation.calculators

import linea.domain.BlocksConflation

class GlobalRollupCalculator(
  private val triggerCalculators: List<RollupTriggerCalculator>,
) : RollupCalculator {
  private val pendingWindow = mutableListOf<BlocksConflation>()
  private var rollupHandler: RollupHandler? = null

  override fun onRollup(handler: RollupHandler) {
    rollupHandler = handler
  }

  override fun newConflation(conflation: BlocksConflation) {
    triggerCalculators.forEach { it.appendConflation(conflation) }
    pendingWindow += conflation
    if (triggerCalculators.any { it.checkTrigger(conflation) != null }) {
      val window = pendingWindow.toList()
      pendingWindow.clear()
      triggerCalculators.forEach { it.reset() }
      rollupHandler?.onRollup(window)
    }
  }
}
