package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import linea.kotlin.toIntervalString
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import org.apache.logging.log4j.LogManager

/*
  variableFees = (
    (averageWeightedBaseFee + averageWeightedPriorityFee) * blob-submission-expected-execution-gas +
    averageWeightedBlobBaseFee * expected-blob-gas
  ) / bytes-per-data-submission * margin
*/
class VariableFeesCalculator(
  val config: Config,
  val averageWeightedBaseFeesCalculator: FeesCalculator = AverageWeightedBaseFeesCalculator,
  val averageWeightedPriorityFeesCalculator: FeesCalculator = AverageWeightedPriorityFeesCalculator,
  val averageWeightedBlobBaseFeesCalculator: FeesCalculator = AverageWeightedBlobBaseFeesCalculator
) : FeesCalculator {
  data class Config(
    val blobSubmissionExpectedExecutionGas: UInt,
    val bytesPerDataSubmission: UInt,
    val expectedBlobGas: UInt,
    val margin: Double
  )

  private val log = LogManager.getLogger(this::class.java)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val averageWeightedBaseFees = averageWeightedBaseFeesCalculator.calculateFees(feeHistory)
    val averageWeightedPriorityFees = averageWeightedPriorityFeesCalculator.calculateFees(feeHistory)
    val averageWeightedBlobBaseFees = averageWeightedBlobBaseFeesCalculator.calculateFees(feeHistory)

    val executionFee = (averageWeightedBaseFees + averageWeightedPriorityFees) *
      config.blobSubmissionExpectedExecutionGas.toDouble()

    val blobFee = averageWeightedBlobBaseFees * config.expectedBlobGas.toDouble()

    val variableFee = ((executionFee + blobFee) * config.margin) / config.bytesPerDataSubmission.toDouble()

    log.debug(
      "Calculated variableFee={} wei executionFee={} wei blobFee={} wei bytesPerDataSubmission={} l1Blocks={}",
      variableFee,
      executionFee,
      blobFee,
      config.bytesPerDataSubmission,
      feeHistory.blocksRange().toIntervalString()
    )

    log.trace("feeHistory={}", feeHistory)

    return variableFee
  }
}
