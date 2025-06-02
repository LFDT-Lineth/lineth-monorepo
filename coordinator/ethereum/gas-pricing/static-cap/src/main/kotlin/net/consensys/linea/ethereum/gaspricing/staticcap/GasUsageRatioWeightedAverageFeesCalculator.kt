package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import linea.kotlin.toIntervalString
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

/**
 * Gas Ratio Weighted Average Fees Calculator
 * L2BaseGasPrice = (sumOf((baseFee[i]*baseFeeCoefficient + reward[i]*priorityFeeWmaCoefficient)*ratio[i]))/sumOf(ratio[i])
 * WeightedL2BlobGasPrice = (sumOf((baseBlobFee[i]*baseBlobFeeCoefficient)*blobRatio[i]))/sumOf(blobRatio[i]) * (131072 / 120_000)
 * L2FinalGasPrice = L2BaseGasPrice + WeightedL2BlobGasPrice
 */
class GasUsageRatioWeightedAverageFeesCalculator(
  val config: Config,
) : FeesCalculator {
  data class Config(
    val baseFeeCoefficient: Double,
    val priorityFeeCoefficient: Double,
    val baseFeeBlobCoefficient: Double,
    val blobSubmissionExpectedExecutionGas: Int,
    val expectedBlobGas: Int,
  )

  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val baseFeePerGasList = feeHistory.baseFeePerGas
    val priorityFeesPerGasList = feeHistory.reward.map { it[0] }
    val baseFeePerBlobGasList = feeHistory.baseFeePerBlobGas.ifEmpty {
      List(baseFeePerGasList.size) { 0uL }
    }

    val executionGasPrice = calculateExecutionGasPrice(feeHistory, baseFeePerGasList, priorityFeesPerGasList)
    val weightedL2BlobGasPrice = calculateWeightedBlobGasPrice(feeHistory, baseFeePerBlobGasList)

    val l2GasPrice = executionGasPrice + weightedL2BlobGasPrice

    log.debug(
      "Calculated gasPrice={} wei, executionGasPrice={} wei, weightedL2BlobGasPrice={} wei, l1Blocks={} feeHistory={}",
      l2GasPrice,
      executionGasPrice,
      weightedL2BlobGasPrice,
      feeHistory.blocksRange().toIntervalString(),
      feeHistory,
    )
    return l2GasPrice
  }

  private fun calculateExecutionGasPrice(
    feeHistory: FeeHistory,
    baseFeePerGasList: List<ULong>,
    priorityFeesPerGasList: List<ULong>,
  ): Double {
    var gasUsageRatioList = feeHistory.gasUsedRatio
    var gasUsageRatiosSum = gasUsageRatioList.sumOf { it }

    if (gasUsageRatiosSum.compareTo(0.0) == 0) {
      log.warn(
        "GasUsedRatio is zero for all l1Blocks={}. Will fallback to Simple Average.",
        feeHistory.blocksRange().toIntervalString(),
      )
      // Giving a weight of one will yield the simple average
      gasUsageRatioList = feeHistory.gasUsedRatio.map { 1.0 }
      gasUsageRatiosSum = feeHistory.gasUsedRatio.size.toDouble()
    }

    val weightedFeesSum = gasUsageRatioList.mapIndexed { index, gasUsedRatio ->
      val baseFeeFractional = baseFeePerGasList[index].toDouble() * this.config.baseFeeCoefficient
      val priorityFeeFractional = priorityFeesPerGasList[index].toDouble() * this.config.priorityFeeCoefficient

      (baseFeeFractional + priorityFeeFractional) * gasUsedRatio
    }.sum()

    return weightedFeesSum / gasUsageRatiosSum
  }

  private fun calculateWeightedBlobGasPrice(
    feeHistory: FeeHistory,
    baseFeePerBlobGasList: List<ULong>,
  ): Double {
    val blobGasUsageRatio = if (feeHistory.blobGasUsedRatio.sumOf { it }.compareTo(0.0) == 0) {
      log.warn(
        "BlobGasUsedRatio is zero for all l1Blocks={}. Will fallback to Simple Average.",
        feeHistory.blocksRange().toIntervalString(),
      )
      List(feeHistory.gasUsedRatio.size) { 1.0 }
    } else {
      feeHistory.blobGasUsedRatio
    }

    val blobGasUsageRatiosSum = blobGasUsageRatio.sumOf { it }

    val weightedBlobFeesSum = blobGasUsageRatio
      .zip(baseFeePerBlobGasList) { blobGasUsedRatio, baseFeePerBlobGas ->
        baseFeePerBlobGas.toDouble() * blobGasUsedRatio
      }.sum() * this.config.baseFeeBlobCoefficient

    return weightedBlobFeesSum * config.expectedBlobGas /
      (blobGasUsageRatiosSum * config.blobSubmissionExpectedExecutionGas)
  }
}
