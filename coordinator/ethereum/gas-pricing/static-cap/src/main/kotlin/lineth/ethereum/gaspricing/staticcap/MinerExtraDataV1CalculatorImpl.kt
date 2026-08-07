package lineth.ethereum.gaspricing.staticcap

import lineth.domain.FeeHistory
import lineth.kotlin.tokWeiUInt
import lineth.ethereum.gaspricing.FeesCalculator
import lineth.ethereum.gaspricing.MinerExtraDataCalculator
import lineth.ethereum.gaspricing.MinerExtraDataV1

class MinerExtraDataV1CalculatorImpl(
  val config: Config,
  private val variableFeesCalculator: FeesCalculator,
  private val legacyFeesCalculator: FeesCalculator,
) : MinerExtraDataCalculator {

  data class Config(
    val fixedCostInKWei: UInt,
    val ethGasPriceMultiplier: Double,
  )

  override fun calculateMinerExtraData(feeHistory: FeeHistory): MinerExtraDataV1 {
    val variableFees = variableFeesCalculator.calculateFees(feeHistory)
    val legacyFees = legacyFeesCalculator.calculateFees(feeHistory) * config.ethGasPriceMultiplier
    return MinerExtraDataV1(
      fixedCostInKWei = config.fixedCostInKWei,
      variableCostInKWei = variableFees.tokWeiUInt(),
      ethGasPriceInKWei = legacyFees.tokWeiUInt(),
    )
  }
}
