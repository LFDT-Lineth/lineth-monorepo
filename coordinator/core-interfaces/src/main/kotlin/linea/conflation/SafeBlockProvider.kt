package linea.conflation

import linea.domain.Block
import linea.domain.BlockHeaderSummary
import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import linea.kotlin.minusCoercingUnderflow
import net.consensys.linea.async.toSafeFuture
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface SafeBlockProvider {
  fun getLatestSafeBlock(): SafeFuture<Block>

  fun getLatestSafeBlockHeader(): SafeFuture<BlockHeaderSummary> {
    return getLatestSafeBlock().thenApply { it.headerSummary }
  }
}

class FixedLaggingHeadSafeBlockProvider(
  private val ethApiBlockClient: EthApiBlockClient,
  private val blocksToFinalization: ULong,
) : SafeBlockProvider {
  override fun getLatestSafeBlock(): SafeFuture<Block> {
    if (blocksToFinalization == 0UL) {
      return ethApiBlockClient.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).toSafeFuture()
    }

    return ethApiBlockClient
      .ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST)
      .thenCompose { block ->
        val safeBlockNumber = block.number.minusCoercingUnderflow(blocksToFinalization)
        ethApiBlockClient.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(safeBlockNumber)).toSafeFuture()
      }
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
