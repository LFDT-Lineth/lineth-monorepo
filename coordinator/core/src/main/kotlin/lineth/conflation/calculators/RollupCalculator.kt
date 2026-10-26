package lineth.conflation.calculators

import linea.domain.BlocksConflation

enum class RollupTrigger {
  CONFLATION_COUNT,
  TARGET_BLOCK_NUMBER,
}

fun interface RollupHandler {
  fun onRollup(window: List<BlocksConflation>)
}

interface RollupCalculator {
  fun newConflation(conflation: BlocksConflation)

  fun onRollup(handler: RollupHandler)
}

interface RollupTriggerCalculator {
  fun appendConflation(conflation: BlocksConflation)

  fun checkTrigger(conflation: BlocksConflation): RollupTrigger?

  fun reset()
}
