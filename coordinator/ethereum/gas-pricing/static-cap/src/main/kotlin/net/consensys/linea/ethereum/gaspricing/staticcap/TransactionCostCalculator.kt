package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

/**
 * This calculator is supposed to compute cost of a plain transfer transaction and multiply it by a constant
 * Cost of a plain transfer transaction should be computed the same way as on Sequencer
 * @see <a href=”https://github.com/Consensys/linea-sequencer/blob/main/sequencer/src/main/java/net/consensys/linea/bl/TransactionProfitabilityCalculator.java#L38”>this</a>
 * */
class TransactionCostCalculator(
  private val dataCostCalculator: FeesCalculator,
  private val config: Config
) : FeesCalculator {
  data class Config(
    val sampleTransactionCostMultiplier: Double,
    val fixedCostWei: ULong,
    val compressedTxSize: Int = 125,
    val expectedGas: Int = 21000
  )

  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val dataCostCost = dataCostCalculator.calculateFees(feeHistory)
    log.trace("Data cost: $dataCostCost")
    return config.sampleTransactionCostMultiplier *
      ((dataCostCost * config.compressedTxSize / config.expectedGas) + config.fixedCostWei.toDouble())
  }
}
