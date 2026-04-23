package linea.ftx.conflation

import linea.forcedtx.ForcedTransactionInclusionResult

data class FtxConflationInfo(
  val ftxNumber: ULong,
  val blockNumber: ULong,
  val inclusionResult: ForcedTransactionInclusionResult,
) {
  fun toStringShortForLogging(): String {
    return "ftx=$ftxNumber blockNumber=$blockNumber result=$inclusionResult"
  }
}
