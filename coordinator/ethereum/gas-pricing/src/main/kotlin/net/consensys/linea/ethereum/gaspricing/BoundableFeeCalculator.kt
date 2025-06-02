package net.consensys.linea.ethereum.gaspricing

import linea.domain.FeeHistory
import linea.kotlin.toGWei
import linea.kotlin.toIntervalString
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class BoundableFeeCalculator(
  val config: Config,
  private val feesCalculator: FeesCalculator,
) : FeesCalculator {
  data class Config(
    val feeUpperBound: Double,
    val feeLowerBound: Double,
    val feeMargin: Double,
  ) {
    init {
      require(feeUpperBound >= 0.0 && feeLowerBound >= 0.0 && feeMargin >= 0.0) {
        "feeUpperBound, feeLowerBound, and feeMargin must be no less than 0."
      }

      require(feeUpperBound >= feeLowerBound) {
        "feeUpperBound must be no less than feeLowerBound."
      }
    }
  }

  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun calculateFees(feeHistory: FeeHistory): Double {
    val blockRange = feeHistory.blocksRange()
    val fee = feesCalculator.calculateFees(feeHistory) + config.feeMargin
    val gasPriceToUpdate = if (fee.compareTo(config.feeUpperBound) == 1) {
      log.debug(
        "Gas fee update: fee={}GWei, l1Blocks={}. fee is higher than feeUpperBound={}GWei." +
          " Will default to the upper bound value",
        fee.toGWei(),
        blockRange.toIntervalString(),
        config.feeUpperBound.toGWei(),
      )
      config.feeUpperBound
    } else if (fee.compareTo(config.feeLowerBound) == -1) {
      log.debug(
        "Gas fee update: fee={}GWei, l1Blocks={}. fee is lower than feeLowerBound={}GWei." +
          " Will default to the lower bound value",
        fee.toGWei(),
        blockRange.toIntervalString(),
        config.feeLowerBound.toGWei(),
      )
      config.feeLowerBound
    } else {
      log.debug(
        "Gas fee update: gasPrice={}GWei, l1Blocks={}.",
        fee.toGWei(),
        blockRange.toIntervalString(),
      )
      fee
    }
    return gasPriceToUpdate
  }
}
