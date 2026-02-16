package net.consensys.zkevm.ethereum.coordination.blockcreation

import linea.domain.Block
import linea.domain.BlockHeaderSummary
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface SafeBlockProvider {
  fun getLatestSafeBlock(): SafeFuture<Block>

  fun getLatestSafeBlockHeader(): SafeFuture<BlockHeaderSummary> {
    return getLatestSafeBlock().thenApply { it.headerSummary }
  }
}

/**
 * Provides the Safe Block Number (SBN) for conflation.
 * SBN represents the highest block number that is safe to conflate.
 *
 * For example, when Forced Transactions are in flight, conflation must be held back
 * to prevent conflating past the FTX execution blocks.
 */
interface ConflationSafeBlockNumberProvider {
  /**
   * Returns the highest safe block number that can be conflated:
   * - 0 until it safely can determine the correct height
   * - null if unrestricted.
   */
  fun getHighestSafeBlockNumber(): ULong?

  /**
   * Checks if the given block number is safe to conflate.
   * @param blockNumber The block number to check
   * @return true if the block can be safely conflated, false otherwise
   */
  fun isBlockSafeToConflate(blockNumber: ULong): Boolean {
    val safeBlockNumber = getHighestSafeBlockNumber()
    return safeBlockNumber == null || blockNumber <= safeBlockNumber
  }
}

class AlwaysSafeBlockNumberProvider : ConflationSafeBlockNumberProvider {
  override fun getHighestSafeBlockNumber(): ULong? = null
}
