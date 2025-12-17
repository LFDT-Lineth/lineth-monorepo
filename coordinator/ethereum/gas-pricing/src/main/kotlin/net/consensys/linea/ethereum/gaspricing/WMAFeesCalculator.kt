package net.consensys.linea.ethereum.gaspricing

import linea.domain.FeeHistory
import linea.kotlin.toIntervalString
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

/**
 * Weighted Average Fees Calculator
 * WMA Gasfee = (lastBaseFee * baseFeeCoefficient) +
 *  (sumOf(reward[i] * ratio[i] * (i+1)) / sumOf(ratio[i] * (i+1))) * priorityFeeWmaCoefficient
 */
class WMAFeesCalculator(
  private val config: Config,
) : FeesCalculator {
  data class Config(
    val baseFeeCoefficient: Double,
    val priorityFeeWmaCoefficient: Double,
  )

  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val lastBlockBaseFeePerGas =
      feeHistory.baseFeePerGas[feeHistory.baseFeePerGas.lastIndex]
    val scaledBaseFee = this.config.baseFeeCoefficient * lastBlockBaseFeePerGas.toDouble()
    val weightedSumOfRewards =
      feeHistory.reward
        .mapIndexed { index, rewards ->
          feeHistory.gasUsedRatio[index] * (index + 1).toDouble() * rewards[0].toDouble()
        }
        .sum()

    val weightedRatiosSum =
      feeHistory.gasUsedRatio
        .mapIndexed { index, gasUsedRatio -> gasUsedRatio * (index + 1).toDouble() }
        .sum()

    try {
      val weightedPriorityFee =
        when {
          weightedSumOfRewards == 0.0 || weightedRatiosSum == 0.0 -> 0.0
          else -> (weightedSumOfRewards * this.config.priorityFeeWmaCoefficient) / weightedRatiosSum
        }

      val calculatedGasPrice = scaledBaseFee + weightedPriorityFee

      log.debug(
        "Calculated gasPrice={} wei, l1Blocks={} feeHistory={}",
        calculatedGasPrice,
        feeHistory.blocksRange().toIntervalString(),
        feeHistory,
      )
      return calculatedGasPrice
    } catch (e: Exception) {
      log.error(
        "Error: weightedSumOfRewards={} weightedRatiosSum={} feehistory={} errorMessage={}",
        weightedSumOfRewards,
        weightedRatiosSum,
        feeHistory,
        e.message,
        e,
      )
      throw e
    }
  }
}
