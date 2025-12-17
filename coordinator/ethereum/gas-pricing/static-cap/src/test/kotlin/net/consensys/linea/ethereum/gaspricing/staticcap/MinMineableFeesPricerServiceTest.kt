package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.domain.FeeHistory
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.GasPriceUpdater
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.ArgumentMatchers.anyLong
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.doAnswer
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class MinMineableFeesPricerServiceTest {
  private val feeHistory = FeeHistory(
    oldestBlock = 100uL,
    baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
    reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
    gasUsedRatio = listOf(
      0.25,
      0.5,
      0.75,
      0.9,
    ),
    baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
    blobGasUsedRatio = listOf(
      0.25,
      0.5,
      0.75,
      0.9,
    ),
  )

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun start_startsPollingProcess(vertx: Vertx, testContext: VertxTestContext) {
    val pollingInterval = 10.milliseconds
    val gasPriceUpperBound = 9000000000000.0
    val gasPriceLowerBound = 1000000.0
    val gasPriceFixedCost = 17.0
    val calculatedL2GasPrice = gasPriceUpperBound - 100.0
    val expectedGasPrice = calculatedL2GasPrice + gasPriceFixedCost

    val mockFeesFetcher = mock<FeesFetcher> {
      on { getL1EthGasPriceData() } doReturn SafeFuture.completedFuture(feeHistory)
    }
    val mockFeesCalculator = mock<FeesCalculator> {
      on { calculateFees(eq(feeHistory)) } doReturn calculatedL2GasPrice
    }
    val boundableFeeCalculator = BoundableFeeCalculator(
      BoundableFeeCalculator.Config(
        gasPriceUpperBound,
        gasPriceLowerBound,
        gasPriceFixedCost,
      ),
      mockFeesCalculator,
    )
    val mockGasPriceUpdater = mock<GasPriceUpdater> {
      on { updateMinerGasPrice(anyLong().toULong()) } doAnswer { SafeFuture.completedFuture(Unit) }
    }
    val monitor =
      MinMineableFeesPricerService(
        pollingInterval = pollingInterval,
        vertx = vertx,
        feesFetcher = mockFeesFetcher,
        feesCalculator = boundableFeeCalculator,
        gasPriceUpdater = mockGasPriceUpdater,
      )
    monitor.start().thenApply {
      vertx.setTimer(100) {
        testContext
          .verify {
            monitor.stop()
            verify(mockFeesFetcher, atLeastOnce()).getL1EthGasPriceData()
            verify(mockFeesCalculator, atLeastOnce()).calculateFees(feeHistory)
            verify(mockGasPriceUpdater, atLeastOnce()).updateMinerGasPrice(expectedGasPrice.toULong())
          }
          .completeNow()
      }
    }
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun updatePrice_defaultsToPriceCapCalculatedPriceGoesOverMaxLimit(vertx: Vertx) {
    val pollingInterval = 10.milliseconds
    val gasPriceUpperBound = 10_000_000_000.0 // 10 GWei
    val gasPriceLowerBound = 1_000_000.0 // 0.001 GWei
    val gasPriceFixedCost = 0.0
    val calculatedL2GasPrice = gasPriceUpperBound + 1.0

    val mockFeesFetcher = mock<FeesFetcher> {
      on { getL1EthGasPriceData() } doReturn SafeFuture.completedFuture(feeHistory)
    }
    val mockFeesCalculator = mock<FeesCalculator> {
      on { calculateFees(any()) } doReturn calculatedL2GasPrice
    }
    val boundableFeeCalculator = BoundableFeeCalculator(
      BoundableFeeCalculator.Config(
        gasPriceUpperBound,
        gasPriceLowerBound,
        gasPriceFixedCost,
      ),
      mockFeesCalculator,
    )
    val mockGasPriceUpdater = mock<GasPriceUpdater> {
      on { updateMinerGasPrice(anyLong().toULong()) } doAnswer { SafeFuture.completedFuture(Unit) }
    }

    val monitor =
      MinMineableFeesPricerService(
        pollingInterval = pollingInterval,
        vertx = vertx,
        feesFetcher = mockFeesFetcher,
        feesCalculator = boundableFeeCalculator,
        gasPriceUpdater = mockGasPriceUpdater,
      )

    monitor.action().get()
    verify(mockGasPriceUpdater).updateMinerGasPrice(gasPriceUpperBound.toULong())
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun updatePrice_defaultsToPriceCapCalculatedPriceGoesUnderMinLimit(vertx: Vertx) {
    val pollingInterval = 10.milliseconds
    val gasPriceUpperBound = 10_000_000_000.0 // 10 GWei
    val gasPriceLowerBound = 90_000_000.0 // 0.09 GWei
    val gasPriceFixedCost = 0.0
    val calculatedL2GasPrice = gasPriceLowerBound - 1.0

    val mockFeesFetcher = mock<FeesFetcher> {
      on { getL1EthGasPriceData() } doReturn SafeFuture.completedFuture(feeHistory)
    }
    val mockFeesCalculator = mock<FeesCalculator> {
      on { calculateFees(any()) } doReturn calculatedL2GasPrice
    }
    val boundableFeeCalculator = BoundableFeeCalculator(
      BoundableFeeCalculator.Config(
        gasPriceUpperBound,
        gasPriceLowerBound,
        gasPriceFixedCost,
      ),
      mockFeesCalculator,
    )
    val mockGasPriceUpdater = mock<GasPriceUpdater> {
      on { updateMinerGasPrice(anyLong().toULong()) } doAnswer { SafeFuture.completedFuture(Unit) }
    }

    val monitor =
      MinMineableFeesPricerService(
        pollingInterval = pollingInterval,
        vertx = vertx,
        feesFetcher = mockFeesFetcher,
        feesCalculator = boundableFeeCalculator,
        gasPriceUpdater = mockGasPriceUpdater,
      )

    monitor.action().get()
    verify(mockGasPriceUpdater).updateMinerGasPrice(gasPriceLowerBound.toULong())
  }
}
