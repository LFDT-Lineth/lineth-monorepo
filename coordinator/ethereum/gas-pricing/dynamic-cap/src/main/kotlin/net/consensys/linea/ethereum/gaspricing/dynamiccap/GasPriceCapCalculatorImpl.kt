package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.kotlin.toGWei
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.math.pow
import kotlin.time.Duration

class GasPriceCapCalculatorImpl : GasPriceCapCalculator {
  private val log: Logger = LogManager.getLogger(this::class.java)

  private fun calculateOverallMultiplier(
    adjustmentConstant: UInt,
    finalizationTargetMaxDelay: Duration,
    elapsedTimeSinceBlockTimestamp: Duration,
    timeOfDayMultiplier: Double
  ): Double {
    val finalizationDelayMultiplier = elapsedTimeSinceBlockTimestamp.inWholeSeconds.toDouble()
      .div(finalizationTargetMaxDelay.inWholeSeconds.toDouble()).pow(2)

    return 1.0 +
      adjustmentConstant.toDouble().times(timeOfDayMultiplier).times(finalizationDelayMultiplier)
  }

  override fun calculateGasPriceCap(
    adjustmentConstant: UInt,
    finalizationTargetMaxDelay: Duration,
    historicGasPriceCap: ULong,
    elapsedTimeSinceBlockTimestamp: Duration,
    timeOfDayMultiplier: Double
  ): ULong {
    require(finalizationTargetMaxDelay > Duration.ZERO) {
      "finalizationTargetMaxDelay duration must be longer than zero second." +
        " Value=$finalizationTargetMaxDelay"
    }

    val overallMultiplier = calculateOverallMultiplier(
      adjustmentConstant,
      finalizationTargetMaxDelay,
      elapsedTimeSinceBlockTimestamp,
      timeOfDayMultiplier
    )

    val gasPriceCap = historicGasPriceCap.toDouble() * overallMultiplier

    log.debug(
      "Gas price cap calculation: gasPriceCap={} GWei," +
        " historicGasPriceCap={} GWei, adjustmentConstant={}" +
        " finalizationTargetMaxDelay={} sec, elapsedTimeSinceBlockTimestamp={} sec," +
        " timeOfDayMultiplier={} overallMultiplier={}",
      gasPriceCap.toGWei(),
      historicGasPriceCap.toGWei(),
      adjustmentConstant,
      finalizationTargetMaxDelay.inWholeSeconds,
      elapsedTimeSinceBlockTimestamp.inWholeSeconds,
      timeOfDayMultiplier,
      overallMultiplier
    )

    return gasPriceCap.toULong()
  }
}
