package lineth.coordination

import lineth.coordination.aggregation.AggregationL2StateProviderImpl.Companion.GENESIS_ZERO_HASH
import lineth.persistence.ForcedTransactionsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class FtxRollingInfo(
  val ftxNumber: ULong,
  val ftxRollingHash: ByteArray,
) {
  companion object {
    val GENESIS = FtxRollingInfo(0uL, GENESIS_ZERO_HASH.copyOf())
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as FtxRollingInfo

    if (ftxNumber != other.ftxNumber) return false
    if (!ftxRollingHash.contentEquals(other.ftxRollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = ftxNumber.hashCode()
    result = 31 * result + ftxRollingHash.contentHashCode()
    return result
  }
}

fun interface FtxRollingInfoProvider {
  fun getFtxRollingHashByBlockNumber(blockNumber: ULong): SafeFuture<FtxRollingInfo>
}

class FtxRollingInfoProviderImpl(
  private val forcedTransactionsDao: ForcedTransactionsDao,
) : FtxRollingInfoProvider {
  override fun getFtxRollingHashByBlockNumber(blockNumber: ULong): SafeFuture<FtxRollingInfo> {
    if (blockNumber == 0uL) {
      // return genesis ftx number and hash
      return SafeFuture.completedFuture(FtxRollingInfo.GENESIS)
    }

    return forcedTransactionsDao
      .findHighestForcedTransaction(upToSimulatedExecutionBlockNumberInclusive = blockNumber)
      .thenApply { highestFtx ->
        highestFtx
          ?.let { FtxRollingInfo(it.ftxNumber, it.ftxRollingHash) }
          ?: FtxRollingInfo.GENESIS
      }
  }
}
