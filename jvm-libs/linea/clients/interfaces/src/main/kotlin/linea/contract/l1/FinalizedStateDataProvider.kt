package linea.contract.l1

import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface FinalizedStateDataProvider {
  data class FinalizedStateData(
    val blockNumber: ULong,
    val forcedTransactionNumber: ULong?,
  )

  fun getFinalizedStateData(blockParameter: BlockParameter): SafeFuture<FinalizedStateData>
}
