package lineth.ethereum.gaspricing.dynamiccap

import io.vertx.junit5.VertxExtension
import linea.domain.gas.GasPriceCaps
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.doThrow
import org.mockito.kotlin.mock
import java.math.BigDecimal
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.hours
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class GasPriceCapProviderImplV2Test {
  private val currentTime = Instant.parse("2024-03-20T00:00:00Z") // Wednesday
  private val gasFeePercentile = 10.0
  private val gasFeePercentileWindowInBlocks = 100U
  private val gasFeePercentileWindowLeewayInBlocks = 20U
  private val timeOfDayMultipliers = mapOf(
    "WEDNESDAY_0" to 1.0,
    "WEDNESDAY_1" to 2.0,
    "WEDNESDAY_2" to 3.0,
    "WEDNESDAY_3" to 4.0,
  )
  private val p10BaseFeeGas = 1000000000uL // 1GWei
  private val p10BaseFeeBlobGas = 100000000uL // 0.1GWei
  private val avgP10Reward = 200000000uL // 0.2GWei
  private val storedFeeHistoriesNum = 100
  private val adjustmentConstant = 25U
  private val finalizationTargetMaxDelay = 6.hours
  private val gasPriceCapsCoefficient = 1.0.div(1.1)
  private val gasPriceCapCalculator = GasPriceCapCalculatorImpl()

  private lateinit var targetBlockTime: Instant
  private lateinit var mockedL1FeeHistoriesRepository: FeeHistoriesRepositoryWithCache
  private lateinit var mockedClock: Clock

  private fun createGasPriceCapProvider(
    enabled: Boolean = true,
    gasFeePercentile: Double = this.gasFeePercentile,
    gasFeePercentileWindowInBlocks: UInt = this.gasFeePercentileWindowInBlocks,
    gasFeePercentileWindowLeewayInBlocks: UInt = this.gasFeePercentileWindowLeewayInBlocks,
    timeOfDayMultipliers: TimeOfDayMultipliers = this.timeOfDayMultipliers,
    adjustmentConstant: UInt = this.adjustmentConstant,
    blobAdjustmentConstant: UInt = this.adjustmentConstant,
    finalizationTargetMaxDelay: Duration = this.finalizationTargetMaxDelay,
    gasPriceCapsCoefficient: Double = this.gasPriceCapsCoefficient,
    feeHistoriesRepository: FeeHistoriesRepositoryWithCache = mockedL1FeeHistoriesRepository,
    gasPriceCapCalculator: GasPriceCapCalculator = this.gasPriceCapCalculator,
    clock: Clock = mockedClock,
  ): GasPriceCapProviderImplV2 {
    return GasPriceCapProviderImplV2(
      config = GasPriceCapProviderImplV2.Config(
        enabled = enabled,
        gasFeePercentile = gasFeePercentile,
        gasFeePercentileWindowInBlocks = gasFeePercentileWindowInBlocks,
        gasFeePercentileWindowLeewayInBlocks = gasFeePercentileWindowLeewayInBlocks,
        timeOfDayMultipliers = timeOfDayMultipliers,
        adjustmentConstant = adjustmentConstant,
        blobAdjustmentConstant = blobAdjustmentConstant,
        finalizationTargetMaxDelay = finalizationTargetMaxDelay,
        gasPriceCapsCoefficient = gasPriceCapsCoefficient,
      ),
      feeHistoriesRepository = feeHistoriesRepository,
      gasPriceCapCalculator = gasPriceCapCalculator,
      clock = clock,
    )
  }

  @BeforeEach
  fun beforeEach() {
    targetBlockTime = currentTime - 1.hours

    mockedL1FeeHistoriesRepository = mock<FeeHistoriesRepositoryWithCache> {
      on { getCachedNumOfFeeHistoriesFromBlockNumber() } doReturn storedFeeHistoriesNum
      on { getCachedPercentileGasFees() } doReturn PercentileGasFees(
        percentileBaseFeePerGas = p10BaseFeeGas,
        percentileBaseFeePerBlobGas = p10BaseFeeBlobGas,
        percentileAvgReward = avgP10Reward,
      )
    }

    mockedClock = mock<Clock> {
      on { now() } doReturn currentTime
    }
  }

  @Test
  fun `constructor throws error if config variables are invalid`() {
    val negativePercentile = -10.0
    assertThrows<IllegalArgumentException> {
      createGasPriceCapProvider(
        gasFeePercentile = negativePercentile,
      )
    }.also { exception ->
      assertThat(exception.message)
        .isEqualTo(
          "gasFeePercentile must be no less than 0.0. Value=$negativePercentile",
        )
    }

    val negativeDuration = (-1).hours
    assertThrows<IllegalArgumentException> {
      createGasPriceCapProvider(
        finalizationTargetMaxDelay = negativeDuration,
      )
    }.also { exception ->
      assertThat(exception.message)
        .isEqualTo(
          "finalizationTargetMaxDelay duration must be longer than zero second. Value=$negativeDuration",
        )
    }

    val negativeCoefficient = -1.0
    assertThrows<IllegalArgumentException> {
      createGasPriceCapProvider(
        gasPriceCapsCoefficient = negativeCoefficient,
      )
    }.also { exception ->
      assertThat(exception.message)
        .isEqualTo(
          "gasPriceCapsCoefficient must be greater than 0.0. Value=$negativeCoefficient",
        )
    }
  }

  @Test
  fun `gas price caps should be returned correctly`() {
    val gasPriceCapProvider = createGasPriceCapProvider()

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetBlockTime).get(),
    ).isEqualTo(
      GasPriceCaps(
        maxBaseFeePerGasCap = 1694444444uL,
        maxPriorityFeePerGasCap = 338888888uL,
        maxFeePerGasCap = 2033333332uL,
        maxFeePerBlobGasCap = 169444444uL,
      ),
    )
  }

  @Test
  fun `gas price caps with coefficient should be returned correctly`() {
    val gasPriceCapProvider = createGasPriceCapProvider()
    val coeff = gasPriceCapsCoefficient.toBigDecimal()
    val expectedMaxBaseFeePerGasCap = (1694444444.toBigDecimal() * coeff).toLong().toULong()
    val expectedMaxPriorityFeePerGasCap = (338888888.toBigDecimal() * coeff).toLong().toULong()
    val expectedMaxFeePerBlobGasCap =
      (169444444.toBigDecimal() * coeff).coerceAtLeast(BigDecimal.ONE).toLong().toULong()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetBlockTime).get(),
    ).isEqualTo(
      GasPriceCaps(
        maxBaseFeePerGasCap = expectedMaxBaseFeePerGasCap,
        maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap,
        maxFeePerGasCap = (expectedMaxBaseFeePerGasCap + expectedMaxPriorityFeePerGasCap),
        maxFeePerBlobGasCap = expectedMaxFeePerBlobGasCap,
      ),
    )
  }

  @Test
  fun `gas price coefficient requires a base fee cap`() {
    val gasPriceCaps = GasPriceCaps(
      maxBaseFeePerGasCap = null,
      maxPriorityFeePerGasCap = 1UL,
      maxFeePerGasCap = 1UL,
      maxFeePerBlobGasCap = 1UL,
    )

    val exception = assertThrows<IllegalArgumentException> {
      gasPriceCaps.withCoefficient(2.0)
    }

    assertThat(exception)
      .hasMessage(
        "maxBaseFeePerGasCap must be defined before applying the gas price caps coefficient",
      )
  }

  @Test
  fun `gas price caps should be null if disabled`() {
    val gasPriceCapProvider = createGasPriceCapProvider(
      enabled = false,
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetBlockTime).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetBlockTime).get(),
    ).isNull()
  }

  @Test
  fun `gas price caps should be null if not enough fee history data`() {
    val gasPriceCapProvider = createGasPriceCapProvider(
      gasFeePercentileWindowInBlocks = 200U,
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetBlockTime).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetBlockTime).get(),
    ).isNull()
  }

  @Test
  fun `gas price caps should be null if error on feeHistoriesRepository`() {
    mockedL1FeeHistoriesRepository = mock<FeeHistoriesRepositoryWithCache> {
      on { getCachedNumOfFeeHistoriesFromBlockNumber() } doReturn storedFeeHistoriesNum
      on { getCachedPercentileGasFees() } doThrow RuntimeException("Throw error for testing")
    }
    val gasPriceCapProvider = createGasPriceCapProvider()

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetBlockTime).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetBlockTime).get(),
    ).isNull()
  }

  @Test
  fun `time of day multiplier should default to 1_0 when key is missing`() {
    val baselineCaps = createGasPriceCapProvider().getGasPriceCaps(targetBlockTime).get()
    val gasPriceCapProvider = createGasPriceCapProvider(
      timeOfDayMultipliers = emptyMap(),
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetBlockTime).get(),
    ).isEqualTo(baselineCaps)
  }
}
