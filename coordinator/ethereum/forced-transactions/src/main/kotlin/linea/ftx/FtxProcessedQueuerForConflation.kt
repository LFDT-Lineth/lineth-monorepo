package linea.ftx

import linea.ftx.conflation.FtxConflationInfo
import java.util.Queue

class FtxProcessedQueuerForConflation(
  val conflationFtxQueue: Queue<FtxConflationInfo>,
  val aggregationFtxQueue: Queue<FtxConflationInfo>,
) : ForcedTransactionProcessedListener {

  /**
   * Add the ftx for conflation and aggregation queues.
   * It fails if any of them is full, the caller shall retry latter
   */
  override fun onFtxProcessed(ftxStatus: FtxConflationInfo) {
    conflationFtxQueue.add(ftxStatus)
    aggregationFtxQueue.add(ftxStatus)
  }
}
