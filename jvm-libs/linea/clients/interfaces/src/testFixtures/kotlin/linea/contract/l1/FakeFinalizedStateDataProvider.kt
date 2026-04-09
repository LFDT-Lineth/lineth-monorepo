package linea.contract.l1

import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeFinalizedStateDataProvider(
  private var errorBlockNumbers: Set<ULong> = emptySet(),
) : FinalizedStateDataProvider {
  private var blockNumber = 0UL
  fun setErrorBlockNumbers(errorBlockNumbers: Set<ULong>) {
    this.errorBlockNumbers = errorBlockNumbers
  }

  override fun getFinalizedStateData(
    blockParameter: BlockParameter,
  ): SafeFuture<FinalizedStateDataProvider.FinalizedStateData> {
    blockNumber = blockNumber + 1UL
    if (errorBlockNumbers.contains(blockNumber)) {
      throw Exception("Failure for the testing!")
    }
    return SafeFuture.completedFuture(
      FinalizedStateDataProvider.FinalizedStateData(
        blockNumber = blockNumber,
        forcedTransactionNumber = null,
      ),
    )
  }
}
