package net.consensys.linea.ethereum.gaspricing.dynamiccap

import io.vertx.junit5.VertxExtension
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.domain.BlockWithTxHashes
import linea.domain.gas.GasPriceCaps
import linea.ethapi.EthApiBlockClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.hours

@ExtendWith(VertxExtension::class)
class GasPriceCapProviderImplTest {
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
  private val avgP10Reward = 200000000uL // 2GWei
  private val storedFeeHistoriesNum = 100
  private val adjustmentConstant = 25U
  private val finalizationTargetMaxDelay = 6.hours
  private val gasPriceCapsCoefficient = 1.0.div(1.1)
  private val gasPriceCapCalculator = GasPriceCapCalculatorImpl()

  private lateinit var targetBlockTime: Instant
  private lateinit var mockedL2EthApiBlockClient: EthApiBlockClient
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
    l2EthApiBlockClient: EthApiBlockClient = mockedL2EthApiBlockClient,
    feeHistoriesRepository: FeeHistoriesRepositoryWithCache = mockedL1FeeHistoriesRepository,
    gasPriceCapCalculator: GasPriceCapCalculator = this.gasPriceCapCalculator,
    clock: Clock = mockedClock,
  ): GasPriceCapProviderImpl {
    return GasPriceCapProviderImpl(
      config = GasPriceCapProviderImpl.Config(
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
      l2EthApiBlockClient = l2EthApiBlockClient,
      feeHistoriesRepository = feeHistoriesRepository,
      gasPriceCapCalculator = gasPriceCapCalculator,
      clock = clock,
    )
  }

  @BeforeEach
  fun beforeEach() {
    targetBlockTime = currentTime - 1.hours
    val mockBlock = mock<BlockWithTxHashes> {
      on { timestamp } doReturn targetBlockTime.epochSeconds.toULong()
    }
    mockedL2EthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.completedFuture(
        mockBlock,
      )
    }

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
    val targetL2BlockNumber = 100L
    val gasPriceCapProvider = createGasPriceCapProvider()

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetL2BlockNumber).get(),
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
    val targetL2BlockNumber = 100L
    val gasPriceCapProvider = createGasPriceCapProvider()
    val expectedMaxBaseFeePerGasCap = (1694444444 * gasPriceCapsCoefficient).toULong()
    val expectedMaxPriorityFeePerGasCap = (338888888 * gasPriceCapsCoefficient).toULong()
    val expectedMaxFeePerBlobGasCap = (169444444 * gasPriceCapsCoefficient).toULong()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetL2BlockNumber).get(),
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
  fun `gas price caps should be null if disabled`() {
    val targetL2BlockNumber = 100L
    val gasPriceCapProvider = createGasPriceCapProvider(
      enabled = false,
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetL2BlockNumber).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetL2BlockNumber).get(),
    ).isNull()
  }

  @Test
  fun `gas price caps should be null if not enough fee history data`() {
    val targetL2BlockNumber = 100L
    val gasPriceCapProvider = createGasPriceCapProvider(
      gasFeePercentileWindowInBlocks = 200U,
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetL2BlockNumber).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetL2BlockNumber).get(),
    ).isNull()
  }

  @Test
  fun `gas price caps should be null if error on feeHistoriesRepository`() {
    val targetL2BlockNumber = 100L
    mockedL1FeeHistoriesRepository = mock<FeeHistoriesRepositoryWithCache> {
      on { getNumOfFeeHistoriesFromBlockNumber(any(), any()) } doReturn SafeFuture.failedFuture(
        Error("Throw error for testing"),
      )
    }
    val gasPriceCapProvider = createGasPriceCapProvider(
      l2EthApiBlockClient = mockedL2EthApiBlockClient,
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetL2BlockNumber).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetL2BlockNumber).get(),
    ).isNull()
  }

  @Test
  fun `gas price caps should be null if error on l2ExtendedWeb3JClient`() {
    val targetL2BlockNumber = 100L
    mockedL2EthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.failedFuture(
        Error("Throw error for testing"),
      )
    }
    val gasPriceCapProvider = createGasPriceCapProvider(
      l2EthApiBlockClient = mockedL2EthApiBlockClient,
    )

    assertThat(
      gasPriceCapProvider.getGasPriceCaps(targetL2BlockNumber).get(),
    ).isNull()

    assertThat(
      gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetL2BlockNumber).get(),
    ).isNull()
  }
}
