package linea.ftx.conflation

import net.consensys.zkevm.ethereum.coordination.blockcreation.ConflationSafeBlockNumberProvider

class ForcedTransactionConflationSafeBlockNumberProvider(
  private val listener: SafeBlockNumberUpdateListener? = null,
) :
  ConflationSafeBlockNumberProvider,
  SafeBlockNumberUpdateListener {

  private var safeBlockNumber: ULong? = 0UL

  @Synchronized
  override fun getHighestSafeBlockNumber(): ULong? = safeBlockNumber

  @Synchronized
  override fun onSafeBlockNumberUpdate(safeBlockNumber: ULong?) {
    this.safeBlockNumber = safeBlockNumber
    listener?.onSafeBlockNumberUpdate(safeBlockNumber)
  }
}
