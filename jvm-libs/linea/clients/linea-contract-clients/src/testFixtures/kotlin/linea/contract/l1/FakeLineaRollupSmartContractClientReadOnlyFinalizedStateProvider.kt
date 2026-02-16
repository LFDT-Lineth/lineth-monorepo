package linea.ftx

import linea.contract.l1.LineaRollupFinalizedState
import linea.contract.l1.LineaRollupSmartContractClientReadOnlyFinalizedStateProvider
import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock

class FakeLineaRollupSmartContractClientReadOnlyFinalizedStateProvider(
  var l1FinalizedState: LineaRollupFinalizedState = LineaRollupFinalizedState(
    blockNumber = 0UL,
    blockTimestamp = Clock.System.now(),
    messageNumber = 0UL,
    forcedTransactionNumber = 10UL,
  ),
) :
  LineaRollupSmartContractClientReadOnlyFinalizedStateProvider {

  override fun getLatestFinalizedState(blockParameter: BlockParameter): SafeFuture<LineaRollupFinalizedState> {
    return SafeFuture.completedFuture(l1FinalizedState)
  }
}
