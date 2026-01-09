package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import linea.ethapi.EthApiBlockClient
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.HistoricVariableCostProvider
import net.consensys.linea.ethereum.gaspricing.L2CalldataSizeAccumulator
import org.apache.logging.log4j.LogManager

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
  val ethApiBlockClient: EthApiBlockClient,
  val variableFeesCalculator: FeesCalculator,
  val l2CalldataSizeAccumulator: L2CalldataSizeAccumulator,
  val historicVariableCostProvider: HistoricVariableCostProvider,
) : FeesCalculator {
  data class Config(
    val feeChangeDenominator: UInt,
    val calldataSizeBlockCount: UInt,
    val maxBlockCalldataSize: UInt,
  ) {
    init {
      require(feeChangeDenominator > 0u) { "feeChangeDenominator=$feeChangeDenominator must be greater than 0" }
      require(maxBlockCalldataSize > 0u) { "maxBlockCalldataSize=$maxBlockCalldataSize must be greater than 0" }
      require(calldataSizeBlockCount > 0u) { "calldataSizeBlockCount=$calldataSizeBlockCount must be greater than 0" }
    }
  }

  private val log = LogManager.getLogger(this::class.java)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val variableFee = variableFeesCalculator.calculateFees(feeHistory)

    val callDataTargetSize = config.maxBlockCalldataSize
      .times(config.calldataSizeBlockCount)
      .toDouble().div(2.0)

    val (sumOfL2CalldataSize, latestVariableCost) = ethApiBlockClient.ethBlockNumber()
      .thenCompose { latestBlockNumber ->
        l2CalldataSizeAccumulator.getSumOfL2CalldataSize(latestBlockNumber)
          .thenCombine(
            historicVariableCostProvider.getVariableCost(latestBlockNumber),
          ) { sumOfL2CalldataSize, latestVariableCost ->
            sumOfL2CalldataSize to latestVariableCost
          }
      }.get()

    val delta = sumOfL2CalldataSize.toDouble()
      .minus(callDataTargetSize)
      .div(callDataTargetSize)
      .coerceAtLeast(-1.0)
      .coerceAtMost(1.0)

    val calldataBasedVariableFee =
      latestVariableCost.times(1.0 + (delta.div(config.feeChangeDenominator.toDouble())))

    val finalVariableFee = variableFee.coerceAtLeast(calldataBasedVariableFee)

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
