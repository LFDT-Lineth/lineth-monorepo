package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import linea.kotlin.toIntervalString
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

open class AverageWeightedFeesCalculator(
  val feeListFetcher: (FeeHistory) -> List<ULong>,
  val ratioListFetcher: (FeeHistory) -> List<Double>,
  val log: Logger
) : FeesCalculator {
  override fun calculateFees(feeHistory: FeeHistory): Double {
    val feeList = feeListFetcher(feeHistory)
    if (feeList.isEmpty()) {
      return 0.0
    }
    val ratioList = if (ratioListFetcher(feeHistory).sumOf { it } == 0.0) {
      log.warn(
        "RatioSum is zero for all l1Blocks={}. Will fallback to Simple Average.",
        feeHistory.blocksRange().toIntervalString()
      )
      List(ratioListFetcher(feeHistory).size) { 1.0 }
    } else {
      ratioListFetcher(feeHistory)
    }
    val weightedFeesSum = feeList.zip(ratioList).sumOf { it.first.toDouble() * it.second }
    val ratioSum = ratioList.sumOf { it }
    return weightedFeesSum / ratioSum
  }
}

object AverageWeightedBaseFeesCalculator : AverageWeightedFeesCalculator(
  feeListFetcher = { feeHistory -> feeHistory.baseFeePerGas },
  ratioListFetcher = { feeHistory -> feeHistory.gasUsedRatio },
  log = LogManager.getLogger(AverageWeightedBaseFeesCalculator::class.java)
)

object AverageWeightedPriorityFeesCalculator : AverageWeightedFeesCalculator(
  feeListFetcher = { feeHistory -> feeHistory.reward.map { it.first() } },
  ratioListFetcher = { feeHistory -> feeHistory.gasUsedRatio },
  log = LogManager.getLogger(AverageWeightedPriorityFeesCalculator::class.java)
)

object AverageWeightedBlobBaseFeesCalculator : AverageWeightedFeesCalculator(
  feeListFetcher = { feeHistory ->
    feeHistory.baseFeePerBlobGas.ifEmpty { List(feeHistory.baseFeePerGas.size) { 0uL } }
  },
  ratioListFetcher = { feeHistory -> feeHistory.blobGasUsedRatio },
  log = LogManager.getLogger(AverageWeightedBlobBaseFeesCalculator::class.java)
)
