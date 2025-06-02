package net.consensys.linea.ethereum.gaspricing.dynamiccap

import io.vertx.junit5.VertxExtension
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration.Companion.hours

@ExtendWith(VertxExtension::class)
class GasPriceCapCalculatorImplTest {
  @Test
  fun `calculateFeeGasPriceCap returns gas price cap correctly after 1 hour since last finalization`() {
    val gasPriceCapCalculator = GasPriceCapCalculatorImpl()

    // GasPriceCap = (historicGasPriceCap * (1 + adjustmentConstant * timeOfDayMultiplier *
    //               (timeSincePrevFinalization/finalizationTargetMaxDelay)^2)) + avgRewardAtPercentile
    //             = 1GWei * (1 + 25 * 1.75 * (1/32)^2) = 1.042724609GWei
    val calculatedGasPriceCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = 25U,
      finalizationTargetMaxDelay = 32.hours,
      historicGasPriceCap = 1000000000uL, // 1GWei
      elapsedTimeSinceBlockTimestamp = 1.hours,
      timeOfDayMultiplier = 1.75, // i.e. SUNDAY_2 in config/common/gas-price-cap-time-of-day-multipliers.toml
    )
    assertThat(calculatedGasPriceCap).isEqualTo(1042724609uL)
  }

  @Test
  fun `calculateFeeGasPriceCap returns gas price cap correctly after 16 hours since last finalization`() {
    val gasPriceCapCalculator = GasPriceCapCalculatorImpl()

    // GasPriceCap = (historicGasPriceCap * (1 + adjustmentConstant * timeOfDayMultiplier *
    //               (timeSincePrevFinalization/finalizationTargetMaxDelay)^2)) + avgRewardAtPercentile
    //             = 1GWei * (1 + 25 * 1.75 * (16/32)^2) = 11.9375GWei
    val calculatedGasPriceCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = 25U,
      finalizationTargetMaxDelay = 32.hours,
      historicGasPriceCap = 1000000000uL, // 1GWei
      elapsedTimeSinceBlockTimestamp = 16.hours,
      timeOfDayMultiplier = 1.75, // i.e. SUNDAY_2 in config/common/gas-price-cap-time-of-day-multipliers.toml
    )
    assertThat(calculatedGasPriceCap).isEqualTo(11937500000uL)
  }

  @Test
  fun `calculateFeeGasPriceCap returns gas price cap correctly after 32 hours since last finalization`() {
    val gasPriceCapCalculator = GasPriceCapCalculatorImpl()

    // GasPriceCap = (historicGasPriceCap * (1 + adjustmentConstant * timeOfDayMultiplier *
    //               (timeSincePrevFinalization/finalizationTargetMaxDelay)^2)) + avgRewardAtPercentile
    //             = 1GWei * (1 + 25 * 1.75 * (32/32)^2) = 44.75Wei
    val calculatedGasPriceCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = 25U,
      finalizationTargetMaxDelay = 32.hours,
      historicGasPriceCap = 1000000000uL, // 1GWei
      elapsedTimeSinceBlockTimestamp = 32.hours,
      timeOfDayMultiplier = 1.75, // i.e. SUNDAY_2 in config/common/gas-price-cap-time-of-day-multipliers.toml
    )
    assertThat(calculatedGasPriceCap).isEqualTo(44750000000uL)
  }

  @Test
  fun `calculateFeeGasPriceCap returns gas price cap correctly without specifying avgReward and tdm`() {
    val gasPriceCapCalculator = GasPriceCapCalculatorImpl()

    // GasPriceCap = (historicGasPriceCap * (1 + adjustmentConstant * timeOfDayMultiplier *
    //               (timeSincePrevFinalization/finalizationTargetMaxDelay)^2)) + avgRewardAtPercentile
    //             = 1GWei * (1 + 25 * 1.0 * (32/32)^2) = 26GWei
    val calculatedGasPriceCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = 25U,
      finalizationTargetMaxDelay = 32.hours,
      historicGasPriceCap = 1000000000uL, // 1GWei
      elapsedTimeSinceBlockTimestamp = 32.hours,
      // timeOfDayMultiplier = 1.0 as default
    )
    assertThat(calculatedGasPriceCap).isEqualTo(26000000000uL)
  }

  @Test
  fun `calculateFeeGasPriceCap throws error when finalizationTargetMaxDelay is 0`() {
    assertThrows<IllegalArgumentException> {
      GasPriceCapCalculatorImpl().calculateGasPriceCap(
        adjustmentConstant = 25U,
        finalizationTargetMaxDelay = 0.hours,
        historicGasPriceCap = 1000000000uL, // 1GWei
        elapsedTimeSinceBlockTimestamp = 32.hours,
      )
    }.also { exception ->
      assertThat(exception.message)
        .isEqualTo(
          "finalizationTargetMaxDelay duration must be longer than zero second. Value=0s",
        )
    }
  }
}
