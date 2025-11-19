package linea.ethapi

import linea.domain.BlockParameter
import linea.domain.FeeHistory
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

interface EthApiFeeClient {
  fun ethGasPrice(): SafeFuture<BigInteger>
  fun ethMaxPriorityFeePerGas(): SafeFuture<BigInteger>
  fun ethFeeHistory(
    blockCount: Int,
    newestBlock: BlockParameter,
    rewardPercentiles: List<Double>,
  ): SafeFuture<FeeHistory>
}
