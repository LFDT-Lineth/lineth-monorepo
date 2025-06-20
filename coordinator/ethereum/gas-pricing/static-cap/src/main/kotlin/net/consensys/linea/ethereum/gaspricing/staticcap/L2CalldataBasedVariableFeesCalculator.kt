package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.L2CalldataSizeAccumulator
import org.apache.logging.log4j.LogManager
import java.util.concurrent.atomic.AtomicReference

/*
  CALLDATA_BASED_FEE_CHANGE_DENOMINATOR = 32
  CALLDATA_BASED_FEE_BLOCK_COUNT = 5
  MAX_BLOCK_CALLDATA_SIZE = 109000

  variable_cost = as documented in VARIABLE_COST (https://docs.linea.build/get-started/how-to/gas-fees#gas-pricing)
  calldata_target = CALLDATA_BASED_FEE_BLOCK_COUNT * MAX_BLOCK_CALLDATA_SIZE / 2
  # delta fluctuates between [-1 and 1]
  delta = (sum(block_size over CALLDATA_BASED_FEE_BLOCK_COUNT) - calldata_target) / calldata_target
  variable_cost = max(variable_cost, previous_variable_cost * ( 1 + delta / CALLDATA_BASED_FEE_CHANGE_DENOMINATOR )
*/
class L2CalldataBasedVariableFeesCalculator(
  val config: Config,
  val variableFeesCalculator: FeesCalculator,
  val l2CalldataSizeAccumulator: L2CalldataSizeAccumulator,
) : FeesCalculator {
  data class Config(
    val feeChangeDenominator: UInt,
    val calldataSizeBlockCount: UInt,
    val maxBlockCalldataSize: UInt,
  ) {
    init {
      require(feeChangeDenominator > 0u) { "feeChangeDenominator=$feeChangeDenominator must be greater than 0" }
      require(maxBlockCalldataSize > 0u) { "maxBlockCalldataSize=$maxBlockCalldataSize must be greater than 0" }
    }
  }

  private val log = LogManager.getLogger(this::class.java)
  private var lastVariableCost: AtomicReference<Double> = AtomicReference(0.0)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val variableFee = variableFeesCalculator.calculateFees(feeHistory)

    if (config.calldataSizeBlockCount == 0u) {
      log.debug(
        "Calldata-based variable fee is disabled as calldataSizeBlockCount is set as 0: variableFee={} wei",
        variableFee,
      )
      return variableFee
    }

    val callDataTargetSize = config.maxBlockCalldataSize
      .times(config.calldataSizeBlockCount)
      .toDouble().div(2.0)

    val delta = (
      l2CalldataSizeAccumulator
        .getSumOfL2CalldataSize().get().toDouble()
        .minus(callDataTargetSize)
      )
      .div(callDataTargetSize)
      .coerceAtLeast(-1.0)
      .coerceAtMost(1.0)

    val calldataBasedVariableFee =
      lastVariableCost.get().times(1.0 + (delta.div(config.feeChangeDenominator.toDouble())))

    val finalVariableFee = variableFee.coerceAtLeast(calldataBasedVariableFee)

    lastVariableCost.set(finalVariableFee)

    log.debug(
      "Calculated calldataBasedVariableFee={} wei variableFee={} wei finalVariableFee={} wei " +
        "delta={} maxBlockCalldataSize={} calldataSizeBlockCount={} feeChangeDenominator={}",
      calldataBasedVariableFee,
      variableFee,
      finalVariableFee,
      delta,
      config.maxBlockCalldataSize,
      config.calldataSizeBlockCount,
      config.feeChangeDenominator,
    )

    return finalVariableFee
  }
}
