package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.domain.FeeHistory
import linea.kotlin.toKWei
import net.consensys.linea.ethereum.gaspricing.ExtraDataUpdater
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataV1
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.extension.ExtendWith
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
class ExtraDataV1PricerServiceTest {
  private val defaultFixedCost = 123u
  private val defaultEthGasPriceMultiplier = 1.3
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
  @Timeout(2, timeUnit = TimeUnit.SECONDS)
  fun start_startsPollingProcess(vertx: Vertx, testContext: VertxTestContext) {
    val pollingInterval = 10.milliseconds
    val variableFees = 15000.0
    val expectedVariableFees = variableFees
    val legacyFees = 100000.0
    val expectedEthGasPrice = defaultEthGasPriceMultiplier * legacyFees

    val mockFeesFetcher = mock<FeesFetcher> {
      on { getL1EthGasPriceData() } doReturn SafeFuture.completedFuture(feeHistory)
    }
    val mockVariableFeesCalculator = mock<FeesCalculator> {
      on { calculateFees(eq(feeHistory)) } doReturn variableFees
    }
    val mockLegacyFeesCalculator = mock<FeesCalculator> {
      on { calculateFees(eq(feeHistory)) } doReturn legacyFees
    }
    val boundableFeeCalculator = MinerExtraDataV1CalculatorImpl(
      MinerExtraDataV1CalculatorImpl.Config(
        defaultFixedCost,
        defaultEthGasPriceMultiplier,
      ),
      variableFeesCalculator = mockVariableFeesCalculator,
      legacyFeesCalculator = mockLegacyFeesCalculator,
    )
    val mockExtraDataUpdater = mock<ExtraDataUpdater> {
      on { updateMinerExtraData(any()) } doAnswer { SafeFuture.completedFuture(Unit) }
    }
    val monitor =
      ExtraDataV1PricerService(
        pollingInterval = pollingInterval,
        vertx = vertx,
        feesFetcher = mockFeesFetcher,
        minerExtraDataCalculator = boundableFeeCalculator,
        extraDataUpdater = mockExtraDataUpdater,
      )

    val expectedExtraData = MinerExtraDataV1(
      defaultFixedCost,
      expectedVariableFees.toKWei().toUInt(),
      expectedEthGasPrice.toKWei().toUInt(),
    )

    // Start the service
    monitor.start().thenAccept { _ ->
      // Wait for a reasonable amount of time to ensure polling executes at least once
      // Use 5x the polling interval to ensure enough time for the action to complete
      vertx.setTimer(pollingInterval.inWholeMilliseconds * 5) {
        testContext
          .verify {
            // Stop the service
            monitor.stop()
            // Verify the mocks were called as expected
            verify(mockFeesFetcher, atLeastOnce()).getL1EthGasPriceData()
            verify(mockVariableFeesCalculator, atLeastOnce()).calculateFees(feeHistory)
            verify(mockLegacyFeesCalculator, atLeastOnce()).calculateFees(feeHistory)
            verify(mockExtraDataUpdater, atLeastOnce()).updateMinerExtraData(expectedExtraData)
          }
          .completeNow()
      }
    }
  }
}
