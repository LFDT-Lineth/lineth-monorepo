package linea.ftx

import linea.contract.l1.LinethRollupFinalizedState
import linea.contract.l1.LinethRollupSmartContractClientReadOnlyFinalizedStateProvider
import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock

class FakeLinethRollupSmartContractClientReadOnlyFinalizedStateProvider(
  var l1FinalizedState: LinethRollupFinalizedState = LinethRollupFinalizedState(
    blockNumber = 0UL,
    blockTimestamp = Clock.System.now(),
    messageNumber = 0UL,
    forcedTransactionNumber = 10UL,
  ),
) :
  LinethRollupSmartContractClientReadOnlyFinalizedStateProvider {

  override fun getLatestFinalizedState(blockParameter: BlockParameter): SafeFuture<LinethRollupFinalizedState> {
    return SafeFuture.completedFuture(l1FinalizedState)
  }
}
